package v1alpha1

import (
	"encoding/json"
	"fmt"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	// sourceTemplateScheme is used for encoding/decoding source templates
	sourceTemplateScheme  = runtime.NewScheme()
	sourceTemplateDecoder runtime.Decoder
	sourceTemplateEncoder runtime.Encoder
)

func init() {
	// Register Flux source types
	_ = sourcev1.AddToScheme(sourceTemplateScheme)
	codecFactory := serializer.NewCodecFactory(sourceTemplateScheme)
	sourceTemplateDecoder = codecFactory.UniversalDeserializer()
	sourceTemplateEncoder = codecFactory.LegacyCodec(sourcev1.GroupVersion)
}

// DecodeSourceTemplate decodes a runtime.RawExtension into a Flux source object.
// Returns the decoded object and its kind string, or an error if decoding fails.
func DecodeSourceTemplate(raw *runtime.RawExtension) (runtime.Object, string, error) {
	if raw == nil || raw.Raw == nil {
		return nil, "", fmt.Errorf("template is required")
	}

	// First peek at the TypeMeta to get the GVK
	typeMeta := &metav1.TypeMeta{}
	if err := json.Unmarshal(raw.Raw, typeMeta); err != nil {
		return nil, "", fmt.Errorf("failed to peek at GVK: %w", err)
	}

	gvk := typeMeta.GroupVersionKind()
	if gvk.Kind == "" {
		return nil, "", fmt.Errorf("could not find 'kind' in template")
	}

	// Decode into the specific type
	obj, err := sourceTemplateScheme.New(gvk)
	if err != nil {
		return nil, gvk.Kind, fmt.Errorf("unsupported source type %v: %w", gvk, err)
	}

	if err := runtime.DecodeInto(sourceTemplateDecoder, raw.Raw, obj); err != nil {
		return nil, gvk.Kind, fmt.Errorf("failed to decode into %v: %w", gvk, err)
	}

	return obj, gvk.Kind, nil
}

// encodeSourceTemplate encodes a Flux source object back to runtime.RawExtension.
func encodeSourceTemplate(obj runtime.Object) (*runtime.RawExtension, error) {
	raw, err := runtime.Encode(sourceTemplateEncoder, obj)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: raw}, nil
}
