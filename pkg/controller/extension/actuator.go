package extension

import (
	"context"
	"fmt"
	"maps"
	"time"

	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
	"github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1/validation"
)

const (
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "gardener-extension-" + fluxv1alpha1.ExtensionType
)

type actuator struct {
	client  client.Client
	decoder runtime.Decoder
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(client client.Client) extension.Actuator {
	return &actuator{
		client:  client,
		decoder: serializer.NewCodecFactory(client.Scheme()).UniversalDecoder(),
	}
}

// Reconcile the extension resource.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ext *extensionsv1alpha1.Extension) error {
	cluster, err := extensionscontroller.GetCluster(ctx, a.client, ext.Namespace)
	if err != nil {
		return fmt.Errorf("error reading Cluster object: %w", err)
	}

	config, err := a.DecodeProviderConfig(ext.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error decoding providerConfig: %w", err)
	}

	// TODO: add an admission component that validates the providerConfig when creating/updating Shoots
	if allErrs := validation.ValidateFluxConfig(config, cluster.Shoot, nil); len(allErrs) > 0 {
		return fmt.Errorf("invalid providerConfig: %w", allErrs.ToAggregate())
	}

	_, shootClient, err := util.NewClientForShoot(ctx, a.client, ext.Namespace, client.Options{Scheme: a.client.Scheme()}, extensionsconfig.RESTOptions{})
	if err != nil {
		return fmt.Errorf("error creating shoot client: %w", err)
	}

	fluxBootstrapped := IsFluxBootstrapped(ctx, shootClient, *config.Flux.Namespace)
	if config.SyncMode == fluxv1alpha1.SyncModeOnce && fluxBootstrapped {
		// exit early if "Once" mode is enabled.
		log.V(1).Info("Flux installation has been bootstrapped already, skipping reconciliation of Flux resources")
		return SetFluxBootstrappedCondition(ctx, a.client, ext)
	}

	if !fluxBootstrapped {
		if err := InstallFlux(ctx, log, shootClient, config.Flux, "", 5*time.Second, time.Minute); err != nil {
			return fmt.Errorf("error installing Flux: %w", err)
		}
	}

	if err := ReconcileSecrets(ctx, log, shootClient, a.client, ext.Namespace, config, cluster.Shoot.Spec.Resources); err != nil {
		return fmt.Errorf("error reconciling secrets: %w", err)
	}

	if config.Source != nil {
		if err := ReconcileSource(ctx, log, shootClient, config.Source, 5*time.Second, 5*time.Minute); err != nil {
			return fmt.Errorf("error bootstrappping Flux GitRepository: %w", err)
		}
	}

	if config.Kustomization != nil {
		if err := ReconcileKustomization(ctx, log, shootClient, config.Kustomization, 5*time.Second, 5*time.Minute); err != nil {
			return fmt.Errorf("error bootstrappping Flux Kustomization: %w", err)
		}
	}

	if err := SetFluxBootstrappedCondition(ctx, a.client, ext); err != nil {
		return fmt.Errorf("error marking successful boostrapping: %w", err)
	}

	return nil
}

// Delete does nothing. The extension purposely does not perform deletion of the deployed Flux components or resources
// because it will most likely be a destructive operation. If users want to uninstall flux, they should use the
// documented approaches. On Shoot deletion, the objects will be cleaned up anyway, there is no point in deleting them
// gracefully.
func (a *actuator) Delete(context.Context, logr.Logger, *extensionsv1alpha1.Extension) error {
	return nil
}

// ForceDelete force deletes the extension resource.
func (a *actuator) ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Extension) error {
	return nil
}

// Migrate the extension resource.
func (a *actuator) Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Extension) error {
	return nil
}

// Restore the extension resource.
func (a *actuator) Restore(context.Context, logr.Logger, *extensionsv1alpha1.Extension) error {
	return nil
}

// DecodeProviderConfig decodes the given providerConfig and performs API defaulting. If the providerConfig is empty,
// a new empty FluxConfig object is defaulted instead. This simplifies the controller's code as we can assume that all
// fields have been defaulted.
func (a *actuator) DecodeProviderConfig(rawExtension *runtime.RawExtension) (*fluxv1alpha1.FluxConfig, error) {
	config := &fluxv1alpha1.FluxConfig{}
	if rawExtension == nil || rawExtension.Raw == nil {
		a.client.Scheme().Default(config)
	} else if err := runtime.DecodeInto(a.decoder, rawExtension.Raw, config); err != nil {
		return nil, err
	}
	return config, nil
}

// IsFluxBootstrapped checks whether Flux was bootstrapped successfully by checking if a source-controller deployment
// exists.
func IsFluxBootstrapped(ctx context.Context, shootClient client.Client, shootNamespace string) bool {
	// check if the source controller exists, then we can assume a previous reconcile installed flux.
	sourceController := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-controller",
			Namespace: shootNamespace,
		},
	}
	if err := shootClient.Get(ctx, client.ObjectKeyFromObject(sourceController), sourceController); err != nil {
		return false
	}
	return true
}

