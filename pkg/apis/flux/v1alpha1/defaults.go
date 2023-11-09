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
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_FluxConfig(obj *FluxConfig) {
	if obj.Flux == nil {
		obj.Flux = &FluxInstallation{}
	}

	if sourceName := obj.Source.Template.Name; obj.Kustomization.Template.Spec.SourceRef.Name == "" && sourceName != "" {
		obj.Kustomization.Template.Spec.SourceRef.Name = sourceName
	}
	if sourceNamespace := obj.Source.Template.Namespace; obj.Kustomization.Template.Spec.SourceRef.Namespace == "" && sourceNamespace != "" {
		obj.Kustomization.Template.Spec.SourceRef.Namespace = sourceNamespace
	}

	if namespace := ptr.Deref(obj.Flux.Namespace, ""); namespace != "" {
		if obj.Source.Template.Namespace == "" {
			obj.Source.Template.Namespace = namespace
		}
		if obj.Kustomization.Template.Namespace == "" {
			obj.Kustomization.Template.Namespace = namespace
		}
		if obj.Kustomization.Template.Spec.SourceRef.Namespace == "" {
			obj.Kustomization.Template.Spec.SourceRef.Namespace = namespace
		}
	}
}

func SetDefaults_FluxInstallation(obj *FluxInstallation) {
	if obj.Version == nil {
		// TODO: add renovate to upgrade this in lockstep with go modules update
		//  If you do automate this, also automate updating the API doc string ;)
		obj.Version = ptr.To("v2.1.2")
	}

	if obj.Registry == nil {
		obj.Registry = ptr.To("ghcr.io/fluxcd")
	}

	if obj.Namespace == nil {
		obj.Namespace = ptr.To(defaultFluxNamespace)
	}
}

func SetDefaults_Source(obj *Source) {
	SetDefaults_Flux_GitRepository(&obj.Template)

	hasSecretRef := obj.Template.Spec.SecretRef != nil && obj.Template.Spec.SecretRef.Name != ""
	hasSecretResourceName := ptr.Deref(obj.SecretResourceName, "") != ""
	if hasSecretResourceName && !hasSecretRef {
		obj.Template.Spec.SecretRef = &meta.LocalObjectReference{
			Name: "flux-system",
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
