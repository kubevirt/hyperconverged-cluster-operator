package passt

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	DeployPasstNetworkBindingAnnotation = "deployPasstNetworkBinding"

	BindingName = "passt"

	networkBindingNADName       = "primary-udn-kubevirt-binding"
	networkBindingNADNamespace  = "default"
	NetworkAttachmentDefinition = networkBindingNADNamespace + "/" + networkBindingNADName

	bindingComputeMemoryOverhead = "250Mi"
)

var passtResourceMemory = resource.MustParse(bindingComputeMemoryOverhead)

// CheckPasstImagesEnvExists checks if the passt image environment variable exists
func CheckPasstImagesEnvExists() error {
	if _, passtImageVarExists := os.LookupEnv(hcoutil.PasstImageEnvV); !passtImageVarExists {
		return fmt.Errorf("the %s environment variable must be set", hcoutil.PasstImageEnvV)
	}
	if _, passtCNIImageVarExists := os.LookupEnv(hcoutil.PasstCNIImageEnvV); !passtCNIImageVarExists {
		return fmt.Errorf("the %s environment variable must be set", hcoutil.PasstCNIImageEnvV)
	}
	return nil
}

// NetworkBinding creates an InterfaceBindingPlugin for passt network binding
func NetworkBinding() kubevirtcorev1.InterfaceBindingPlugin {
	return kubevirtcorev1.InterfaceBindingPlugin{
		NetworkAttachmentDefinition: NetworkAttachmentDefinition,
		SidecarImage:                os.Getenv(hcoutil.PasstImageEnvV),
		Migration:                   &kubevirtcorev1.InterfaceBindingMigration{},
		ComputeResourceOverhead: &kubevirtcorev1.ResourceRequirementsWithoutClaims{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: passtResourceMemory,
			},
		},
	}
}

// NewPasstBindingCNISA creates a ServiceAccount for the passt binding CNI
func NewPasstBindingCNISA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "passt-binding-cni",
			Namespace: hc.Namespace,
			Labels:    hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork),
		},
	}
}

// NewPasstServiceAccountHandler creates a conditional handler for passt ServiceAccount
func NewPasstServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return createPasstConditionalHandler(
		operands.NewServiceAccountHandler(Client, Scheme, NewPasstBindingCNISA),
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewPasstBindingCNISA(hc)
		},
	)
}

// createPasstConditionalHandler creates a conditional handler that checks for passt deployment annotation
func createPasstConditionalHandler(handler *operands.GenericOperand, objectCreator func(hc *hcov1beta1.HyperConverged) client.Object) operands.Operand {
	return operands.NewConditionalHandler(
		handler,
		func(hc *hcov1beta1.HyperConverged) bool {
			value, ok := hc.Annotations[DeployPasstNetworkBindingAnnotation]
			return ok && value == "true"
		},
		objectCreator,
	)
}