// SetFluxBootstrappedCondition sets the bootstrapped condition in the Extension status to mark a successful initial bootstrap
// of Flux. Future reconciliations of the Extension resource will skip reconciliation of the Flux resources.
func SetFluxBootstrappedCondition(ctx context.Context, c client.Client, ext *extensionsv1alpha1.Extension) error {
	b, err := v1beta1helper.NewConditionBuilder(fluxv1alpha1.ConditionBootstrapped)
	utilruntime.Must(err)

	if cond := v1beta1helper.GetCondition(ext.Status.Conditions, fluxv1alpha1.ConditionBootstrapped); cond != nil {
		b.WithOldCondition(*cond)
	}

	cond, updated := b.WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason("BootstrapSuccessful").
		WithMessage("Flux has been successfully bootstrapped on the Shoot cluster.").
		Build()
	if !updated {
		return nil
	}

	patch := client.MergeFromWithOptions(ext.DeepCopy(), client.MergeFromWithOptimisticLock{})
	ext.Status.Conditions = v1beta1helper.MergeConditions(ext.Status.Conditions, cond)
	if err := c.Status().Patch(ctx, ext, patch); err != nil {
		return fmt.Errorf("error setting %s condition in Extension status: %w", fluxv1alpha1.ConditionBootstrapped, err)
	}

	return nil
}

// ReconcileSecrets copies all secrets referenced in the extension (additionalSecretResources or
// source.SecretResourceName), and deletes all secrets that are no longer referenced.
func ReconcileSecrets(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	seedClient client.Client,
	seedNamespace string,
	config *fluxv1alpha1.FluxConfig,
	resources []gardencorev1beta1.NamedResourceReference,
) error {
	shootNamespace := *config.Flux.Namespace
	secretsToKeep := sets.Set[string]{}

	secretResources := config.AdditionalSecretResources
	if config.Source.SecretResourceName != nil {
		secretResources = append(secretResources, fluxv1alpha1.AdditionalResource{
			Name:       *config.Source.SecretResourceName,
			TargetName: ptr.To(config.Source.Template.Spec.SecretRef.Name),
		})
	}
	for _, resource := range secretResources {
		name, err := copySecretToShoot(ctx, log, seedClient, shootClient, seedNamespace, shootNamespace, resources, resource)
		if err != nil {
			return fmt.Errorf("failed to copy secret: %w", err)
		}
		secretsToKeep.Insert(name)
	}

	// cleanup unreferenced secrets
	secretList := &corev1.SecretList{}
	if err := shootClient.List(ctx, secretList,
		client.InNamespace(shootNamespace),
		client.MatchingLabels{managedByLabelKey: managedByLabelValue},
	); err != nil {
		return fmt.Errorf("failed to list managed secrets in shoot: %w", err)
	}
	for _, secret := range secretList.Items {
		if secretsToKeep.Has(secret.Name) {
			continue
		}
		if err := shootClient.Delete(ctx, &secret); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete secret that is no longer referenced: %w", err)
		}
		log.Info("deleted secret that is no longer referenced by the extension", "secretName", secret.Name)
	}
	return nil
}

func copySecretToShoot(
	ctx context.Context,
	log logr.Logger,
	seedClient client.Client,
	shootClient client.Client,
	seedNamespace string,
	targetNamespace string,
	resources []gardencorev1beta1.NamedResourceReference,
	additionalResource fluxv1alpha1.AdditionalResource,
) (string, error) {
	resource := v1beta1helper.GetResourceByName(resources, additionalResource.Name)
	if resource == nil {
		return "", fmt.Errorf("secret resource name does not match any of the resource names in Shoot.spec.resources[].name")
	}

	seedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.ReferencedResourcesPrefix + resource.ResourceRef.Name,
			Namespace: seedNamespace,
		},
	}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(seedSecret), seedSecret); err != nil {
		return "", fmt.Errorf("error reading referenced secret: %w", err)
	}

	name := resource.ResourceRef.Name
	if additionalResource.TargetName != nil {
		name = *additionalResource.TargetName
	}
	shootSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: targetNamespace,
		},
	}
	res, err := controllerutil.CreateOrUpdate(ctx, shootClient, shootSecret, func() error {
		shootSecret.Data = maps.Clone(seedSecret.Data)
		setSecretMeta(seedSecret, shootSecret)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to ensure secret resource")
	}
	if res != controllerutil.OperationResultNone {
		log.Info("Ensured secret", "secretName", shootSecret.Name, "result", res)
	}

	return shootSecret.Name, nil
}
