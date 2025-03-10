package extension

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
	"github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1/validation"
)

var _ = Describe("DecodeProviderConfig", func() {
	var (
		scheme     *runtime.Scheme
		fakeClient client.Client
		a          *actuator
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(fluxv1alpha1.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		a = NewActuator(fakeClient).(*actuator)
	})

	Context("valid providerConfig given", func() {
		It("should decode and default providerConfig", func() {
			rawExtension := &runtime.RawExtension{Raw: []byte(`apiVersion: flux.extensions.gardener.cloud/v1alpha1
kind: FluxConfig
flux:
  version: v2.0.0
`)}

			config, err := a.DecodeProviderConfig(rawExtension)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Flux.Version).To(PointTo(Equal("v2.0.0")))
			Expect(config.Flux.Namespace).To(PointTo(Equal("flux-system")))

			Expect(validation.ValidateFluxConfig(config, nil, nil)).
				To(BeEmpty(), "decoded providerConfig should be accepted by validation")
		})
	})

	Context("no providerConfig given", func() {
		It("should default the providerConfig", func() {
			config, err := a.DecodeProviderConfig(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Flux.Namespace).To(PointTo(Equal("flux-system")))

			Expect(validation.ValidateFluxConfig(config, nil, nil)).
				To(BeEmpty(), "defaulted providerConfig should be accepted by validation")
		})
	})
})

var _ = Describe("InstallFlux", func() {
	var (
		tmpDir      string
		shootClient client.Client
		config      *fluxv1alpha1.FluxInstallation
	)
	BeforeEach(func() {
		tmpDir = setupManifests()
		shootClient = newShootClient()
		config = &fluxv1alpha1.FluxInstallation{
			Version:   ptr.To("v2.1.3"),
			Registry:  ptr.To("reg.example.com"),
			Namespace: ptr.To("gotk-system"),
		}
	})
	It("succesfully apply and wait for readiness", func() {
		done := testAsync(func() {
			Expect(
				installFlux(ctx, log, shootClient, config, tmpDir, poll, timeout),
			).To(Succeed())
		})
		Eventually(fakeFluxReady(ctx, shootClient, *config.Namespace)).Should(Succeed())

		Eventually(done).Should(BeClosed())
	})
	It("should fail if the resources do not get ready", func() {
		done := testAsync(func() {
			Expect(
				installFlux(ctx, log, shootClient, config, tmpDir, poll, timeout),
			).To(MatchError(ContainSubstring("error waiting for Flux installation to get ready")))
		})

		Eventually(done).Should(BeClosed())
	})
})

var _ = Describe("BootstrapSource", func() {
	var (
		shootClient client.Client
		config      *fluxv1alpha1.Source
	)
	BeforeEach(func() {
		shootClient = newShootClient()
		config = &fluxv1alpha1.Source{
			Template: sourcev1.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gitrepo",
					Namespace: "custom-namespace",
				},
				Spec: sourcev1.GitRepositorySpec{
					URL: "http://example.com",
				},
			},
		}
	})
	It("should succesfully apply and wait for readiness", func() {
		done := testAsync(func() {
			Expect(
				bootstrapSource(ctx, log, shootClient, config, poll, timeout),
			).To(Succeed())
		})
		repo := config.Template.DeepCopy()
		Eventually(fakeFluxResourceReady(ctx, shootClient, repo)).Should(Succeed())
		Eventually(done).Should(BeClosed())

		createdRepo := &sourcev1.GitRepository{}
		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(repo), createdRepo))
		Expect(createdRepo.Spec.URL).To(Equal("http://example.com"))

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.Template.Namespace}}
		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).Should(Succeed())
	})
	It("should fail if the resources do not get ready", func() {
		Eventually(testAsync(func() {
			Expect(
				bootstrapSource(ctx, log, shootClient, config, poll, timeout),
			).To(MatchError(ContainSubstring("error waiting for GitRepository to get ready")))
		})).Should(BeClosed())
	})
})

