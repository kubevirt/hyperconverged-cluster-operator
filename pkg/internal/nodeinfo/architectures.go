package nodeinfo

import (
	"maps"
	"slices"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

const S390X = "s390x"

var (
	architectures = newArchitectures()
)

func GetControlPlaneArchitectures() []string {
	return architectures.getCPArches()
}

func GetWorkloadsArchitectures() []string {
	return architectures.getWorkloadArches()
}

// GetDefaultArchitecture returns the default architecture for virtual machines
// Assuming a single control plane architecture, HCO will choose this architecture as default, if the workload
// architecture contains it. If not, HCO will choose the architecture used by most of the workload nodes.
func GetDefaultArchitecture() string {
	architectures.lock.RLock()
	defer architectures.lock.RUnlock()

	return architectures.defaultArch
}

type Architectures struct {
	workloadArches []string
	cpArches       []string
	workloadCount  map[string]int
	defaultArch    string

	lock *sync.RWMutex
}

func newArchitectures() *Architectures {
	return &Architectures{
		lock: &sync.RWMutex{},
	}
}

func (a *Architectures) getWorkloadArches() []string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return slices.Clone(a.workloadArches)
}

func (a *Architectures) getCPArches() []string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return slices.Clone(a.cpArches)
}

func (a *Architectures) set(wlArches map[string]int, cpArches sets.Set[string]) bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	modified := false
	if !maps.Equal(a.workloadCount, wlArches) {
		a.workloadArches = getSortedMapKeys(wlArches)
		a.workloadCount = maps.Clone(wlArches)
		modified = true
	}

	cpArchesList := cpArches.UnsortedList()
	slices.Sort(cpArchesList)

	if !slices.Equal(a.cpArches, cpArchesList) {
		a.cpArches = cpArchesList
		modified = true
	}

	if modified {
		if len(cpArchesList) > 0 {
			if cpArch := cpArchesList[0]; slices.Contains(a.workloadArches, cpArch) {
				a.defaultArch = cpArch
				return true
			}
		}

		maxCount := 0
		maxArch := ""
		for _, wrkArch := range a.workloadArches {
			count := a.workloadCount[wrkArch]
			if maxCount < count {
				maxCount = count
				maxArch = wrkArch
			}
		}

		a.defaultArch = maxArch
	}

	return modified
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

func getSortedMapKeys(m map[string]int) []string {
	return slices.Sorted(maps.Keys(m))
}
