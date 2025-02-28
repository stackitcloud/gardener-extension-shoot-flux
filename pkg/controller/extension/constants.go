package extension

import fluxv1alpha1 "github.com/stackitcloud/gardener-extension-shoot-flux/pkg/apis/flux/v1alpha1"

const (
	managedByLabelKey      = "app.kubernetes.io/managed-by"
	managedByLabelValue    = "gardener-extension-" + fluxv1alpha1.ExtensionType
	shootInfoConfigMapName = "shoot-info"
)
