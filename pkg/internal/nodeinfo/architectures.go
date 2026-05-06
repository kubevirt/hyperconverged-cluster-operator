package nodeinfo

import (
	"slices"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

const S390X = "s390x"

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

func hasWorkloadRequirements(hc *hcov1.HyperConverged) bool {
	if hc == nil || hc.Spec.Deployment.NodePlacements == nil || hc.Spec.Deployment.NodePlacements.Workload == nil {
		return false
	}

	return len(hc.Spec.Deployment.NodePlacements.Workload.NodeSelector) > 0 ||
		(hc.Spec.Deployment.NodePlacements.Workload.Affinity != nil &&
			hc.Spec.Deployment.NodePlacements.Workload.Affinity.NodeAffinity != nil &&
			hc.Spec.Deployment.NodePlacements.Workload.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil)
}

func getWorkloadMatcher(hc *hcov1.HyperConverged) nodeaffinity.RequiredNodeAffinity {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeSelector: hc.Spec.Deployment.NodePlacements.Workload.NodeSelector,
			Affinity:     hc.Spec.Deployment.NodePlacements.Workload.Affinity,
		},
	}

	return nodeaffinity.GetRequiredNodeAffinity(pod)
}
