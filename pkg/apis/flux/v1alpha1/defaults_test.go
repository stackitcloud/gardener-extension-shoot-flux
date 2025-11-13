package v1alpha1

import (
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
)

var _ = Describe("FluxConfig defaulting", func() {
	var obj *FluxConfig

	BeforeEach(func() {
		gitRepo := &sourcev1.GitRepository{
			Spec: sourcev1.GitRepositorySpec{
				Reference: &sourcev1.GitRepositoryRef{
					Branch: "main",
				},
				URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
			},
		}

		obj = &FluxConfig{
			Source: &Source{
				Template: encodeSourceTemplateForTest(gitRepo),
			},
			Kustomization: &Kustomization{
				Template: kustomizev1.Kustomization{
					Spec: kustomizev1.KustomizationSpec{
						Path: "clusters/production/flux-system",
					},
				},
			},
		}
	})

	It("should not overwrite required fields", func() {
		before := obj.DeepCopy()

		SetObjectDefaults_FluxConfig(obj)

		// Decode to check that required fields weren't overwritten
		beforeGit := decodeSourceTemplateForTest(before.Source.Template).(*sourcev1.GitRepository)
		afterGit := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)

		Expect(afterGit.Spec.Reference).To(DeepEqual(beforeGit.Spec.Reference))
		Expect(afterGit.Spec.URL).To(DeepEqual(beforeGit.Spec.URL))
		Expect(obj.Kustomization.Template.Spec.Path).To(DeepEqual(before.Kustomization.Template.Spec.Path))
	})

	Describe("FluxInstallation defaulting", func() {
		It("should default all standard fields", func() {
			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Flux).To(DeepEqual(&FluxInstallation{
				Version:   ptr.To(defaultFluxVersion),
				Registry:  ptr.To("ghcr.io/fluxcd"),
				Namespace: ptr.To("flux-system"),
			}))
		})
	})

	Describe("Source defaulting", func() {
		It("should default all standard fields for GitRepository", func() {
			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			gitRepo := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)

			Expect(gitRepo.Name).To(Equal("flux-system"))
			Expect(gitRepo.Namespace).To(Equal("flux-system"))
			Expect(gitRepo.Spec.Interval.Duration).To(Equal(time.Minute))
		})

		It("should default secretRef.name to flux-system if secretResourceName is set", func() {
			obj.Source.SecretResourceName = ptr.To("my-flux-secret")

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			gitRepo := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)

			Expect(gitRepo.Spec.SecretRef).NotTo(BeNil())
			Expect(gitRepo.Spec.SecretRef.Name).To(Equal("flux-system"))
		})

		It("should not overwrite secretRef.name if secretResourceName is set", func() {
			// Create GitRepository with explicit SecretRef
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
					URL:       "https://github.com/fluxcd/flux2-kustomize-helm-example",
					SecretRef: &meta.LocalObjectReference{Name: "flux-secret"},
				},
			}
			obj.Source.Template = encodeSourceTemplateForTest(gitRepo)
			obj.Source.SecretResourceName = ptr.To("my-flux-secret")

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			gitRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)

			Expect(gitRepoAfter.Spec.SecretRef).NotTo(BeNil())
			Expect(gitRepoAfter.Spec.SecretRef.Name).To(Equal("flux-secret"))
		})

		It("should handle if the kustomization is omitted", func() {
			obj.Kustomization = nil
			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			gitRepo := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)

			Expect(gitRepo.Name).To(Equal("flux-system"))
			Expect(gitRepo.Namespace).To(Equal("flux-system"))
		})

		It("should default all standard fields for OCIRepository", func() {
			// Switch to OCI repository
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
				},
			}
			obj.Source = &Source{
				Template: encodeSourceTemplateForTest(ociRepo),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			ociRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.OCIRepository)

			Expect(ociRepoAfter.Name).To(Equal("flux-system"))
			Expect(ociRepoAfter.Namespace).To(Equal("flux-system"))
			Expect(ociRepoAfter.Spec.Interval.Duration).To(Equal(time.Minute))
		})

		It("should default secretRef.name to flux-system for OCIRepository if secretResourceName is set", func() {
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
				},
			}
			obj.Source = &Source{
				Template:           encodeSourceTemplateForTest(ociRepo),
				SecretResourceName: ptr.To("my-oci-secret"),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			ociRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.OCIRepository)

			Expect(ociRepoAfter.Spec.SecretRef).NotTo(BeNil())
			Expect(ociRepoAfter.Spec.SecretRef.Name).To(Equal("flux-system"))
		})

		It("should not overwrite secretRef.name for OCIRepository if secretResourceName is set", func() {
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
					SecretRef: &meta.LocalObjectReference{Name: "oci-secret"},
				},
			}
			obj.Source = &Source{
				Template:           encodeSourceTemplateForTest(ociRepo),
				SecretResourceName: ptr.To("my-oci-secret"),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			ociRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.OCIRepository)

			Expect(ociRepoAfter.Spec.SecretRef).NotTo(BeNil())
			Expect(ociRepoAfter.Spec.SecretRef.Name).To(Equal("oci-secret"))
		})

		It("should handle OCIRepository when kustomization is omitted", func() {
			ociRepo := &sourcev1.OCIRepository{
				Spec: sourcev1.OCIRepositorySpec{
					URL: "oci://ghcr.io/example/manifests",
					Reference: &sourcev1.OCIRepositoryRef{
						Tag: "v1.0.0",
					},
				},
			}
			obj.Source = &Source{
				Template: encodeSourceTemplateForTest(ociRepo),
			}
			obj.Kustomization = nil

			SetObjectDefaults_FluxConfig(obj)

			// Decode to check defaults
			ociRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.OCIRepository)

			Expect(ociRepoAfter.Name).To(Equal("flux-system"))
			Expect(ociRepoAfter.Namespace).To(Equal("flux-system"))
		})

		// The new API format uses Source.Template (runtime.RawExtension) directly,
		// no separate GitRepository/OCIRepository wrapper structs needed
		It("should work with GitRepository encoded in Template", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
					URL: "https://github.com/example/repo",
				},
			}
			obj.Source = &Source{
				Template: encodeSourceTemplateForTest(gitRepo),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Verify defaults were applied
			gitRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)
			Expect(gitRepoAfter.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(gitRepoAfter.Spec.Reference.Branch).To(Equal("main"))
		})

		It("should apply secretRef defaults when secretResourceName is set", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
					URL: "https://github.com/example/repo",
				},
			}
			obj.Source = &Source{
				Template:           encodeSourceTemplateForTest(gitRepo),
				SecretResourceName: ptr.To("my-secret"),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Verify defaults were applied
			gitRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)
			Expect(gitRepoAfter.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(gitRepoAfter.Spec.SecretRef).NotTo(BeNil())
			Expect(gitRepoAfter.Spec.SecretRef.Name).To(Equal("flux-system"))
		})

		It("should not overwrite explicit secretRef.name", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
					URL:       "https://github.com/example/repo",
					SecretRef: &meta.LocalObjectReference{Name: "my-custom-secret"},
				},
			}
			obj.Source = &Source{
				Template:           encodeSourceTemplateForTest(gitRepo),
				SecretResourceName: ptr.To("my-secret"),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Verify custom secretRef was not overwritten
			gitRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)
			Expect(gitRepoAfter.Spec.SecretRef.Name).To(Equal("my-custom-secret"))
		})

		It("should apply defaults to empty template", func() {
			gitRepo := &sourcev1.GitRepository{
				Spec: sourcev1.GitRepositorySpec{
					Reference: &sourcev1.GitRepositoryRef{
						Branch: "main",
					},
					URL: "https://github.com/example/repo",
				},
			}
			obj.Source = &Source{
				Template: encodeSourceTemplateForTest(gitRepo),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Should have defaults applied
			gitRepoAfter := decodeSourceTemplateForTest(obj.Source.Template).(*sourcev1.GitRepository)
			Expect(gitRepoAfter.Name).To(Equal("flux-system"))
			Expect(gitRepoAfter.Namespace).To(Equal("flux-system"))
			Expect(gitRepoAfter.Spec.Interval.Duration).To(Equal(time.Minute))
		})
	})

	Describe("Kustomization defaulting", func() {
		It("should default all standard fields", func() {
			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Kustomization.Template.Name).To(Equal("flux-system"))
			Expect(obj.Kustomization.Template.Namespace).To(Equal("flux-system"))
			Expect(obj.Kustomization.Template.Spec.Interval.Duration).To(Equal(time.Minute))
		})
		It("should handle if the source is omitted", func() {
			obj.Source = nil
			SetObjectDefaults_FluxConfig(obj)
			Expect(obj.Kustomization.Template.Name).To(Equal("flux-system"))
			Expect(obj.Kustomization.Template.Namespace).To(Equal("flux-system"))
		})
	})
})

// Helper functions for encoding/decoding source templates in tests

var (
	// testScheme is used for encoding/decoding source templates in tests
	testScheme  = runtime.NewScheme()
	testEncoder runtime.Encoder
	testDecoder runtime.Decoder
)

func init() {
	// Register Flux source types for test encoding/decoding
	_ = sourcev1.AddToScheme(testScheme)
	codecFactory := serializer.NewCodecFactory(testScheme)
	testEncoder = codecFactory.LegacyCodec(sourcev1.GroupVersion)
	testDecoder = codecFactory.UniversalDeserializer()
}

// encodeSourceTemplateForTest encodes a Flux source object to runtime.RawExtension for use in tests.
func encodeSourceTemplateForTest(obj runtime.Object) *runtime.RawExtension {
	// Set the proper GVK if not already set
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

// decodeSourceTemplateForTest decodes a runtime.RawExtension into a Flux source object for testing.
func decodeSourceTemplateForTest(raw *runtime.RawExtension) runtime.Object {
	if raw == nil || raw.Raw == nil {
		return nil
	}

	obj, _, err := testDecoder.Decode(raw.Raw, nil, nil)
	if err != nil {
		panic(err)
	}
	return obj
}
