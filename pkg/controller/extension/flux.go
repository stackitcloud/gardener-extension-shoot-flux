package extension

import (
	"context"
	"fmt"
	"time"

	fluxinstall "github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
)

// InstallFlux applies the Flux install manifest based on the given configuration. It also performs a basic health check
// before returning.
func InstallFlux(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	config *fluxv1alpha1.FluxInstallation,
	manifestsBase string,
	interval time.Duration,
	timeout time.Duration,
) error {
	log = log.WithValues("version", config.Version)
	log.Info("Installing Flux")

	installManifest, err := GenerateInstallManifest(config, manifestsBase)
	if err != nil {
		return fmt.Errorf("error generating install manifest: %w", err)
	}

	if err := kubernetes.NewApplier(c, c.RESTMapper()).ApplyManifest(ctx, kubernetes.NewManifestReader(installManifest), kubernetes.DefaultMergeFuncs); err != nil {
		return fmt.Errorf("error applying Flux install manifest: %w", err)
	}

	log.Info("Waiting for Flux installation to get ready")
	// Wait for GitRepository CRD to become healthy as a basic indicator of whether the installation is ready to be
	// bootstrapped.
	// We don't intend to health check the entire Flux installation, but we want to avoid bootstrap failures that could
	// have been avoided by a short wait.
	gitRepositoryCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "gitrepositories." + sourcev1.GroupVersion.Group},
	}
	if err := WaitForObject(ctx, c, gitRepositoryCRD, interval, timeout, func() (done bool, err error) {
		err = health.CheckCustomResourceDefinition(gitRepositoryCRD)
		return err == nil, err
	}); err != nil {
		return fmt.Errorf("error waiting for Flux installation to get ready: %w", err)
	}

	// Wait for one of the deployments to be available to ensure the selected registry actually hosts flux container
	// images.
	sourceController := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-controller",
			Namespace: *config.Namespace,
		},
	}
	if err := WaitForObject(ctx, c, sourceController, interval, timeout, func() (done bool, err error) {
		err = health.CheckDeployment(sourceController)
		return err == nil, err
	}); err != nil {
		return fmt.Errorf("error waiting for Flux installation to get ready: %w", err)
	}

	log.Info("Successfully installed Flux")

	return nil
}

// GenerateInstallManifest generates the Flux install manifest based on the given configuration just like
// "flux install --export". manifestsBase can be set for tests.
func GenerateInstallManifest(config *fluxv1alpha1.FluxInstallation, manifestsBase string) ([]byte, error) {
	options := fluxinstall.MakeDefaultOptions()
	options.Version = *config.Version
	options.Namespace = *config.Namespace
	options.Registry = *config.Registry

	// don't deploy optional components
	options.ComponentsExtra = nil

	manifest, err := fluxinstall.Generate(options, manifestsBase)
	if err != nil {
		return nil, err
	}

	return []byte(manifest.Content), nil
}

// ReconcileSource and wait fo it to get ready.
func ReconcileSource(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	config *fluxv1alpha1.Source,
	interval time.Duration,
	timeout time.Duration,
) error {
	log.Info("Bootstrapping Flux GitRepository")

	// Create GitRepository
	gitRepository := config.Template.DeepCopy()
	// todo: ensure the namespace is correct
	if _, err := controllerutil.CreateOrUpdate(ctx, shootClient, gitRepository, func() error {
		mergeMeta(&config.Template, gitRepository)
		config.Template.Spec.DeepCopyInto(&gitRepository.Spec)
		return nil
	}); err != nil {
		return fmt.Errorf("error applying GitRepository template: %w", err)
	}

	log.Info("Waiting for GitRepository to get ready")
	if err := WaitForObject(ctx, shootClient, gitRepository, interval, timeout, CheckFluxObject(gitRepository)); err != nil {
		return fmt.Errorf("error waiting for GitRepository to get ready: %w", err)
	}

	log.Info("Successfully bootstrapped Flux GitRepository")

	return nil
}

// ReconcileKustomization and wait fo it to get ready.
func ReconcileKustomization(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	config *fluxv1alpha1.Kustomization,
	interval time.Duration,
	timeout time.Duration,
) error {
	log.Info("Bootstrapping Flux Kustomization")

	kustomization := config.Template.DeepCopy()
	// TODO: could we do apply / patch here so that we only set the fields that were specified in the providerConfig?
	if _, err := controllerutil.CreateOrUpdate(ctx, c, kustomization, func() error {
		mergeMeta(&config.Template, kustomization)
		config.Template.Spec.DeepCopyInto(&kustomization.Spec)
		return nil
	}); err != nil {
		return fmt.Errorf("error applying Kustomization template: %w", err)
	}

	log.Info("Waiting for Kustomization to get ready")
	if err := WaitForObject(ctx, c, kustomization, interval, timeout, CheckFluxObject(kustomization)); err != nil {
		return fmt.Errorf("error waiting for Kustomization to get ready: %w", err)
	}

	log.Info("Successfully bootstrapped Flux Kustomization")

	return nil
}

// CheckFluxObject returns a ConditionFunc that determines the health of Flux objects based on the Ready condition.
func CheckFluxObject(obj fluxmeta.ObjectWithConditions) ConditionFunc {
	return func() (healthy bool, err error) {
		if cond := meta.FindStatusCondition(obj.GetConditions(), fluxmeta.ReadyCondition); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				return true, nil
			case metav1.ConditionFalse:
				return true, fmt.Errorf("reconciliation failed: %s", cond.Message)
			}
		}

		return false, fmt.Errorf("has not been reconciled yet")
	}
}
