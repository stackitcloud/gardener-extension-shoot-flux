package extension

import (
	"context"
	"fmt"
	fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type actuator struct {
	client  client.Client
	decoder runtime.Decoder
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(mgr manager.Manager) extension.Actuator {
	return &actuator{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetClient().Scheme()).UniversalDecoder(),
	}
}

// Reconcile the extension resource.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ext *extensionsv1alpha1.Extension) error {
	config, err := a.DecodeProviderConfig(ext.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("errored deconding providerconfig info: %w", err)
	}
	log.Info("Bootstrapping flux", "GitRepo", config.Source.Template.Spec.URL)

	return nil
}

// Delete the extension resource.
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return nil
}

// ForceDelete force deletes the extension resource.
func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return nil
}

// Migrate the extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return nil
}

// Restore the extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return nil
}

func (a *actuator) DecodeProviderConfig(rawExtension *runtime.RawExtension) (*fluxv1alpha1.FluxConfig, error) {
	config := &fluxv1alpha1.FluxConfig{}
	if rawExtension == nil {
		a.client.Scheme().Default(config)
	} else if err := runtime.DecodeInto(a.decoder, rawExtension.Raw, config); err != nil {
		return nil, err
	}
	return config, nil
}
