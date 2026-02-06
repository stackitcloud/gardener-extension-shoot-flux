package validation

import (
	"slices"
	"strings"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
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

var requiredComponents = []string{"kustomize-controller", "source-controller"}

// ValidateFluxInstallation validates a FluxInstallation object.
func ValidateFluxInstallation(fluxInstallation *fluxv1alpha1.FluxInstallation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if namespace := fluxInstallation.Namespace; namespace != nil && *namespace != "" {
		for _, msg := range apivalidation.ValidateNamespaceName(*namespace, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), *namespace, msg))
		}
	}

	if len(fluxInstallation.Components) > 0 {
		wantedComponents := append(fluxInstallation.Components, fluxInstallation.ComponentsExtra...)
		for _, requiredComponent := range requiredComponents {
			if !slices.Contains(wantedComponents, requiredComponent) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("components"), fluxInstallation.Components, "missing required component "+requiredComponent))
			}
		}
	}

	return allErrs
}

var (
	supportedGitRepositoryGVK = sourcev1.GroupVersion.WithKind(sourcev1.GitRepositoryKind)
	supportedOCIRepositoryGVK = sourcev1.GroupVersion.WithKind(sourcev1.OCIRepositoryKind)
)

// ValidateSource validates a Source object.
func ValidateSource(source *fluxv1alpha1.Source, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if source.Template == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("template"), "template is required"))
		return allErrs
	}

	// Decode the template to determine its type
	obj, kind, err := fluxv1alpha1.DecodeSourceTemplate(source.Template)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("template"), source.Template, err.Error()))
		return allErrs
	}

	templatePath := fldPath.Child("template")

	// Validate based on the source type
	switch v := obj.(type) {
	case *sourcev1.GitRepository:
		allErrs = append(allErrs, validateGitRepository(v, source.SecretResourceName, shoot, templatePath, fldPath)...)
	case *sourcev1.OCIRepository:
		allErrs = append(allErrs, validateOCIRepository(v, source.SecretResourceName, shoot, templatePath, fldPath)...)
	default:
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("kind"), kind, []string{sourcev1.GitRepositoryKind, sourcev1.OCIRepositoryKind}))
	}

	return allErrs
}

// validateGitRepository validates a GitRepository template.
func validateGitRepository(template *sourcev1.GitRepository, secretResourceName *string, shoot *gardencorev1beta1.Shoot, templatePath, parentPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate GVK
	if gvk := template.GroupVersionKind(); !gvk.Empty() && gvk != supportedGitRepositoryGVK {
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("apiVersion"), template.APIVersion, []string{supportedGitRepositoryGVK.GroupVersion().String()}))
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("kind"), template.Kind, []string{supportedGitRepositoryGVK.Kind}))
	}

	// Validate spec fields
	specPath := templatePath.Child("spec")
	if ref := template.Spec.Reference; ref == nil || apiequality.Semantic.DeepEqual(ref, &sourcev1.GitRepositoryRef{}) {
		allErrs = append(allErrs, field.Required(specPath.Child("ref"), "GitRepository must have a reference"))
	}

	if template.Spec.URL == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("url"), "GitRepository must have a URL"))
	}

	// Validate secret references
	allErrs = append(allErrs, validateSourceSecretReferences(template.Spec.SecretRef, secretResourceName, shoot, specPath, parentPath)...)

	return allErrs
}

// validateOCIRepository validates an OCIRepository template.
func validateOCIRepository(template *sourcev1.OCIRepository, secretResourceName *string, shoot *gardencorev1beta1.Shoot, templatePath, parentPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate GVK
	if gvk := template.GroupVersionKind(); !gvk.Empty() && gvk != supportedOCIRepositoryGVK {
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("apiVersion"), template.APIVersion, []string{supportedOCIRepositoryGVK.GroupVersion().String()}))
		allErrs = append(allErrs, field.NotSupported(templatePath.Child("kind"), template.Kind, []string{supportedOCIRepositoryGVK.Kind}))
	}

	// Validate spec fields
	specPath := templatePath.Child("spec")

	// Validate URL
	if template.Spec.URL == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("url"), "OCIRepository must have a URL"))
	} else if !strings.HasPrefix(template.Spec.URL, "oci://") {
		allErrs = append(allErrs, field.Invalid(specPath.Child("url"), template.Spec.URL, "must start with oci://"))
	}

	// Validate reference
	if ref := template.Spec.Reference; ref == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("ref"), "OCIRepository must have a reference"))
	} else if ref.Tag == "" && ref.SemVer == "" && ref.Digest == "" {
		allErrs = append(allErrs, field.Invalid(specPath.Child("ref"), ref, "must specify tag, semver, or digest"))
	}

	// Validate secret references
	allErrs = append(allErrs, validateSourceSecretReferences(template.Spec.SecretRef, secretResourceName, shoot, specPath, parentPath)...)

	return allErrs
}

// validateSourceSecretReferences validates the secret reference consistency between
// spec.secretRef and source.secretResourceName.
func validateSourceSecretReferences(secretRef *meta.LocalObjectReference, secretResourceName *string, shoot *gardencorev1beta1.Shoot, specPath, parentPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	hasSecretRef := secretRef != nil && secretRef.Name != ""
	hasSecretResourceName := ptr.Deref(secretResourceName, "") != ""
	secretRefPath := specPath.Child("secretRef")
	secretResourceNamePath := parentPath.Child("secretResourceName")

	if hasSecretRef && !hasSecretResourceName {
		allErrs = append(allErrs, field.Required(secretResourceNamePath, "must specify a secret resource name if "+secretRefPath.String()+" is specified"))
	}
	if !hasSecretRef && hasSecretResourceName {
		allErrs = append(allErrs, field.Required(secretRefPath, "must specify a secret ref if "+secretResourceNamePath.String()+" is specified"))
	}

	if hasSecretResourceName {
		allErrs = append(allErrs, validateSecretResource(shoot.Spec.Resources, secretResourceNamePath, *secretResourceName)...)
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

	for i, r := range additionalResources {
		if ptr.Deref(r.TargetName, "") != "" && len(validation.IsDNS1123Subdomain(*r.TargetName)) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("targetName"), *r.TargetName, "must be a valid resource name"))
		}
		allErrs = append(allErrs, validateSecretResource(shoot.Spec.Resources, fldPath.Index(i).Child("name"), r.Name)...)
	}

	return allErrs
}

func validateSecretResource(resources []gardencorev1beta1.NamedResourceReference, fldPath *field.Path, name string) field.ErrorList {
	allErrs := field.ErrorList{}
	r := v1beta1helper.GetResourceByName(resources, name)
	if r == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, name, "secret resource name does not match any of the resource names in Shoot.spec.resources[].name"))
		return allErrs
	}
	if r.ResourceRef.Kind != "Secret" {
		allErrs = append(allErrs, field.Invalid(fldPath, r.Name, "secret resource name references a Shoot.spec.resources[], which is not a secret"))
	}
	return allErrs
}
