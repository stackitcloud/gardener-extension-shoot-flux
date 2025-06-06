package extension

import (
	"context"
	"fmt"
	"time"

	fluxinstall "github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
	"github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1/validation"
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

	if extensionscontroller.IsHibernationEnabled(cluster) {
		// when hibernation is enabled there is nothing for us to do
		return nil
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

	if IsFluxBootstrapped(ext) {
		log.V(1).Info("Flux installation has been bootstrapped already, will only reconcile secrets")

		if err := ReconcileSecrets(ctx, log, a.client, shootClient, ext.Namespace, config, cluster.Shoot.Spec.Resources); err != nil {
			return fmt.Errorf("error reconciling secrets: %w", err)
		}

		if err := ReconcileShootInfoConfigMap(ctx, log, shootClient, config, cluster); err != nil {
			return fmt.Errorf("error reconciling ConfigMap %q: %w", shootInfoConfigMapName, err)
		}

		return nil
	}

	if err := InstallFlux(ctx, log, shootClient, config.Flux); err != nil {
		return fmt.Errorf("error installing Flux: %w", err)
	}

	// secrets might be necessary for the source to get ready
	if err := ReconcileSecrets(ctx, log, a.client, shootClient, ext.Namespace, config, cluster.Shoot.Spec.Resources); err != nil {
		return fmt.Errorf("error reconciling secrets: %w", err)
	}

	if config.Source != nil {
		if err := BootstrapSource(ctx, log, shootClient, config.Source); err != nil {
			return fmt.Errorf("error bootstrappping Flux GitRepository: %w", err)
		}
	}

	// configMap might be necessary for the kustomization to get ready
	if err := ReconcileShootInfoConfigMap(ctx, log, shootClient, config, cluster); err != nil {
		return fmt.Errorf("error reconciling ConfigMap %q: %w", shootInfoConfigMapName, err)
	}

	if config.Kustomization != nil {
		if err := BootstrapKustomization(ctx, log, shootClient, config.Kustomization); err != nil {
			return fmt.Errorf("error bootstrappping Flux Kustomization: %w", err)
		}
	}

	if err := SetFluxBootstrapped(ctx, a.client, ext); err != nil {
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

// ReconcileShootInfoConfigMap creates or updates a ConfigMap in the specified Flux namespace in the shoot cluster.
// The ConfigMap contains information about the Shoot cluster, such as its technical ID which can be used for
// substitutions in flux kustomizations or helmreleases.
func ReconcileShootInfoConfigMap(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	fluxCfg *fluxv1alpha1.FluxConfig,
	cluster *extensions.Cluster,
) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shootInfoConfigMapName,
			Namespace: *fluxCfg.Flux.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, shootClient, configMap, func() error {
		configMap.Labels = map[string]string{
			managedByLabelKey: managedByLabelValue,
		}
		configMap.Data = map[string]string{
			"SHOOT_INFO_CLUSTER_IDENTITY": *cluster.Shoot.Status.ClusterIdentity,
			"SHOOT_INFO_NAME":             cluster.Shoot.Name,
			"SHOOT_INFO_TECHNICAL_ID":     cluster.Shoot.Status.TechnicalID,
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update ConfigMap %q: %w", client.ObjectKeyFromObject(configMap), err)
	}

	if result != controllerutil.OperationResultNone {
		log.Info("Synced shoot info ConfigMap", "configMap", client.ObjectKeyFromObject(configMap))
	}

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

// IsFluxBootstrapped checks whether Flux was bootstrapped successfully at least once by checking the bootstrapped
// condition in the Extension status.
func IsFluxBootstrapped(ext *extensionsv1alpha1.Extension) bool {
	cond := v1beta1helper.GetCondition(ext.Status.Conditions, fluxv1alpha1.ConditionBootstrapped)
	return cond != nil && cond.Status == gardencorev1beta1.ConditionTrue
}

// SetFluxBootstrapped sets the bootstrapped condition in the Extension status to mark a successful initial bootstrap
// of Flux. Future reconciliations of the Extension resource will skip reconciliation of the Flux resources.
func SetFluxBootstrapped(ctx context.Context, c client.Client, ext *extensionsv1alpha1.Extension) error {
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

// InstallFlux applies the Flux install manifest based on the given configuration. It also performs a basic health check
// before returning.
func InstallFlux(ctx context.Context, log logr.Logger, c client.Client, config *fluxv1alpha1.FluxInstallation) error {
	return installFlux(ctx, log, c, config, "", 5*time.Second, time.Minute)
}

func installFlux(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	config *fluxv1alpha1.FluxInstallation,
	manifestsBase string,
	interval time.Duration,
	timeout time.Duration,
) error {
	log = log.WithValues("version", config.Version)
	log.Info("Installing Flux")

	installManifest, err := GenerateInstallManifest(config, manifestsBase)
	if err != nil {
		return fmt.Errorf("error generating install manifest: %w", err)
	}

	if err := kubernetes.NewApplier(c, c.RESTMapper()).ApplyManifest(ctx, kubernetes.NewManifestReader(installManifest), nil); err != nil {
		return fmt.Errorf("error applying Flux install manifest: %w", err)
	}

	log.Info("Waiting for Flux installation to get ready")
	// Wait for GitRepository CRD to become healthy as a basic indicator of whether the installation is ready to be
	// bootstrapped.
	// We don't intend to health check the entire Flux installation, but we want to avoid bootstrap failures that could
	// have been avoided by a short wait.
	gitRepositoryCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "gitrepositories." + sourcev1.GroupVersion.Group},
	}
	if err := WaitForObject(ctx, c, gitRepositoryCRD, interval, timeout, func() (done bool, err error) {
		err = health.CheckCustomResourceDefinition(gitRepositoryCRD)
		return err == nil, err
	}); err != nil {
		return fmt.Errorf("error waiting for Flux installation to get ready: %w", err)
	}

	// Wait for one of the deployments to be available to ensure the selected registry actually hosts flux container
	// images.
	sourceController := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-controller",
			Namespace: *config.Namespace,
		},
	}
	if err := WaitForObject(ctx, c, sourceController, interval, timeout, func() (done bool, err error) {
		err = health.CheckDeployment(sourceController)
		return err == nil, err
	}); err != nil {
		return fmt.Errorf("error waiting for Flux installation to get ready: %w", err)
	}

	log.Info("Successfully installed Flux")

	return nil
}

// GenerateInstallManifest generates the Flux install manifest based on the given configuration just like
// "flux install --export". manifestsBase can be set for tests.
func GenerateInstallManifest(config *fluxv1alpha1.FluxInstallation, manifestsBase string) ([]byte, error) {
	options := fluxinstall.MakeDefaultOptions()
	options.Version = *config.Version
	options.Namespace = *config.Namespace
	options.Registry = *config.Registry

	// don't deploy optional components
	options.ComponentsExtra = nil

	manifest, err := fluxinstall.Generate(options, manifestsBase)
	if err != nil {
		return nil, err
	}

	return []byte(manifest.Content), nil
}

// BootstrapSource creates the GitRepository object specified in the given config and waits for it to get ready.
func BootstrapSource(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	config *fluxv1alpha1.Source,
) error {
	return bootstrapSource(ctx, log, shootClient, config, 5*time.Second, 5*time.Minute)
}

func bootstrapSource(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	config *fluxv1alpha1.Source,
	interval time.Duration,
	timeout time.Duration,
) error {
	log.Info("Bootstrapping Flux GitRepository")

	// Create Namespace in case the GitRepository is located in a different namespace than the Flux components.
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.Template.Namespace}}
	if err := shootClient.Create(ctx, namespace); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("error creating %s namespace: %w", config.Template.Namespace, err)
	}

	// Create GitRepository
	gitRepository := config.Template.DeepCopy()
	if _, err := controllerutil.CreateOrUpdate(ctx, shootClient, gitRepository, func() error {
		config.Template.Spec.DeepCopyInto(&gitRepository.Spec)
		return nil
	}); err != nil {
		return fmt.Errorf("error applying GitRepository template: %w", err)
	}

	log.Info("Waiting for GitRepository to get ready")
	if err := WaitForObject(ctx, shootClient, gitRepository, interval, timeout, CheckFluxObject(gitRepository)); err != nil {
		return fmt.Errorf("error waiting for GitRepository to get ready: %w", err)
	}

	log.Info("Successfully bootstrapped Flux GitRepository")

	return nil
}

