package v1alpha1

import (
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("FluxConfig defaulting", func() {
	var obj *FluxConfig

	BeforeEach(func() {
		obj = &FluxConfig{
			Source: &Source{
				GitRepository: &GitRepositorySource{
					Template: sourcev1.GitRepository{
						Spec: sourcev1.GitRepositorySpec{
							Reference: &sourcev1.GitRepositoryRef{
								Branch: "main",
							},
							URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
						},
					},
				},
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

		Expect(obj.Source.GitRepository.Template.Spec.Reference).To(DeepEqual(before.Source.GitRepository.Template.Spec.Reference))
		Expect(obj.Source.GitRepository.Template.Spec.URL).To(DeepEqual(before.Source.GitRepository.Template.Spec.URL))
		Expect(obj.Source.GitRepository.Template.Spec.URL).To(DeepEqual(before.Source.GitRepository.Template.Spec.URL))
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

			Expect(obj.Source.GitRepository.Template.Name).To(Equal("flux-system"))
			Expect(obj.Source.GitRepository.Template.Namespace).To(Equal("flux-system"))
			Expect(obj.Source.GitRepository.Template.Spec.Interval.Duration).To(Equal(time.Minute))
		})

		It("should default secretRef.name to flux-system if secretResourceName is set", func() {
			obj.Source.GitRepository.SecretResourceName = ptr.To("my-flux-secret")

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.GitRepository.Template.Spec.SecretRef).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Spec.SecretRef.Name).To(Equal("flux-system"))
		})

		It("should not overwrite secretRef.name if secretResourceName is set", func() {
			obj.Source.GitRepository.Template.Spec.SecretRef = &meta.LocalObjectReference{Name: "flux-secret"}
			obj.Source.GitRepository.SecretResourceName = ptr.To("my-flux-secret")

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.GitRepository.Template.Spec.SecretRef).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Spec.SecretRef.Name).To(Equal("flux-secret"))
		})

		It("should handle if the kustomization is omitted", func() {
			obj.Kustomization = nil
			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.GitRepository.Template.Name).To(Equal("flux-system"))
			Expect(obj.Source.GitRepository.Template.Namespace).To(Equal("flux-system"))
		})

		It("should default all standard fields for OCIRepository", func() {
			// Switch to OCI repository
			obj.Source = &Source{
				OCIRepository: &OCIRepositorySource{
					Template: sourcev1.OCIRepository{
						Spec: sourcev1.OCIRepositorySpec{
							URL: "oci://ghcr.io/example/manifests",
							Reference: &sourcev1.OCIRepositoryRef{
								Tag: "v1.0.0",
							},
						},
					},
				},
			}

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.OCIRepository.Template.Name).To(Equal("flux-system"))
			Expect(obj.Source.OCIRepository.Template.Namespace).To(Equal("flux-system"))
			Expect(obj.Source.OCIRepository.Template.Spec.Interval.Duration).To(Equal(time.Minute))
		})

		It("should default secretRef.name to flux-system for OCIRepository if secretResourceName is set", func() {
			obj.Source = &Source{
				OCIRepository: &OCIRepositorySource{
					Template: sourcev1.OCIRepository{
						Spec: sourcev1.OCIRepositorySpec{
							URL: "oci://ghcr.io/example/manifests",
							Reference: &sourcev1.OCIRepositoryRef{
								Tag: "v1.0.0",
							},
						},
					},
					SecretResourceName: ptr.To("my-oci-secret"),
				},
			}

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.OCIRepository.Template.Spec.SecretRef).NotTo(BeNil())
			Expect(obj.Source.OCIRepository.Template.Spec.SecretRef.Name).To(Equal("flux-system"))
		})

		It("should not overwrite secretRef.name for OCIRepository if secretResourceName is set", func() {
			obj.Source = &Source{
				OCIRepository: &OCIRepositorySource{
					Template: sourcev1.OCIRepository{
						Spec: sourcev1.OCIRepositorySpec{
							URL: "oci://ghcr.io/example/manifests",
							Reference: &sourcev1.OCIRepositoryRef{
								Tag: "v1.0.0",
							},
							SecretRef: &meta.LocalObjectReference{Name: "oci-secret"},
						},
					},
					SecretResourceName: ptr.To("my-oci-secret"),
				},
			}

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.OCIRepository.Template.Spec.SecretRef).NotTo(BeNil())
			Expect(obj.Source.OCIRepository.Template.Spec.SecretRef.Name).To(Equal("oci-secret"))
		})

		It("should handle OCIRepository when kustomization is omitted", func() {
			obj.Source = &Source{
				OCIRepository: &OCIRepositorySource{
					Template: sourcev1.OCIRepository{
						Spec: sourcev1.OCIRepositorySpec{
							URL: "oci://ghcr.io/example/manifests",
							Reference: &sourcev1.OCIRepositoryRef{
								Tag: "v1.0.0",
							},
						},
					},
				},
			}
			obj.Kustomization = nil

			SetObjectDefaults_FluxConfig(obj)

			Expect(obj.Source.OCIRepository.Template.Name).To(Equal("flux-system"))
			Expect(obj.Source.OCIRepository.Template.Namespace).To(Equal("flux-system"))
		})

		// Backwards compatibility tests for deprecated Template field
		It("should migrate deprecated Template field to GitRepository", func() {
			obj.Source = &Source{
				Template: &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/example/repo",
					},
				},
			}

			SetObjectDefaults_FluxConfig(obj)

			// Should migrate to new format
			Expect(obj.Source.GitRepository).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(obj.Source.GitRepository.Template.Spec.Reference.Branch).To(Equal("main"))

			// Old fields should be cleared
			Expect(obj.Source.Template).To(BeNil())
		})

		It("should migrate deprecated Template and SecretResourceName fields to GitRepository", func() {
			obj.Source = &Source{
				Template: &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/example/repo",
					},
				},
				SecretResourceName: ptr.To("my-secret"),
			}

			SetObjectDefaults_FluxConfig(obj)

			// Should migrate to new format
			Expect(obj.Source.GitRepository).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(obj.Source.GitRepository.SecretResourceName).To(Equal(ptr.To("my-secret")))

			// Should default secretRef
			Expect(obj.Source.GitRepository.Template.Spec.SecretRef).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Spec.SecretRef.Name).To(Equal("flux-system"))

			// Old fields should be cleared
			Expect(obj.Source.Template).To(BeNil())
			Expect(obj.Source.SecretResourceName).To(BeNil())
		})

		It("should not migrate if new GitRepository field is already set", func() {
			obj.Source = &Source{
				GitRepository: &GitRepositorySource{
					Template: sourcev1.GitRepository{
						Spec: sourcev1.GitRepositorySpec{
							Reference: &sourcev1.GitRepositoryRef{
								Branch: "main",
							},
							URL: "https://github.com/new/repo",
						},
					},
				},
				// These should be ignored if GitRepository is set
				Template: &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						URL: "https://github.com/old/repo",
					},
				},
			}

			SetObjectDefaults_FluxConfig(obj)

			// Should keep new format unchanged
			Expect(obj.Source.GitRepository.Template.Spec.URL).To(Equal("https://github.com/new/repo"))
			// Old fields should NOT be migrated
			Expect(obj.Source.Template).NotTo(BeNil()) // Still present but ignored
		})

		It("should handle empty deprecated Template", func() {
			obj.Source = &Source{
				Template: &sourcev1.GitRepository{}, // Empty
			}

			SetObjectDefaults_FluxConfig(obj)

			// Should still migrate but with defaults applied
			Expect(obj.Source.GitRepository).NotTo(BeNil())
			Expect(obj.Source.GitRepository.Template.Name).To(Equal("flux-system"))
			Expect(obj.Source.Template).To(BeNil())
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
