package v1alpha1

import (
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// validation will ensure that both Source & Kustomization are set or both
	// are nil, but we have to handle all cases, since defaulting happens first.
	if obj.Source != nil && obj.Kustomization != nil {
		// Decode source template to get name and namespace
		sourceObj, _, err := DecodeSourceTemplate(obj.Source.Template)
		if err == nil {
			clientObj := sourceObj.(client.Object)
			sourceName := clientObj.GetName()
			sourceNamespace := clientObj.GetNamespace()

			if obj.Kustomization.Template.Spec.SourceRef.Name == "" && sourceName != "" {
				obj.Kustomization.Template.Spec.SourceRef.Name = sourceName
			}
			if obj.Kustomization.Template.Spec.SourceRef.Namespace == "" && sourceNamespace != "" {
				obj.Kustomization.Template.Spec.SourceRef.Namespace = sourceNamespace
			}
		}
	}

	if namespace := ptr.Deref(obj.Flux.Namespace, ""); namespace != "" {
		if obj.Source != nil && obj.Source.Template != nil {
			// Decode, update namespace if needed, re-encode
			sourceObj, _, err := DecodeSourceTemplate(obj.Source.Template)
			if err == nil {
				clientObj := sourceObj.(client.Object)
				if clientObj.GetNamespace() == "" {
					clientObj.SetNamespace(namespace)
					if encoded, err := encodeSourceTemplate(sourceObj); err == nil {
						obj.Source.Template = encoded
					}
				}
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
	if obj.Template == nil {
		return
	}

	// Decode the template
	sourceObj, _, err := DecodeSourceTemplate(obj.Template)
	if err != nil {
		return
	}

	oldSource := sourceObj.DeepCopyObject()

	// Apply defaults based on source type
	switch v := sourceObj.(type) {
	case *sourcev1.GitRepository:
		SetDefaults_Flux_GitRepository(v)

		// If secretResourceName is set but secretRef is not, create default secretRef
		hasSecretRef := v.Spec.SecretRef != nil && v.Spec.SecretRef.Name != ""
		hasSecretResourceName := ptr.Deref(obj.SecretResourceName, "") != ""
		if hasSecretResourceName && !hasSecretRef {
			v.Spec.SecretRef = &meta.LocalObjectReference{
				Name: "flux-system",
			}
		}

	case *sourcev1.OCIRepository:
		SetDefaults_Flux_OCIRepository(v)

		// If secretResourceName is set but secretRef is not, create default secretRef
		hasSecretRef := v.Spec.SecretRef != nil && v.Spec.SecretRef.Name != ""
		hasSecretResourceName := ptr.Deref(obj.SecretResourceName, "") != ""
		if hasSecretResourceName && !hasSecretRef {
			v.Spec.SecretRef = &meta.LocalObjectReference{
				Name: "flux-system",
			}
		}
	}

	// Re-encode if we modified the object
	if !equality.Semantic.DeepEqual(oldSource, sourceObj) {
		if encoded, err := encodeSourceTemplate(sourceObj); err == nil {
			obj.Template = encoded
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
