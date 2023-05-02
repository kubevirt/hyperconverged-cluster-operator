package libvmi

import (
	"context"
	"fmt"

	"kubevirt.io/kubevirt/tests/framework/kubevirt"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/controller"
)

func GetPodByVirtualMachineInstance(vmi *v1.VirtualMachineInstance, namespace string) (*k8sv1.Pod, error) {
	virtCli := kubevirt.Client()

	pods, err := virtCli.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var controlledPod *k8sv1.Pod
	for podIndex := range pods.Items {
		pod := &pods.Items[podIndex]
		if controller.IsControlledBy(pod, vmi) {
			controlledPod = pod
			break
		}
	}

	if controlledPod == nil {
		return nil, fmt.Errorf("no controlled pod was found for VMI")
	}

	return controlledPod, nil
}

func IndexInterfaceStatusByName(vmi *v1.VirtualMachineInstance) map[string]v1.VirtualMachineInstanceNetworkInterface {
	interfaceStatusByName := map[string]v1.VirtualMachineInstanceNetworkInterface{}
	for _, interfaceStatus := range vmi.Status.Interfaces {
		interfaceStatusByName[interfaceStatus.Name] = interfaceStatus
	}
	return interfaceStatusByName
}