var _ = Describe("BootstrapKustomization", func() {
	var (
		shootClient client.Client
		config      *fluxv1alpha1.Kustomization
	)
	BeforeEach(func() {
		shootClient = newShootClient()
		config = &fluxv1alpha1.Kustomization{
			Template: kustomizev1.Kustomization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kustomization",
					Namespace: "custom-namespace",
				},
				Spec: kustomizev1.KustomizationSpec{
					Path: "/some/path",
				},
			},
		}
	})
	It("should succesfully apply and wait for readiness", func() {
		done := testAsync(func() {
			Expect(bootstrapKustomization(ctx, log, shootClient, config, poll, timeout)).To(Succeed())
		})
		ks := config.Template.DeepCopy()
		Eventually(fakeFluxResourceReady(ctx, shootClient, ks)).Should(Succeed())
		Eventually(done).Should(BeClosed())

		createdKS := &kustomizev1.Kustomization{}
		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(ks), createdKS))
		Expect(createdKS.Spec.Path).To(Equal("/some/path"))

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.Template.Namespace}}
		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)).Should(Succeed())
	})
	It("should handle if the namespace already exists", func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.Template.Namespace}}
		Expect(shootClient.Create(ctx, ns)).To(Succeed())

		done := testAsync(func() {
			Expect(bootstrapKustomization(ctx, log, shootClient, config, poll, timeout)).To(Succeed())
		})
		ks := config.Template.DeepCopy()
		Eventually(fakeFluxResourceReady(ctx, shootClient, ks)).Should(Succeed())
		Eventually(done).Should(BeClosed())
	})
	It("should fail if the resources do not get ready", func() {
		Eventually(testAsync(func() {
			Expect(
				bootstrapKustomization(ctx, log, shootClient, config, poll, timeout),
			).To(MatchError(ContainSubstring("error waiting for Kustomization to get ready")))
		})).Should(BeClosed())
	})
})

var _ = Describe("Bootstrapped Condition", func() {
	It("should set and detect a bootstrapped condition", func() {
		seedClient := newSeedClient()
		ext := &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		Expect(seedClient.Create(ctx, ext)).To(Succeed())

		By("being initially false")
		Expect(IsFluxBootstrapped(ext)).To(BeFalse())

		By("setting the bootstrapped condition")
		Expect(SetFluxBootstrapped(ctx, seedClient, ext)).To(Succeed())

		By("reading the condition")
		Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(ext), ext)).To(Succeed())
		Expect(IsFluxBootstrapped(ext)).To(BeTrue())
	})
})

