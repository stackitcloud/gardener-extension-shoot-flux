// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package lifecycle contains functions used at the lifecycle controller
package lifecycle

import (
	"context"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/stackitcloud/gardener-extension-shoot-flux/pkg/constants"
)

const (
	// Type is the type of Extension resource.
	Type = constants.ExtensionType
	// Name is the name of the lifecycle controller.
	Name = "shoot_flux_lifecycle_controller"
	// FinalizerSuffix is the finalizer suffix for the shoot flux controller.
	FinalizerSuffix = constants.ExtensionType
)

// DefaultAddOptions contains configuration for the shoot flux controller
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the shoot flux controller to the manager.
type AddOptions struct {
	// ControllerOptions contains options for the controller.
	ControllerOptions controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManager adds a Shoot Flux Lifecycle controller to the given Controller Manager.
//
// PARAMETERS
// mgr  manager.Manager Lifecycle controller manager instance
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return extension.Add(ctx, mgr, extension.AddArgs{
		Actuator:          NewActuator(),
		ControllerOptions: DefaultAddOptions.ControllerOptions,
		Name:              Name,
		FinalizerSuffix:   FinalizerSuffix,
		Resync:            60 * time.Minute,
		Predicates:        extension.DefaultPredicates(ctx, mgr, DefaultAddOptions.IgnoreOperationAnnotation),
		Type:              constants.ExtensionType,
	})
}
