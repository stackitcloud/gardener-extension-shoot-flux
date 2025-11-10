package v1alpha1_test

import (
	"encoding/json"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
)

var _ = Describe("Source Type", func() {
	Describe("JSON marshaling", func() {
		Context("with GitRepository source", func() {
			It("should marshal and unmarshal correctly", func() {
				source := &Source{
					GitRepository: &GitRepositorySource{
						Template: sourcev1.GitRepository{
							Spec: sourcev1.GitRepositorySpec{
								URL: "https://github.com/example/repo",
								Reference: &sourcev1.GitRepositoryRef{
									Branch: "main",
								},
							},
						},
						SecretResourceName: ptr.To("my-secret"),
					},
				}

				// Marshal to JSON
				data, err := json.Marshal(source)
				Expect(err).NotTo(HaveOccurred())

				// Unmarshal back
				var unmarshaled Source
				err = json.Unmarshal(data, &unmarshaled)
				Expect(err).NotTo(HaveOccurred())

				// Verify GitRepository is set
				Expect(unmarshaled.GitRepository).NotTo(BeNil())
				Expect(unmarshaled.GitRepository.Template.Spec.URL).To(Equal("https://github.com/example/repo"))
				Expect(unmarshaled.GitRepository.Template.Spec.Reference.Branch).To(Equal("main"))
				Expect(unmarshaled.GitRepository.SecretResourceName).To(Equal(ptr.To("my-secret")))

				// Verify OCIRepository is nil
				Expect(unmarshaled.OCIRepository).To(BeNil())
			})
		})

		Context("with OCIRepository source", func() {
			It("should marshal and unmarshal correctly", func() {
				source := &Source{
					OCIRepository: &OCIRepositorySource{
						Template: sourcev1.OCIRepository{
							Spec: sourcev1.OCIRepositorySpec{
								URL: "oci://ghcr.io/example/manifests",
								Reference: &sourcev1.OCIRepositoryRef{
									Tag: "v1.0.0",
								},
							},
						},
						SecretResourceName: ptr.To("oci-secret"),
					},
				}

				// Marshal to JSON
				data, err := json.Marshal(source)
				Expect(err).NotTo(HaveOccurred())

				// Unmarshal back
				var unmarshaled Source
				err = json.Unmarshal(data, &unmarshaled)
				Expect(err).NotTo(HaveOccurred())

				// Verify OCIRepository is set
				Expect(unmarshaled.OCIRepository).NotTo(BeNil())
				Expect(unmarshaled.OCIRepository.Template.Spec.URL).To(Equal("oci://ghcr.io/example/manifests"))
				Expect(unmarshaled.OCIRepository.Template.Spec.Reference.Tag).To(Equal("v1.0.0"))
				Expect(unmarshaled.OCIRepository.SecretResourceName).To(Equal(ptr.To("oci-secret")))

				// Verify GitRepository is nil
				Expect(unmarshaled.GitRepository).To(BeNil())
			})
		})

		Context("with both GitRepository and OCIRepository", func() {
			It("should preserve both when marshaling (validation will catch this)", func() {
				source := &Source{
					GitRepository: &GitRepositorySource{
						Template: sourcev1.GitRepository{
							Spec: sourcev1.GitRepositorySpec{
								URL: "https://github.com/example/repo",
								Reference: &sourcev1.GitRepositoryRef{
									Branch: "main",
								},
							},
						},
					},
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

				// Marshal to JSON
				data, err := json.Marshal(source)
				Expect(err).NotTo(HaveOccurred())

				// Unmarshal back
				var unmarshaled Source
				err = json.Unmarshal(data, &unmarshaled)
				Expect(err).NotTo(HaveOccurred())

				// Both should be set (validation will reject this later)
				Expect(unmarshaled.GitRepository).NotTo(BeNil())
				Expect(unmarshaled.OCIRepository).NotTo(BeNil())
			})
		})

		Context("with neither GitRepository nor OCIRepository", func() {
			It("should handle empty source", func() {
				source := &Source{}

				// Marshal to JSON
				data, err := json.Marshal(source)
				Expect(err).NotTo(HaveOccurred())

				// Unmarshal back
				var unmarshaled Source
				err = json.Unmarshal(data, &unmarshaled)
				Expect(err).NotTo(HaveOccurred())

				// Both should be nil
				Expect(unmarshaled.GitRepository).To(BeNil())
				Expect(unmarshaled.OCIRepository).To(BeNil())
			})
		})
	})

	Describe("GitRepositorySource", func() {
		It("should have Template and SecretResourceName fields", func() {
			source := &GitRepositorySource{
				Template: sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						URL: "https://github.com/example/repo",
					},
				},
				SecretResourceName: ptr.To("my-secret"),
			}

			Expect(source.Template.Spec.URL).To(Equal("https://github.com/example/repo"))
			Expect(source.SecretResourceName).To(Equal(ptr.To("my-secret")))
		})

		It("should allow nil SecretResourceName", func() {
			source := &GitRepositorySource{
				Template: sourcev1.GitRepository{
					Spec: sourcev1.GitRepositorySpec{
						URL: "https://github.com/example/repo",
					},
				},
			}

			Expect(source.SecretResourceName).To(BeNil())
		})
	})

	Describe("OCIRepositorySource", func() {
		It("should have Template and SecretResourceName fields", func() {
			source := &OCIRepositorySource{
				Template: sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
					},
				},
				SecretResourceName: ptr.To("oci-secret"),
			}

			Expect(source.Template.Spec.URL).To(Equal("oci://ghcr.io/example/manifests"))
			Expect(source.SecretResourceName).To(Equal(ptr.To("oci-secret")))
		})

		It("should allow nil SecretResourceName", func() {
			source := &OCIRepositorySource{
				Template: sourcev1.OCIRepository{
					Spec: sourcev1.OCIRepositorySpec{
						URL: "oci://ghcr.io/example/manifests",
					},
				},
			}

			Expect(source.SecretResourceName).To(BeNil())
		})
	})

	Describe("FluxConfig with new Source types", func() {
		It("should work with GitRepository source", func() {
			config := &FluxConfig{
				Source: &Source{
					GitRepository: &GitRepositorySource{
						Template: sourcev1.GitRepository{
							Spec: sourcev1.GitRepositorySpec{
								URL: "https://github.com/example/repo",
								Reference: &sourcev1.GitRepositoryRef{
									Branch: "main",
								},
							},
						},
					},
				},
			}

			Expect(config.Source.GitRepository).NotTo(BeNil())
			Expect(config.Source.OCIRepository).To(BeNil())
		})

		It("should work with OCIRepository source", func() {
			config := &FluxConfig{
				Source: &Source{
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
				},
			}

			Expect(config.Source.OCIRepository).NotTo(BeNil())
			Expect(config.Source.GitRepository).To(BeNil())
		})
	})
})
