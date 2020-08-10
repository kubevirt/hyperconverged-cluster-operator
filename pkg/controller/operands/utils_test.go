package operands

import (
	"context"
	"github.com/go-logr/logr"
	networkaddons "github.com/kubevirt/cluster-network-addons-operator/pkg/apis"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	sspopv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis"
	vmimportv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// name and namespace of our primary resource
const (
	name      = "kubevirt-hyperconverged"
	namespace = "kubevirt-hyperconverged"
)

var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	log = logf.Log.WithName("controller_hyperconverged")
)

func newHco() *hcov1beta1.HyperConverged {
	return &hcov1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: hcov1beta1.HyperConvergedSpec{},
	}
}

func newReq(inst *hcov1beta1.HyperConverged) *common.HcoRequest {
	return &common.HcoRequest{
		Request:    request,
		Logger:     log,
		Conditions: common.NewHcoConditions(),
		Ctx:        context.TODO(),
		Instance:   inst,
	}
}

func initClient(clientObjects []runtime.Object) client.Client {
	// Create a fake client to mock API calls
	return fake.NewFakeClient(clientObjects...)
}

func getResource() *runtime.Scheme {
	s := scheme.Scheme
	for _, f := range []func(*runtime.Scheme) error{
		apis.AddToScheme,
		cdiv1alpha1.AddToScheme,
		networkaddons.AddToScheme,
		sspopv1.AddToScheme,
		vmimportv1.AddToScheme,
	} {
		Expect(f(s)).To(BeNil())
	}
	return s
}

var eeMock = &eventEmitterMock{}

type eventEmitterMock struct{}

func (eventEmitterMock) Init(_ context.Context, _ manager.Manager, _ hcoutil.ClusterInfo, _ logr.Logger) {
}

func (eventEmitterMock) EmitEvent(_ runtime.Object, _, _, _ string) {
}

func (eventEmitterMock) UpdateClient(_ context.Context, _ client.Reader, _ logr.Logger) {
}
func (eventEmitterMock) EmitCreatedEvent(_ runtime.Object, _, _ string) {}
func (eventEmitterMock) EmitUpdatedEvent(_ runtime.Object, _, _ string) {}
func (eventEmitterMock) EmitDeletedEvent(_ runtime.Object, _, _ string) {}
