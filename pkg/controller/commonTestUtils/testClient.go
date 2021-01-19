package commonTestUtils

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type HcoTestClient struct {
	client      client.Client
	sw          *HcoTestStatusWriter
	readErrors  TestErrors
	writeErrors TestErrors
}

func (c *HcoTestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if ok, err := c.readErrors.GetNextError(); ok {
		return err
	}
	return c.client.Get(ctx, key, obj)
}

func (c *HcoTestClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.List(ctx, list, opts...)
}

func (c *HcoTestClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.Create(ctx, obj, opts...)
}

func (c *HcoTestClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.Delete(ctx, obj, opts...)
}

func (c *HcoTestClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.Update(ctx, obj, opts...)
}

func (c *HcoTestClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *HcoTestClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if ok, err := c.writeErrors.GetNextError(); ok {
		return err
	}
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *HcoTestClient) Status() client.StatusWriter {
	return c.sw
}

func (c *HcoTestClient) InitiateReadErrors(errs ...error) {
	c.readErrors = errs
}

func (c *HcoTestClient) InitiateWriteErrors(errs ...error) {
	c.writeErrors = errs
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
	cl := fake.NewFakeClient(clientObjects...)
	return &HcoTestClient{client: cl, sw: &HcoTestStatusWriter{client: cl}}
}
