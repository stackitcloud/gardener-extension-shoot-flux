// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build tools
// +build tools

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	// Gardener provided build tools like crd-ref-docs, goimports, controller-gen,...
	_ "github.com/gardener/gardener/hack/tools"
	_ "k8s.io/code-generator"
)
