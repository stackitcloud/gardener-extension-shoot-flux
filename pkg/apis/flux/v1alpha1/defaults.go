package v1alpha1

import (
	"encoding/json"
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
)

const (
	defaultFluxNamespace     = "flux-system"
	defaultGitRepositoryName = "flux-system"

	// defaultFluxVersion is maintained by renovate via a customManager. Don't
	// change this without also updating the renovate config.
	defaultFluxVersion = "v2.7.5"
)

var (
	// defaultingScheme is used for encoding/decoding source templates
	defaultingScheme  = runtime.NewScheme()
	defaultingEncoder runtime.Encoder
	defaultingDecoder runtime.Decoder
)

func init() {
	// Register Flux source types for defaulting
	_ = sourcev1.AddToScheme(defaultingScheme)
	codecFactory := serializer.NewCodecFactory(defaultingScheme)
	defaultingEncoder = codecFactory.LegacyCodec(sourcev1.GroupVersion)
	defaultingDecoder = codecFactory.UniversalDeserializer()
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// decodeSourceTemplateForDefaulting decodes a runtime.RawExtension into a Flux source object.
// Returns the decoded object or nil if decoding fails (fails silently to not break defaulting).
func decodeSourceTemplateForDefaulting(raw *runtime.RawExtension) runtime.Object {
	if raw == nil || raw.Raw == nil {
		return nil
	}

	//  Peek at TypeMeta to get GVK
	typeMeta := &metav1.TypeMeta{}
	if err := json.Unmarshal(raw.Raw, typeMeta); err != nil {
		return nil
	}

	gvk := typeMeta.GroupVersionKind()
	if gvk.Kind == "" {
		return nil
	}

	// Create object and decode
	obj, err := defaultingScheme.New(gvk)
	if err != nil {
		return nil
	}

	if err := runtime.DecodeInto(defaultingDecoder, raw.Raw, obj); err != nil {
		return nil
	}

	return obj
}

// encodeSourceTemplate encodes a Flux source object back to runtime.RawExtension.
func encodeSourceTemplate(obj runtime.Object) (*runtime.RawExtension, error) {
	raw, err := runtime.Encode(defaultingEncoder, obj)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: raw}, nil
}

func SetDefaults_FluxConfig(obj *FluxConfig) {
	if obj.Flux == nil {
		obj.Flux = &FluxInstallation{}
	}

	// validation will ensure that both Source & Kustomization are set or both
	// are nil, but we have to handle all cases, since defaulting happens first.
	if obj.Source != nil && obj.Kustomization != nil {
		// Decode source template to get name and namespace
		sourceObj := decodeSourceTemplateForDefaulting(obj.Source.Template)
		if sourceObj != nil {
			var sourceName, sourceNamespace string
			switch v := sourceObj.(type) {
			case *sourcev1.GitRepository:
				sourceName = v.Name
				sourceNamespace = v.Namespace
			case *sourcev1.OCIRepository:
				sourceName = v.Name
				sourceNamespace = v.Namespace
			}

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
			sourceObj := decodeSourceTemplateForDefaulting(obj.Source.Template)
			if sourceObj != nil {
				modified := false
				switch v := sourceObj.(type) {
				case *sourcev1.GitRepository:
					if v.Namespace == "" {
						v.Namespace = namespace
						modified = true
					}
				case *sourcev1.OCIRepository:
					if v.Namespace == "" {
						v.Namespace = namespace
						modified = true
					}
				}
				if modified {
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
	sourceObj := decodeSourceTemplateForDefaulting(obj.Template)
	if sourceObj == nil {
		return
	}

	modified := false

	// Apply defaults based on source type
	switch v := sourceObj.(type) {
	case *sourcev1.GitRepository:
		SetDefaults_Flux_GitRepository(v)
		modified = true

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
		modified = true

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
	if modified {
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
