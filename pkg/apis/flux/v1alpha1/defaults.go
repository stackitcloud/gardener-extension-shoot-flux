package v1alpha1

import (
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	defaultFluxNamespace     = "flux-system"
	defaultGitRepositoryName = "flux-system"

	// defaultFluxVersion is maintained by renovate via a customManager. Don't
	// change this without also updating the renovate config.
	defaultFluxVersion = "v2.7.5"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_FluxConfig(obj *FluxConfig) {
	if obj.Flux == nil {
		obj.Flux = &FluxInstallation{}
	}

	// validation will ensure that both Source & Kustomization or set or both
	// are nil, but we have to handle all cases, since defaulting happens first.
	if obj.Source != nil && obj.Kustomization != nil {
		var sourceName, sourceNamespace string
		if obj.Source.GitRepository != nil {
			sourceName = obj.Source.GitRepository.Template.Name
			sourceNamespace = obj.Source.GitRepository.Template.Namespace
		} else if obj.Source.OCIRepository != nil {
			sourceName = obj.Source.OCIRepository.Template.Name
			sourceNamespace = obj.Source.OCIRepository.Template.Namespace
		}

		if obj.Kustomization.Template.Spec.SourceRef.Name == "" && sourceName != "" {
			obj.Kustomization.Template.Spec.SourceRef.Name = sourceName
		}
		if obj.Kustomization.Template.Spec.SourceRef.Namespace == "" && sourceNamespace != "" {
			obj.Kustomization.Template.Spec.SourceRef.Namespace = sourceNamespace
		}
	}

	if namespace := ptr.Deref(obj.Flux.Namespace, ""); namespace != "" {
		if obj.Source != nil {
			if obj.Source.GitRepository != nil && obj.Source.GitRepository.Template.Namespace == "" {
				obj.Source.GitRepository.Template.Namespace = namespace
			}
			if obj.Source.OCIRepository != nil && obj.Source.OCIRepository.Template.Namespace == "" {
				obj.Source.OCIRepository.Template.Namespace = namespace
			}
		}
		if obj.Kustomization != nil && obj.Kustomization.Template.Namespace == "" {
			obj.Kustomization.Template.Namespace = namespace
		}
		if obj.Kustomization != nil && obj.Kustomization.Template.Spec.SourceRef.Namespace == "" {
			obj.Kustomization.Template.Spec.SourceRef.Namespace = namespace
		}
	}
}

func SetDefaults_FluxInstallation(obj *FluxInstallation) {
	if obj.Version == nil {
		obj.Version = ptr.To(defaultFluxVersion)
	}

	if obj.Registry == nil {
		obj.Registry = ptr.To("ghcr.io/fluxcd")
	}

	if obj.Namespace == nil {
		obj.Namespace = ptr.To(defaultFluxNamespace)
	}
}

func SetDefaults_Source(obj *Source) {
	// Migrate deprecated fields to new structure for backwards compatibility
	if obj.Template != nil && obj.GitRepository == nil && obj.OCIRepository == nil {
		// Old format detected, migrate to new format
		obj.GitRepository = &GitRepositorySource{
			Template:           *obj.Template,
			SecretResourceName: obj.SecretResourceName,
		}
		// Clear deprecated fields after migration
		obj.Template = nil
		obj.SecretResourceName = nil
	}

	if obj.GitRepository != nil {
		SetDefaults_Flux_GitRepository(&obj.GitRepository.Template)

		hasSecretRef := obj.GitRepository.Template.Spec.SecretRef != nil && obj.GitRepository.Template.Spec.SecretRef.Name != ""
		hasSecretResourceName := ptr.Deref(obj.GitRepository.SecretResourceName, "") != ""
		if hasSecretResourceName && !hasSecretRef {
			obj.GitRepository.Template.Spec.SecretRef = &meta.LocalObjectReference{
				Name: "flux-system",
			}
		}
	}

	if obj.OCIRepository != nil {
		SetDefaults_Flux_OCIRepository(&obj.OCIRepository.Template)

		hasSecretRef := obj.OCIRepository.Template.Spec.SecretRef != nil && obj.OCIRepository.Template.Spec.SecretRef.Name != ""
		hasSecretResourceName := ptr.Deref(obj.OCIRepository.SecretResourceName, "") != ""
		if hasSecretResourceName && !hasSecretRef {
			obj.OCIRepository.Template.Spec.SecretRef = &meta.LocalObjectReference{
				Name: "flux-system",
			}
		}
	}
}

func SetDefaults_Kustomization(obj *Kustomization) {
	SetDefaults_Flux_Kustomization(&obj.Template)
}

func SetDefaults_Flux_GitRepository(obj *sourcev1.GitRepository) {
	if obj.Name == "" {
		obj.Name = defaultGitRepositoryName
	}

	if obj.Namespace == "" {
		obj.Namespace = defaultFluxNamespace
	}

	if obj.Spec.Interval.Duration == 0 {
		obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
	}
}

func SetDefaults_Flux_OCIRepository(obj *sourcev1.OCIRepository) {
	if obj.Name == "" {
		obj.Name = defaultGitRepositoryName
	}

	if obj.Namespace == "" {
		obj.Namespace = defaultFluxNamespace
	}

	if obj.Spec.Interval.Duration == 0 {
		obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
	}
}

func SetDefaults_Flux_Kustomization(obj *kustomizev1.Kustomization) {
	if obj.Name == "" {
		obj.Name = "flux-system"
	}

	if obj.Namespace == "" {
		obj.Namespace = defaultFluxNamespace
	}

	if obj.Spec.SourceRef.Kind == "" {
		obj.Spec.SourceRef.Kind = sourcev1.GitRepositoryKind
	}
	if obj.Spec.SourceRef.Name == "" {
		obj.Spec.SourceRef.Name = defaultGitRepositoryName
	}
	if obj.Spec.SourceRef.Namespace == "" {
		obj.Spec.SourceRef.Namespace = defaultFluxNamespace
	}

	if obj.Spec.Interval.Duration == 0 {
		obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
	}
}
