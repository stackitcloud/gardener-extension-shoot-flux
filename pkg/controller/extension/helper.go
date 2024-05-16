package extension

import (
	"context"
	"time"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func mergeMeta(from, into client.Object) {
	into.SetLabels(utils.MergeStringMaps(into.GetLabels(), from.GetLabels()))
	into.SetAnnotations(utils.MergeStringMaps(into.GetAnnotations(), from.GetAnnotations()))
}

func setSecretMeta(from, into client.Object) {
	labels := from.GetLabels()
	labels[managedByLabelKey] = managedByLabelValue
	delete(labels, resourcesv1alpha1.ManagedBy)
	into.SetLabels(utils.MergeStringMaps(into.GetLabels(), labels))

	annotations := from.GetAnnotations()
	delete(annotations, resourcesv1alpha1.OriginAnnotation)
	delete(annotations, "resources.gardener.cloud/description")
	into.SetAnnotations(utils.MergeStringMaps(into.GetAnnotations(), annotations))
}

// ConditionFunc checks the health of a polled object. If done==true, waiting should stop and propagate the returned
// error. If done==false, the error is preserved but the check is retried.
type ConditionFunc func() (done bool, err error)

// WaitForObject periodically reads the given object and waits for the given ConditionFunc to return done==true.
// If the check times out, it returns the last error from the ConditionFunc.
func WaitForObject(ctx context.Context, c client.Reader, obj client.Object, interval, timeout time.Duration, check ConditionFunc) error {
	var lastError error
	if err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		lastError = c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(lastError) {
			// wait for the object to appear
			return false, nil
		}
		if lastError != nil {
			// severe error, fail immediately
			return false, lastError
		}

		var done bool
		done, lastError = check()
		if done {
			return true, lastError
		}
		return false, nil
	}); err != nil {
		// if we timed out waiting, return the last error that we observed instead of "context deadline exceeded" or similar
		if lastError != nil {
			return lastError
		}
		return err
	}

	return nil
}