var _ = Describe("GenerateInstallManifest", func() {
	It("should contain the provided options", func() {
		dir := setupManifests()
		out, err := GenerateInstallManifest(&fluxv1alpha1.FluxInstallation{
			Version:   ptr.To("v2.0.0"),
			Registry:  ptr.To("registry.example.com"),
			Namespace: ptr.To("a-namespace"),
		}, dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(And(
			ContainSubstring("v2.0.0"),
			ContainSubstring("registry.example.com"),
			ContainSubstring("a-namespace"),
		))
	})
})

var _ = Describe("ReconcileShootInfoConfigMap", func() {
	var (
		shootName       string
		technicalID     string
		clusterIdentity string
		shootClient     client.Client
		config          *fluxv1alpha1.FluxConfig
		cluster         *extensions.Cluster
		configMap       *corev1.ConfigMap
	)

	BeforeEach(func() {
		shootName = "test-shoot"
		technicalID = fmt.Sprintf("shoot--asdf-test--%s", shootName)
		clusterIdentity = "magic-cluster-identity"
		shootClient = newShootClient()
		config = &fluxv1alpha1.FluxConfig{
			Flux: &fluxv1alpha1.FluxInstallation{
				Namespace: ptr.To("flux-system"),
			},
		}
		cluster = &extensions.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
				Status: gardencorev1beta1.ShootStatus{
					TechnicalID:     technicalID,
					ClusterIdentity: &clusterIdentity,
				},
			},
		}
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootInfoConfigMapName,
				Namespace: *config.Flux.Namespace,
			},
		}
	})

	It("should apply successfully and contain expected keys", func() {
		Expect(ReconcileShootInfoConfigMap(ctx, log, shootClient, config, cluster)).To(Succeed())

		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
		Expect(len(configMap.Data)).To(Equal(3))
		Expect(configMap.Data).To(Equal(map[string]string{
			"SHOOT_INFO_CLUSTER_IDENTITY": clusterIdentity,
			"SHOOT_INFO_NAME":             shootName,
			"SHOOT_INFO_TECHNICAL_ID":     technicalID,
		}))
	})

	It("should overwrite changes in existing configmap", func() {
		customConfigMap := configMap.DeepCopy()

		_, err := controllerutil.CreateOrUpdate(ctx, shootClient, customConfigMap, func() error {
			customConfigMap.Data = map[string]string{
				"FOOBAR": "should be gone after reconcile",
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(ReconcileShootInfoConfigMap(ctx, log, shootClient, config, cluster)).To(Succeed())
		Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
		Expect(len(configMap.Data)).To(Equal(3))
		Expect(configMap.Data).To(Equal(map[string]string{
			"SHOOT_INFO_CLUSTER_IDENTITY": clusterIdentity,
			"SHOOT_INFO_NAME":             shootName,
			"SHOOT_INFO_TECHNICAL_ID":     technicalID,
		}))
		Expect(configMap.Data["FOOBAR"]).To(BeEmpty())
	})
})

func fakeFluxResourceReady(ctx context.Context, c client.Client, obj fluxmeta.ObjectWithConditionsSetter) func() error {
	return func() error {
		cObj := obj.(client.Object)
		if err := c.Get(ctx, client.ObjectKeyFromObject(cObj), cObj); err != nil {
			return err
		}
		obj.SetConditions([]metav1.Condition{{
			Type:   fluxmeta.ReadyCondition,
			Status: metav1.ConditionTrue,
		}})
		return c.Status().Update(ctx, cObj)
	}
}

func fakeFluxReady(ctx context.Context, c client.Client, namespace string) func() error {
	return func() error {
		gitRepoCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "gitrepositories." + sourcev1.GroupVersion.Group},
		}
		if err := c.Get(ctx, client.ObjectKeyFromObject(gitRepoCRD), gitRepoCRD); err != nil {
			return err
		}
		gitRepoCRD.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{
			{
				Type:   apiextensionsv1.NamesAccepted,
				Status: apiextensionsv1.ConditionTrue,
			},
			{
				Type:   apiextensionsv1.Established,
				Status: apiextensionsv1.ConditionTrue,
			},
		}
		if err := c.Status().Update(ctx, gitRepoCRD); err != nil {
			return err
		}

		sourceController := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "source-controller",
				Namespace: namespace,
			},
		}
		if err := c.Get(ctx, client.ObjectKeyFromObject(sourceController), sourceController); err != nil {
			return err
		}
		sourceController.Status.ObservedGeneration = sourceController.Generation
		sourceController.Status.Conditions = []appsv1.DeploymentCondition{
			{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			},
		}
		if err := c.Status().Update(ctx, sourceController); err != nil {
			return err
		}
		return nil
	}
}

// setupManifests copies the local flux manifests to a tmp directory to use for
// tests. This is necessary because flux writes into that directory and we want
// to avoid test pollution.
func setupManifests() string {
	tmpDir, err := manifestgen.MkdirTempAbs("", "gardener-extension-shoot-flux")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		os.RemoveAll(tmpDir)
	})
	srcDir := "./testdata/fluxmanifests"
	files, err := os.ReadDir(srcDir)
	Expect(err).NotTo(HaveOccurred())
	for _, f := range files {
		Expect(copyFile(
			filepath.Join(srcDir, f.Name()),
			filepath.Join(tmpDir, f.Name()),
		)).To(Succeed())
	}
	return tmpDir
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}
