package extension

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
)

// ReconcileSecrets copies all secrets referenced in the extension (additionalSecretResources or
// source.SecretResourceName), and deletes all secrets that are no longer referenced.
// We cannot use gardener resource manager here, because we want to work in the namespace
// "flux-system", which the resource manager is not configured for.
func ReconcileSecrets(
	ctx context.Context,
	log logr.Logger,
	seedClient client.Client,
	shootClient client.Client,
	seedNamespace string,
	config *fluxv1alpha1.FluxConfig,
	resources []gardencorev1beta1.NamedResourceReference,
) error {
	shootNamespace := *config.Flux.Namespace
	secretsToKeep := sets.Set[string]{}

	secretResources := config.AdditionalSecretResources
	if config.Source != nil && config.Source.SecretResourceName != nil {
		// Decode the source template to extract the secret reference name
		if obj, _, err := fluxv1alpha1.DecodeSourceTemplate(config.Source.Template); err == nil {
			var secretRefName string
			switch v := obj.(type) {
			case *sourcev1.GitRepository:
				if v.Spec.SecretRef != nil {
					secretRefName = v.Spec.SecretRef.Name
				}
			case *sourcev1.OCIRepository:
				if v.Spec.SecretRef != nil {
					secretRefName = v.Spec.SecretRef.Name
				}
			}
			if secretRefName != "" {
				secretResources = append(secretResources, fluxv1alpha1.AdditionalResource{
					Name:       *config.Source.SecretResourceName,
					TargetName: ptr.To(secretRefName),
				})
			}
		}
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
		log.Info("Deleted secret that is no longer referenced by the extension", "secretName", secret.Name)
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

	result, err := controllerutil.CreateOrUpdate(ctx, shootClient, shootSecret, func() error {
		shootSecret.Data = maps.Clone(seedSecret.Data)
		labels := map[string]string{
			managedByLabelKey: managedByLabelValue,
		}
		if shouldCopy, _ := strconv.ParseBool(seedSecret.Annotations["gardener-extension-shoot-flux/copy-labels"]); shouldCopy {
			if seedSecret.Labels != nil {
				for k, v := range seedSecret.Labels {
					labels[k] = v
				}
			}
		}
		shootSecret.Labels = labels
		return nil
	})
	if err != nil {
		return "", err
	}
	log.Info("Synced secret", "secretName", shootSecret.Name, "result", result)

	return shootSecret.Name, nil
}
