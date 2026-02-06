package validation_test

import (
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	. "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
	. "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1/validation"
)

var _ = Describe("FluxConfig validation", func() {
	var (
		rootFldPath *field.Path
		fluxConfig  *FluxConfig
		shoot       *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		rootFldPath = field.NewPath("root")

		gitRepoTemplate := encodeSourceTemplate(&sourcev1.GitRepository{
			Spec: sourcev1.GitRepositorySpec{
				Reference: &sourcev1.GitRepositoryRef{
					Branch: "main",
				},
				URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
			},
		})

		fluxConfig = &FluxConfig{
			Source: &Source{
				Template: gitRepoTemplate,
			},
			Kustomization: &Kustomization{
				Template: kustomizev1.Kustomization{
					Spec: kustomizev1.KustomizationSpec{
						Path: "clusters/production/flux-system",
					},
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{}
	})

	It("should allow basic valid object", func() {
		Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
	})

	It("should allow having neither source nor kustomization", func() {
		fluxConfig.Source = nil
		fluxConfig.Kustomization = nil
		Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
	})

	Describe("FluxInstallation validation", func() {
		BeforeEach(func() {
			fluxConfig.Flux = &FluxInstallation{}
		})

		It("should allow valid namespace names", func() {
			fluxConfig.Flux.Namespace = ptr.To("my-flux-system")

			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
		})

		It("should deny invalid namespace names", func() {
			fluxConfig.Flux.Namespace = ptr.To("this is definitely not a valid namespace")

			Expect(
				ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
			).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("root.flux.namespace"),
			}))))
		})

		It("should check if the required components are present", func() {
			fluxConfig.Flux.Components = []string{"kustomize-controller", "foo-controller"}
			fluxConfig.Flux.ComponentsExtra = []string{"source-controller"}
			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
		})

		It("should error if a required component is present", func() {
			fluxConfig.Flux.Components = []string{"kustomize-controller"}
			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("root.flux.components"),
					"Detail": Equal("missing required component source-controller"),
				})),
			))
		})
	})

	Describe("Source validation", func() {
		It("should deny only omitting the source", func() {
			fluxConfig.Source = nil
			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.source"),
				}))))
		})
		Describe("Git TypeMeta validation", func() {
			It("should allow using supported apiVersion and kind", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow omitting apiVersion and kind", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				// Explicitly clear GVK to test omitting it
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should validate based on decoded template type", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(BeEmpty())
			})
		})

		Describe("Git Reference validation", func() {
			It("should forbid omitting reference", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: nil,
						URL:       "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.ref"),
				}))))
			})

			It("should forbid specifying empty reference", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{},
						URL:       "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.ref"),
				}))))
			})

			It("should allow setting any reference", func() {
				test := func(mutate func(ref *sourcev1.GitRepositoryRef)) {
					gitRepo := &sourcev1.GitRepository{
						Spec: sourcev1.GitRepositorySpec{
							Reference: &sourcev1.GitRepositoryRef{},
							URL:       "https://github.com/fluxcd/flux2-kustomize-helm-example",
						},
					}
					mutate(gitRepo.Spec.Reference)
					fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

					ExpectWithOffset(1, ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
				}

				test(func(ref *sourcev1.GitRepositoryRef) { ref.Branch = "develop" })
				test(func(ref *sourcev1.GitRepositoryRef) { ref.Tag = "latest" })
				test(func(ref *sourcev1.GitRepositoryRef) { ref.SemVer = "v1.0.0" })
				test(func(ref *sourcev1.GitRepositoryRef) { ref.Name = "refs/tags/v1.0.0" })
				test(func(ref *sourcev1.GitRepositoryRef) { ref.Commit = "9c36b9c4bb6438104a703cb983aa8c62ff5e7c4c" })
			})
		})

		Describe("Git URL validation", func() {
			It("should forbid omitting URL", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.url"),
				}))))
			})
		})

		Describe("Git Secret validation", func() {
			It("should allow omitting both secretRef and secretResourceName", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow specifying both secretRef and secretResourceName", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
						SecretRef: &meta.LocalObjectReference{
							Name: "flux-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-flux-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-flux-secret",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "Secret",
					},
				}}

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should deny specifying a secretResourceName without a matching resource", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
						SecretRef: &meta.LocalObjectReference{
							Name: "flux-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-flux-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-other-secret",
				}}

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.source.secretResourceName"),
				}))))
			})

			It("should deny omitting secretRef if secretResourceName is set", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL:       "https://github.com/fluxcd/flux2-kustomize-helm-example",
						SecretRef: nil,
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-flux-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-flux-secret",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "Secret",
					},
				}}

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.secretRef"),
				}))))
			})

			It("should deny omitting secretResourceName if secretRef is set", func() {
				gitRepo := &sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						Reference: &sourcev1.GitRepositoryRef{
							Branch: "main",
						},
						URL: "https://github.com/fluxcd/flux2-kustomize-helm-example",
						SecretRef: &meta.LocalObjectReference{
							Name: "flux-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(gitRepo)
				fluxConfig.Source.SecretResourceName = nil

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.secretResourceName"),
				}))))
			})
		})

		Describe("Source mutex validation", func() {
			It("should deny having both GitRepository and OCIRepository set", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(BeEmpty())
			})

			It("should deny having neither GitRepository nor OCIRepository set", func() {
				fluxConfig.Source.Template = nil

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template"),
				}))))
			})
		})

		Describe("OCI TypeMeta validation", func() {
			BeforeEach(func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source = &Source{
					Template: encodeSourceTemplate(ociRepo),
				}
			})

			It("should allow using supported apiVersion and kind", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow omitting apiVersion and kind", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				// Explicitly clear GVK
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should validate OCI template based on decoded type", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(BeEmpty())
			})
		})

		Describe("OCI Reference validation", func() {
			BeforeEach(func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source = &Source{
					Template: encodeSourceTemplate(ociRepo),
				}
			})

			It("should forbid omitting reference", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL:       "oci://ghcr.io/example/manifests",
						Reference: nil,
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.ref"),
				}))))
			})

			It("should forbid specifying empty reference", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL:       "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.source.template.spec.ref"),
				}))))
			})

			It("should allow setting tag reference", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "latest",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow setting semver reference", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							SemVer: ">= 1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow setting digest reference", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Digest: "sha256:abcd1234",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})
		})

		Describe("OCI URL validation", func() {
			BeforeEach(func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source = &Source{
					Template: encodeSourceTemplate(ociRepo),
				}
			})

			It("should forbid omitting URL", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.url"),
				}))))
			})

			It("should forbid non-OCI URL format", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "https://github.com/example/repo",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.source.template.spec.url"),
				}))))
			})

			It("should allow OCI URL format", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})
		})

		Describe("OCI Secret validation", func() {
			BeforeEach(func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
					},
				}
				fluxConfig.Source = &Source{
					Template: encodeSourceTemplate(ociRepo),
				}
			})

			It("should allow omitting both secretRef and secretResourceName", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
						SecretRef: nil,
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)
				fluxConfig.Source.SecretResourceName = nil

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow specifying both secretRef and secretResourceName", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
						SecretRef: &meta.LocalObjectReference{
							Name: "oci-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-oci-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-oci-secret",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "Secret",
					},
				}}

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should deny specifying a secretResourceName without a matching resource", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
						SecretRef: &meta.LocalObjectReference{
							Name: "oci-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-oci-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-other-secret",
				}}

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.source.secretResourceName"),
				}))))
			})

			It("should deny omitting secretRef if secretResourceName is set", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
						SecretRef: nil,
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)
				fluxConfig.Source.SecretResourceName = ptr.To("my-oci-secret")
				shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
					Name: "my-oci-secret",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "Secret",
					},
				}}

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.template.spec.secretRef"),
				}))))
			})

			It("should deny omitting secretResourceName if secretRef is set", func() {
				ociRepo := &sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
						Reference: &sourcev1.OCIRepositoryRef{
							Tag: "v1.0.0",
						},
						SecretRef: &meta.LocalObjectReference{
							Name: "oci-secret",
						},
					},
				}
				fluxConfig.Source.Template = encodeSourceTemplate(ociRepo)
				fluxConfig.Source.SecretResourceName = nil

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.source.secretResourceName"),
				}))))
			})
		})
	})

	Describe("Kustomization validation", func() {
		It("should deny only omitting the kustomization", func() {
			fluxConfig.Kustomization = nil
			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.kustomization"),
				}))))
		})
		Describe("TypeMeta validation", func() {
			It("should allow using supported apiVersion and kind", func() {
				fluxConfig.Kustomization.Template.APIVersion = "kustomize.toolkit.fluxcd.io/v1"
				fluxConfig.Kustomization.Template.Kind = "Kustomization"

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should allow omitting apiVersion and kind", func() {
				fluxConfig.Kustomization.Template.APIVersion = ""
				fluxConfig.Kustomization.Template.Kind = ""

				Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
			})

			It("should deny using unsupported apiVersion", func() {
				fluxConfig.Kustomization.Template.APIVersion = "kustomize.toolkit.fluxcd.io/v2"
				fluxConfig.Kustomization.Template.Kind = "Kustomization"

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("root.kustomization.template.apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("root.kustomization.template.kind"),
					})),
				))
			})

			It("should deny using unsupported kind", func() {
				fluxConfig.Kustomization.Template.APIVersion = "kustomize.toolkit.fluxcd.io/v1"
				fluxConfig.Kustomization.Template.Kind = "HelmRelease"

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("root.kustomization.template.apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("root.kustomization.template.kind"),
					})),
				))
			})
		})

		Describe("Path validation", func() {
			It("should deny omitting path", func() {
				fluxConfig.Kustomization.Template.Spec.Path = ""

				Expect(
					ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
				).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.kustomization.template.spec.path"),
				}))))
			})
		})
	})
	Describe("additionalSecretResources validation", func() {
		It("should allow specifying nothing", func() {
			fluxConfig.AdditionalSecretResources = nil
			Expect(ValidateFluxConfig(fluxConfig, shoot, rootFldPath)).To(BeEmpty())
		})
		It("should find all errors", func() {
			fluxConfig.AdditionalSecretResources = []AdditionalResource{
				{Name: "valid"},
				{Name: "wrong-kind"},
				{Name: "no-ref"},
				{Name: "valid", TargetName: ptr.To("invalid-name-")},
			}
			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "valid",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "Secret",
					},
				},
				{
					Name: "wrong-kind",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind: "ConfigMap",
					},
				},
			}
			Expect(
				ValidateFluxConfig(fluxConfig, shoot, rootFldPath),
			).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("root.additionalSecretResources[1].name"),
					"Detail": ContainSubstring("is not a secret"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("root.additionalSecretResources[2].name"),
					"Detail": ContainSubstring("does not match any of the resource names"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("root.additionalSecretResources[3].targetName"),
					"Detail": ContainSubstring("must be a valid resource name"),
				})),
			))
		})
	})
})

func encodeSourceTemplate(obj runtime.Object) *runtime.RawExtension {
	// If the object already has TypeMeta set (APIVersion and Kind), use those.
	// Otherwise, auto-detect based on object type.
	existing := obj.GetObjectKind().GroupVersionKind()
	if existing.Kind == "" {
		gvk := sourcev1.GroupVersion.WithKind(sourcev1.GitRepositoryKind)
		if _, ok := obj.(*sourcev1.OCIRepository); ok {
			gvk = sourcev1.GroupVersion.WithKind(sourcev1.OCIRepositoryKind)
		}
		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		panic(err)
	}
	return &runtime.RawExtension{Raw: data}
}
