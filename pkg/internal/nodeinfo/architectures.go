package nodeinfo

import (
	"slices"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var (
	controlPlaneArchitectures = newArchitectures()
	workloadArchitectures     = newArchitectures()
)

func GetControlPlaneArchitectures() []string {
	return controlPlaneArchitectures.get()
}

func GetWorkloadsArchitectures() []string {
	return workloadArchitectures.get()
}

type Architectures struct {
	architectures []string
	lock          *sync.RWMutex
}

func newArchitectures() *Architectures {
	return &Architectures{
		lock: &sync.RWMutex{},
	}
}

func (a *Architectures) get() []string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return slices.Clone(a.architectures)
}

func (a *Architectures) set(archs sets.Set[string]) bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	archList := archs.UnsortedList()
	slices.Sort(archList)
	if slices.Compare(a.architectures, archList) != 0 {
		a.architectures = archList
		return true
	}

	return false
}

func hasWorkloadRequirements(hc *v1beta1.HyperConverged) bool {
	if hc == nil || hc.Spec.Workloads.NodePlacement == nil {
		return false
	}

	return len(hc.Spec.Workloads.NodePlacement.NodeSelector) > 0 ||
		(hc.Spec.Workloads.NodePlacement.Affinity != nil &&
			hc.Spec.Workloads.NodePlacement.Affinity.NodeAffinity != nil &&
			hc.Spec.Workloads.NodePlacement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil)
}

func getWorkloadMatcher(hc *v1beta1.HyperConverged) nodeaffinity.RequiredNodeAffinity {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeSelector: hc.Spec.Workloads.NodePlacement.NodeSelector,
			Affinity:     hc.Spec.Workloads.NodePlacement.Affinity,
		},
	}

	return nodeaffinity.GetRequiredNodeAffinity(pod)
}
