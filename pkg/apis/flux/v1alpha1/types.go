package v1alpha1

import (
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxConfig specifies how to bootstrap Flux on the shoot cluster.
// When both "Source" and "Kustomization" are provided they are also installed in the shoot.
// Otherwise, only Flux itself is installed with no Objects to reconcile.
type FluxConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Flux configures the Flux installation in the Shoot cluster.
	// +optional
	Flux *FluxInstallation `json:"flux,omitempty"`
	// Source configures how to bootstrap a Flux source object.
	// If provided, a "Kustomization" must also be provided.
	// +optional
	Source *Source `json:"source,omitempty"`
	// Kustomization configures how to bootstrap a Flux Kustomization object.
	// If provided, "Source" must also be provided.
	// +optional
	Kustomization *Kustomization `json:"kustomization,omitempty"`

	// SyncMode of the Flux installation. Possible values are:
	//  - Once: Installs Flux and the provided manifests only once, afterwards the extension never looks into the Shoot or updates the extension.
	//  - ManifestsOnly: Installs Flux once, and updates the manifests if they have changed.
	//    For this to work without conflicts, you need to make sure the extension and flux agree on the desired state.
	//
	// Defaults to Once. +optional
	SyncMode SyncMode `json:"syncMode,omitempty"`

	// AdditionalSecretResourceNames to sync to the shoot.
	AdditionalSecretResources []AdditionalResource `json:"additionalSecretResources,omitempty"`
}

// AdditionalResource to sync to the shoot.
type AdditionalResource struct {
	// Name references a resource under Shoot.spec.resources.
	Name string `json:"name"`
	// TargetName optionally overwrites the name of the secret in the shoot.
	// +optional
	TargetName *string `json:"targetName,omitempty"`
}

// SyncMode defines how Flux is reconciled.
type SyncMode string

const (
	// SyncModeOnce only syncs the resources once.
	SyncModeOnce SyncMode = "Once"
	// SyncModeManifestsOnly installs flux once, then only keeps the source and kustomization in sync.
	SyncModeManifestsOnly SyncMode = "ManifestsOnly"
)

// FluxInstallation configures the Flux installation in the Shoot cluster.
type FluxInstallation struct {
	// renovate:flux-version
	// renovate updates the doc string. See renovate config for more details

	// Version specifies the Flux version that should be installed.
	// Defaults to "v2.2.3".
	// +optional
	Version *string `json:"version,omitempty"`
	// Registry specifies the container registry where the Flux controller images are pulled from.
	// Defaults to "ghcr.io/fluxcd".
	// +optional
	Registry *string `json:"registry,omitempty"`
	// Namespace specifes the namespace where Flux should be installed.
	// Defaults to "flux-system".
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// Source configures how to bootstrap a Flux source object.
type Source struct {
	// Template is a partial GitRepository object in API version source.toolkit.fluxcd.io/v1.
	// Required fields: spec.ref.*, spec.url.
	// The following defaults are applied to omitted field:
	// - metadata.name is defaulted to "flux-system"
	// - spec.interval is defaulted to "1m"
	// metadata.namespace is always set to the fluxInstallation namespace
	Template sourcev1.GitRepository `json:"template"`
	// SecretResourceName references a resource under Shoot.spec.resources.
	// The secret data from this resource is used to create the GitRepository's credentials secret
	// (GitRepository.spec.secretRef.name) if specified in Template.
	// +optional
	SecretResourceName *string `json:"secretResourceName,omitempty"`
}

// Kustomization configures how to bootstrap a Flux Kustomization object.
type Kustomization struct {
	// Template is a partial Kustomization object in API version kustomize.toolkit.fluxcd.io/v1.
	// Required fields: spec.path.
	// The following defaults are applied to omitted field:
	// - metadata.name is defaulted to "flux-system"
	// - spec.interval is defaulted to "1m"
	// metadata.namespace is always set to the fluxInstallation namespace
	Template kustomizev1.Kustomization `json:"template"`
}
