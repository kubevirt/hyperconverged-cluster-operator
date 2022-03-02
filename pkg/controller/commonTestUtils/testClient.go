package commonTestUtils

import (
	"context"
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/cmdcommon"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type FakeWriteErrorGenerator func(obj client.Object) error
type FakeReadErrorGenerator func(key client.ObjectKey) error

// implements the client.Client interface (proxy pattern)
type HcoTestClient struct {
	client                 client.Client
	sw                     *HcoTestStatusWriter
	getError               FakeReadErrorGenerator
	createError            FakeWriteErrorGenerator
	updateError            FakeWriteErrorGenerator
	deleteError            FakeWriteErrorGenerator
	cacheSelectorsByObject cache.SelectorsByObject
}

func (c *HcoTestClient) getSelector(obj client.Object) (cache.ObjectSelector, error) {
	for key, selector := range c.cacheSelectorsByObject {
		keyType := reflect.TypeOf(key).String()
		objType := reflect.TypeOf(obj).String()

		if keyType == objType {
			return selector, nil
		}
	}

	return cache.ObjectSelector{}, errors.New("no selector for requested object")
}

func (c *HcoTestClient) filterBySelector(obj client.Object) error {
	selector, selectorErr := c.getSelector(obj)

	if selectorErr != nil || selector.Label == nil {
		return nil
	}

	ls := labels.Set{}
	for k, v := range obj.GetLabels() {
		ls[k] = v
	}

	if selector.Label.Matches(ls) {
		return nil
	}

	return apierrors.NewNotFound(schema.GroupResource{}, obj.GetName())
}

func (c *HcoTestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if c.getError != nil {
		if err := c.getError(key); err != nil {
			return err
		}
	}

	if err := c.client.Get(ctx, key, obj); err != nil {
		return err
	}

	return c.filterBySelector(obj)
}

func (c *HcoTestClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.client.List(ctx, list, opts...)
}

func (c *HcoTestClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.createError != nil {
		if err := c.createError(obj); err != nil {
			return err
		}
	}

	for _, opt := range opts {
		if do, ok := opt.(*client.CreateOptions); ok && len(do.DryRun) == 1 && do.DryRun[0] == metav1.DryRunAll {
			return nil
		}
	}
	return c.client.Create(ctx, obj, opts...)
}

func (c *HcoTestClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.deleteError != nil {
		if err := c.deleteError(obj); err != nil {
			return err
		}
	}

	for _, opt := range opts {
		if do, ok := opt.(*client.DeleteOptions); ok && len(do.DryRun) == 1 && do.DryRun[0] == metav1.DryRunAll {
			return nil
		}
	}

	return c.client.Delete(ctx, obj, opts...)
}

func (c *HcoTestClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.updateError != nil {
		if err := c.updateError(obj); err != nil {
			return err
		}
	}

	for _, opt := range opts {
		if do, ok := opt.(*client.UpdateOptions); ok && len(do.DryRun) == 1 && do.DryRun[0] == metav1.DryRunAll {
			return nil
		}
	}

	return c.client.Update(ctx, obj, opts...)
}

func (c *HcoTestClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *HcoTestClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *HcoTestClient) Status() client.StatusWriter {
	return c.sw
}

func (c *HcoTestClient) InitiateDeleteErrors(f FakeWriteErrorGenerator) {
	c.deleteError = f
}

func (c *HcoTestClient) InitiateUpdateErrors(f FakeWriteErrorGenerator) {
	c.updateError = f
}

func (c *HcoTestClient) InitiateGetErrors(f FakeReadErrorGenerator) {
	c.getError = f
}

func (c *HcoTestClient) InitiateCreateErrors(f FakeWriteErrorGenerator) {
	c.createError = f
}

func (c *HcoTestClient) Scheme() *runtime.Scheme {
	return &runtime.Scheme{}
}

func (c *HcoTestClient) RESTMapper() meta.RESTMapper {
	return nil
}

type HcoTestStatusWriter struct {
	client client.Client
	errors TestErrors
}

func (sw *HcoTestStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if ok, err := sw.errors.GetNextError(); ok {
		return err
	}
	return sw.client.Update(ctx, obj, opts...)
}

func (sw *HcoTestStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if ok, err := sw.errors.GetNextError(); ok {
		return err
	}
	return sw.client.Patch(ctx, obj, patch, opts...)
}

func (sw *HcoTestStatusWriter) InitiateErrors(errs ...error) {
	sw.errors = errs
}

type TestErrors []error

func (errs *TestErrors) GetNextError() (bool, error) {
	if len(*errs) == 0 {
		return false, nil
	}

	err := (*errs)[0]
	*errs = (*errs)[1:]

	return true, err
}

func InitClient(clientObjects []runtime.Object) *HcoTestClient {
	// Create a fake client to mock API calls
	cl := fake.NewClientBuilder().
		WithRuntimeObjects(clientObjects...).
		WithScheme(GetScheme()).
		Build()

	return &HcoTestClient{client: cl, sw: &HcoTestStatusWriter{client: cl}, cacheSelectorsByObject: cmdcommon.GetCacheSelectorsByObject("")}
}
