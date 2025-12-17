package mutator

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	ResourceInvalidNamespace = "an-arbitrary-hcoNamespace"
	HcoValidNamespace        = "kubevirt-hyperconverged"
)

var (
	ErrFakeHcoError = errors.New("fake HyperConverged error")
)

var _ = Describe("webhooks mutator", func() {
	Context("Check mutating webhook for hcoNamespace deletion", func() {
		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		})

		cr := &v1beta1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      util.HyperConvergedName,
				Namespace: HcoValidNamespace,
			},
			Spec: v1beta1.HyperConvergedSpec{},
		}

		var ns runtime.Object = &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: HcoValidNamespace,
			},
		}

		It("should allow the delete of the hcoNamespace if Hyperconverged CR doesn't exist", func() {
			cli := commontestutils.InitClient(nil)
			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Delete, ns, testCodec)}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
		})

		It("should not allow the delete of the hcoNamespace if Hyperconverged CR exists", func() {
			cli := commontestutils.InitClient([]client.Object{cr})
			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Delete, ns, testCodec)}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeFalse())
		})

		It("should not allow when the request is not valid", func() {
			cli := commontestutils.InitClient([]client.Object{cr})
			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Delete}}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeFalse())
		})

		It("should not allow the delete of the hcoNamespace if failed to get Hyperconverged CR", func() {
			cli := commontestutils.InitClient([]client.Object{cr})

			cli.InitiateGetErrors(func(key client.ObjectKey) error {
				if key.Name == util.HyperConvergedName {
					return ErrFakeHcoError
				}
				return nil
			})

			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Delete, ns, testCodec)}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeFalse())
		})

		It("should ignore other namespaces even if Hyperconverged CR exists", func() {
			cli := commontestutils.InitClient([]client.Object{cr})
			otherNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ResourceInvalidNamespace,
				},
			}

			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Delete, otherNs, testCodec)}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
		})

		It("should allow other operations", func() {
			cli := commontestutils.InitClient([]client.Object{cr})
			nsMutator := initMutator(mutatorScheme, cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Update, ns, testCodec)}

			res := nsMutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
		})
	})

})

func initMutator(s *runtime.Scheme, testClient client.Client) *NsMutator {
	decoder := admission.NewDecoder(s)
	nsMutator := NewNsMutator(testClient, decoder, HcoValidNamespace)

	return nsMutator
}

func newRequest(operation admissionv1.Operation, object runtime.Object, encoder runtime.Encoder) admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Operation: operation,
		OldObject: runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, object)),
			Object: object,
		},
	}
}

func newCreateRequest(object runtime.Object, encoder runtime.Encoder) admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, object)),
			Object: object,
		},
	}
}

func newUpdateRequest(origObject runtime.Object, newObject runtime.Object, encoder runtime.Encoder) admissionv1.AdmissionRequest {
	return admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		OldObject: runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, origObject)),
			Object: origObject,
		},
		Object: runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, newObject)),
			Object: newObject,
		},
	}
}
