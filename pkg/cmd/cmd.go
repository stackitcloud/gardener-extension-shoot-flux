// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package cmd provides Kubernetes controller configuration structures used for command execution
package cmd

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/stackitcloud/gardener-extension-shoot-flux/pkg/controller/extension"
)

// NewServiceControllerCommand creates a new command that is used to start the shoot flux controller
func NewServiceControllerCommand() *cobra.Command {
	options := NewOptions()

	cmd := &cobra.Command{
		Use:           "gardener-extension-shoot-flux",
		Short:         "Flux controller manages installations of fluxcd in shoot clusters",
		SilenceErrors: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := options.optionAggregator.Complete(); err != nil {
				return fmt.Errorf("error completing options: %s", err)
			}
			if err := options.heartbeatOptions.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			return options.run(cmd.Context())
		},
	}

	verflag.AddFlags(cmd.Flags())
	options.optionAggregator.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) run(ctx context.Context) error {
	// TODO: Make these flags configurable via command line parameters or component config file.
	util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfig.ClientConnectionConfiguration{
		QPS:   100.0,
		Burst: 130,
	}, o.restOptions.Completed().Config)

	mgrOpts := o.managerOptions.Completed().Options()
	mgrOpts.WebhookServer = nil

	mgrOpts.Client.Cache = &client.CacheOptions{
		DisableFor: []client.Object{
			&corev1.Secret{}, // applied for ManagedResources
		},
	}

	mgr, err := manager.New(o.restOptions.Completed().Config, mgrOpts)
	if err != nil {
		return fmt.Errorf("could not instantiate controller-manager: %s", err)
	}

	if err := extensionscontroller.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("could not update manager scheme: %s", err)
	}

	o.controllerOptions.Completed().Apply(&extension.DefaultAddOptions.Controller)
	o.extensionOptions.Completed().Apply(&extension.DefaultAddOptions.Controller)
	o.heartbeatOptions.Completed().Apply(&heartbeat.DefaultAddOptions)

	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return fmt.Errorf("could not add readycheck for informers: %w", err)
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("could not add healthcheck: %w", err)
	}

	if err := o.controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("could not add controllers to manager: %s", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("error running manager: %s", err)
	}

	return nil
}
