package passt

import (
	"fmt"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

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

var (
	passtResourceMemory = resource.MustParse(bindingComputeMemoryOverhead)
	passtImage          string
	passtImageOnce      sync.Once
)

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
func NetworkBinding(sidecarImage string) kubevirtcorev1.InterfaceBindingPlugin {
	return kubevirtcorev1.InterfaceBindingPlugin{
		NetworkAttachmentDefinition: NetworkAttachmentDefinition,
		SidecarImage:                sidecarImage,
		Migration:                   &kubevirtcorev1.InterfaceBindingMigration{},
		ComputeResourceOverhead: &kubevirtcorev1.ResourceRequirementsWithoutClaims{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: passtResourceMemory,
			},
		},
	}
}

func GetImage() string {
	passtImageOnce.Do(func() {
		passtImage = os.Getenv(hcoutil.PasstImageEnvV)
	})
	return passtImage
}
