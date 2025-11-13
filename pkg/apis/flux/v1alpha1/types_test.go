package v1alpha1_test

import (
	"encoding/json"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"

	. "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
)

var _ = Describe("Source Type", func() {
	Describe("Source.Template with GitRepository", func() {
		It("should marshal and unmarshal correctly", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					URL: "https://github.com/example/repo",
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
				},
			}

			source := &Source{
				Template:           encodeSourceTemplate(gitRepo),
				SecretResourceName: ptr.To("my-secret"),
			}

			// Marshal to JSON
			data, err := json.Marshal(source)
			Expect(err).NotTo(HaveOccurred())

			// Unmarshal back
			var unmarshaled Source
			err = json.Unmarshal(data, &unmarshaled)
			Expect(err).NotTo(HaveOccurred())

			// Verify Template is set
			Expect(unmarshaled.Template).NotTo(BeNil())

			// Decode and verify content
			decodedGit := decodeSourceTemplate(unmarshaled.Template).(*sourcev1.GitRepository)
			Expect(decodedGit.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(decodedGit.Spec.Reference.Branch).To(Equal("main"))
			Expect(unmarshaled.SecretResourceName).To(Equal(ptr.To("my-secret")))
		})
	})

	Describe("Source.Template with OCIRepository", func() {
		It("should marshal and unmarshal correctly", func() {
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
				},
			}

			source := &Source{
				Template:           encodeSourceTemplate(ociRepo),
				SecretResourceName: ptr.To("oci-secret"),
			}

			// Marshal to JSON
			data, err := json.Marshal(source)
			Expect(err).NotTo(HaveOccurred())

			// Unmarshal back
			var unmarshaled Source
			err = json.Unmarshal(data, &unmarshaled)
			Expect(err).NotTo(HaveOccurred())

			// Verify Template is set
			Expect(unmarshaled.Template).NotTo(BeNil())

			// Decode and verify content
			decodedOCI := decodeSourceTemplate(unmarshaled.Template).(*sourcev1.OCIRepository)
			Expect(decodedOCI.Spec.URL).To(Equal("oci://ghcr.io/example/manifests"))
			Expect(decodedOCI.Spec.Reference.Tag).To(Equal("v1.0.0"))
			Expect(unmarshaled.SecretResourceName).To(Equal(ptr.To("oci-secret")))
		})
	})

	Describe("Source with empty Template", func() {
		It("should handle nil template", func() {
			source := &Source{
				SecretResourceName: ptr.To("my-secret"),
			}

			// Marshal to JSON
			data, err := json.Marshal(source)
			Expect(err).NotTo(HaveOccurred())

			// Unmarshal back
			var unmarshaled Source
			err = json.Unmarshal(data, &unmarshaled)
			Expect(err).NotTo(HaveOccurred())

			Expect(unmarshaled.Template).To(BeNil())
			Expect(unmarshaled.SecretResourceName).To(Equal(ptr.To("my-secret")))
		})
	})

	Describe("FluxConfig with Source", func() {
		It("should work with GitRepository template", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					URL: "https://github.com/example/repo",
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
				},
			}

			config := &FluxConfig{
				Source: &Source{
					Template: encodeSourceTemplate(gitRepo),
				},
			}

			Expect(config.Source.Template).NotTo(BeNil())

			// Verify content
			decoded := decodeSourceTemplate(config.Source.Template).(*sourcev1.GitRepository)
			Expect(decoded.Spec.URL).To(Equal("https://github.com/example/repo"))
		})

		It("should work with OCIRepository template", func() {
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
				},
			}

			config := &FluxConfig{
				Source: &Source{
					Template: encodeSourceTemplate(ociRepo),
				},
			}

			Expect(config.Source.Template).NotTo(BeNil())

			// Verify content
			decoded := decodeSourceTemplate(config.Source.Template).(*sourcev1.OCIRepository)
			Expect(decoded.Spec.URL).To(Equal("oci://ghcr.io/example/manifests"))
		})
	})
})

// Helper functions for encoding/decoding source templates

var (
	testScheme  = runtime.NewScheme()
	testEncoder runtime.Encoder
	testDecoder runtime.Decoder
)

func init() {
	_ = sourcev1.AddToScheme(testScheme)
	codecFactory := serializer.NewCodecFactory(testScheme)
	testEncoder = codecFactory.LegacyCodec(sourcev1.GroupVersion)
	testDecoder = codecFactory.UniversalDeserializer()
}

func encodeSourceTemplate(obj runtime.Object) *runtime.RawExtension {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "" {
		kind := sourcev1.GitRepositoryKind
		if _, ok := obj.(*sourcev1.OCIRepository); ok {
			kind = sourcev1.OCIRepositoryKind
		}
		obj.GetObjectKind().SetGroupVersionKind(sourcev1.GroupVersion.WithKind(kind))
	}

	raw, err := runtime.Encode(testEncoder, obj)
	if err != nil {
		panic(err)
	}

	return &runtime.RawExtension{Raw: raw}
}

func decodeSourceTemplate(raw *runtime.RawExtension) runtime.Object {
	if raw == nil || raw.Raw == nil {
		return nil
	}

	obj, _, err := testDecoder.Decode(raw.Raw, nil, nil)
	if err != nil {
		panic(err)
	}
	return obj
}
