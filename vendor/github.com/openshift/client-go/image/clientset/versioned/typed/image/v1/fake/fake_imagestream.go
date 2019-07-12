package fake

import (
	image_v1 "github.com/openshift/api/image/v1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreams implements ImageStreamInterface
type FakeImageStreams struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreamsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"}

var imagestreamsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageStream"}

// Get takes name of the imageStream, and returns the corresponding imageStream object, and an error if there is any.
func (c *FakeImageStreams) Get(name string, options v1.GetOptions) (result *image_v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(imagestreamsResource, c.ns, name), &image_v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStream), err
}

// List takes label and field selectors, and returns the list of ImageStreams that match those selectors.
func (c *FakeImageStreams) List(opts v1.ListOptions) (result *image_v1.ImageStreamList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(imagestreamsResource, imagestreamsKind, c.ns, opts), &image_v1.ImageStreamList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &image_v1.ImageStreamList{}
	for _, item := range obj.(*image_v1.ImageStreamList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *FakeImageStreams) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(imagestreamsResource, c.ns, opts))

}

// Create takes the representation of a imageStream and creates it.  Returns the server's representation of the imageStream, and an error, if there is any.
func (c *FakeImageStreams) Create(imageStream *image_v1.ImageStream) (result *image_v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamsResource, c.ns, imageStream), &image_v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStream), err
}

// Update takes the representation of a imageStream and updates it. Returns the server's representation of the imageStream, and an error, if there is any.
func (c *FakeImageStreams) Update(imageStream *image_v1.ImageStream) (result *image_v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(imagestreamsResource, c.ns, imageStream), &image_v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStream), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeImageStreams) UpdateStatus(imageStream *image_v1.ImageStream) (*image_v1.ImageStream, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(imagestreamsResource, "status", c.ns, imageStream), &image_v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStream), err
}

// Delete takes name of the imageStream and deletes it. Returns an error if one occurs.
func (c *FakeImageStreams) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagestreamsResource, c.ns, name), &image_v1.ImageStream{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeImageStreams) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(imagestreamsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &image_v1.ImageStreamList{})
	return err
}

// Patch applies the patch and returns the patched imageStream.
func (c *FakeImageStreams) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *image_v1.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(imagestreamsResource, c.ns, name, data, subresources...), &image_v1.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStream), err
}

// Secrets takes label and field selectors, and returns the list of Secrets that match those selectors.
func (c *FakeImageStreams) Secrets(imageStreamName string, opts v1.ListOptions) (result *core_v1.SecretList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListSubresourceAction(imagestreamsResource, imageStreamName, "secrets", imagestreamsKind, c.ns, opts), &core_v1.SecretList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*core_v1.SecretList), err
}