// BootstrapKustomization creates the Kustomization object specified in the given config and waits for it to get ready.
func BootstrapKustomization(ctx context.Context, log logr.Logger, c client.Client, config *fluxv1alpha1.Kustomization) error {
	return bootstrapKustomization(ctx, log, c, config, 5*time.Second, 5*time.Minute)
}

func bootstrapKustomization(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	config *fluxv1alpha1.Kustomization,
	interval time.Duration,
	timeout time.Duration,
) error {
	log.Info("Bootstrapping Flux Kustomization")

	// Create Namespace in case the GitRepository is located in a different namespace than the Flux components.
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.Template.Namespace}}
	if err := c.Create(ctx, namespace); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("error creating %s namespace: %w", config.Template.Namespace, err)
	}

	kustomization := config.Template.DeepCopy()
	if _, err := controllerutil.CreateOrUpdate(ctx, c, kustomization, func() error {
		config.Template.Spec.DeepCopyInto(&kustomization.Spec)
		return nil
	}); err != nil {
		return fmt.Errorf("error applying Kustomization template: %w", err)
	}

	log.Info("Waiting for Kustomization to get ready")
	if err := WaitForObject(ctx, c, kustomization, interval, timeout, CheckFluxObject(kustomization)); err != nil {
		return fmt.Errorf("error waiting for Kustomization to get ready: %w", err)
	}

	log.Info("Successfully bootstrapped Flux Kustomization")

	return nil
}

// ConditionFunc checks the health of a polled object. If done==true, waiting should stop and propagate the returned
// error. If done==false, the error is preserved but the check is retried.
type ConditionFunc func() (done bool, err error)

// WaitForObject periodically reads the given object and waits for the given ConditionFunc to return done==true.
// If the check times out, it returns the last error from the ConditionFunc.
func WaitForObject(ctx context.Context, c client.Reader, obj client.Object, interval, timeout time.Duration, check ConditionFunc) error {
	var lastError error
	if err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		lastError = c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(lastError) {
			// wait for the object to appear
			return false, nil
		}
		if lastError != nil {
			// severe error, fail immediately
			return false, lastError
		}

		var done bool
		done, lastError = check()
		if done {
			return true, lastError
		}
		return false, nil
	}); err != nil {
		// if we timed out waiting, return the last error that we observed instead of "context deadline exceeded" or similar
		if lastError != nil {
			return lastError
		}
		return err
	}

	return nil
}

// CheckFluxObject returns a ConditionFunc that determines the health of Flux objects based on the Ready condition.
func CheckFluxObject(obj fluxmeta.ObjectWithConditions) ConditionFunc {
	return func() (healthy bool, err error) {
		if cond := meta.FindStatusCondition(obj.GetConditions(), fluxmeta.ReadyCondition); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				return true, nil
			case metav1.ConditionFalse:
				return true, fmt.Errorf("reconciliation failed: %s", cond.Message)
			}
		}

		return false, fmt.Errorf("has not been reconciled yet")
	}
}
