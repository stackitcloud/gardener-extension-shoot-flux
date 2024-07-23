package validation

import (
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
)

// ValidateFluxConfig validates a FluxConfig object.
func ValidateFluxConfig(fluxConfig *fluxv1alpha1.FluxConfig, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if fluxConfig.Flux != nil {
		allErrs = append(allErrs, ValidateFluxInstallation(fluxConfig.Flux, fldPath.Child("flux"))...)
	}

	if (fluxConfig.Source == nil) && (fluxConfig.Kustomization != nil) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("source"), fluxConfig.Source, "must specify a source if a kustomization is specified"))
	}
	if (fluxConfig.Kustomization == nil) && (fluxConfig.Source != nil) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kustomization"), fluxConfig.Kustomization, "must specify a kustomization if a source is specified"))
	}

	if fluxConfig.Source != nil {
		allErrs = append(allErrs, ValidateSource(fluxConfig.Source, shoot, fldPath.Child("source"))...)
	}

	if fluxConfig.Kustomization != nil {
		allErrs = append(allErrs, ValidateKustomization(fluxConfig.Kustomization, fldPath.Child("kustomization"))...)
	}
	allErrs = append(allErrs, ValidateAdditionalSecretResources(fluxConfig.AdditionalSecretResources, shoot, fldPath.Child("additionalSecretResources"))...)

	return allErrs
}

// ValidateFluxInstallation validates a FluxInstallation object.
func ValidateFluxInstallation(fluxInstallation *fluxv1alpha1.FluxInstallation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if namespace := fluxInstallation.Namespace; namespace != nil && *namespace != "" {
		for _, msg := range apivalidation.ValidateNamespaceName(*namespace, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), *namespace, msg))
		}
	}

	return allErrs
}

var supportedGitRepositoryGVK = sourcev1.GroupVersion.WithKind(sourcev1.GitRepositoryKind)

// ValidateSource validates a Source object.
func ValidateSource(source *fluxv1alpha1.Source, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	template := source.Template
	templatePath := fldPath.Child("template")

	if gvk := template.GroupVersionKind(); !gvk.Empty() && gvk != supportedGitRepositoryGVK {
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("apiVersion"), template.APIVersion, []string{supportedGitRepositoryGVK.GroupVersion().String()}))
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("kind"), template.APIVersion, []string{supportedGitRepositoryGVK.Kind}))
	}

	specPath := templatePath.Child("spec")
	if ref := template.Spec.Reference; ref == nil || apiequality.Semantic.DeepEqual(ref, &sourcev1.GitRepositoryRef{}) {
		allErrs = append(allErrs, field.Required(specPath.Child("ref"), "GitRepository must have a reference"))
	}

	if template.Spec.URL == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("url"), "GitRepository must have an URL"))
	}

	hasSecretRef := template.Spec.SecretRef != nil && template.Spec.SecretRef.Name != ""
	hasSecretResourceName := ptr.Deref(source.SecretResourceName, "") != ""
	secretRefPath := specPath.Child("secretRef")
	secretResourceNamePath := fldPath.Child("secretResourceName")

	if hasSecretRef && !hasSecretResourceName {
		allErrs = append(allErrs, field.Required(secretResourceNamePath, "must specify a secret resource name if "+secretRefPath.String()+" is specified"))
	}
	if !hasSecretRef && hasSecretResourceName {
		allErrs = append(allErrs, field.Required(secretRefPath, "must specify a secret ref if "+secretResourceNamePath.String()+" is specified"))
	}

	if hasSecretResourceName {
		resourceNames := sets.New[string]()
		for _, resource := range shoot.Spec.Resources {
			resourceNames.Insert(resource.Name)
		}

		if !resourceNames.Has(*source.SecretResourceName) {
			allErrs = append(allErrs, field.Invalid(secretResourceNamePath, *source.SecretResourceName, "secret resource name does not match any of the resource names in Shoot.spec.resources[].name"))
		}
	}

	return allErrs
}

var supportedKustomizationGVK = kustomizev1.GroupVersion.WithKind(kustomizev1.KustomizationKind)

// ValidateKustomization validates a Kustomization object.
func ValidateKustomization(kustomization *fluxv1alpha1.Kustomization, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	template := kustomization.Template
	templatePath := fldPath.Child("template")

	if gvk := template.GroupVersionKind(); !gvk.Empty() && gvk != supportedKustomizationGVK {
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("apiVersion"), template.APIVersion, []string{supportedKustomizationGVK.GroupVersion().String()}))
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("kind"), template.APIVersion, []string{supportedKustomizationGVK.Kind}))
	}

	specPath := templatePath.Child("spec")
	if template.Spec.Path == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("path"), "Kustomization must have a path"))
	}

	return allErrs
}

// ValidateAdditionalSecretResources validates additionalResources
func ValidateAdditionalSecretResources(additionalResources []fluxv1alpha1.AdditionalResource, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(additionalResources) == 0 {
		return allErrs
	}
	resources := map[string]gardencorev1beta1.NamedResourceReference{}
	for _, resource := range shoot.Spec.Resources {
		resources[resource.Name] = resource
	}
	for i, r := range additionalResources {
		if _, found := resources[r.Name]; !found {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), r.Name, "additionalSecretResource name does not match any of the resource names in Shoot.spec.resources[].name"))
			continue
		}
		if resources[r.Name].ResourceRef.Kind != "Secret" {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), r.Name, "additionalSecretResource name references a Shoot.spec.resources[], which is not a secret"))
		}
	}

	return allErrs
}
