package observability

import (
	"strings"
	"sync"

	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	datasourceName = "perses-thanos-datasource"
)

var (
	// defaultManagedDashboardNames lists dashboards that HCO actively manages by default.
	defaultManagedDashboardNames = []string{
		"perses-dashboard-node-memory-overview",
	}

	managedDashboards = struct {
		sync.RWMutex
		names map[string]struct{}
	}{names: map[string]struct{}{}}

	dashboardPredicate = predicate.NewTypedPredicateFuncs[*persesv1alpha1.PersesDashboard](func(d *persesv1alpha1.PersesDashboard) bool {
		// Only reconcile dashboards in operator namespace and that are in the managed list
		if d.Namespace != hcoutil.GetOperatorNamespaceFromEnv() {
			return false
		}
		managedDashboards.RLock()
		_, ok := managedDashboards.names[d.Name]
		managedDashboards.RUnlock()
		return ok
	})

	datasourcePredicate = predicate.NewTypedPredicateFuncs[*persesv1alpha1.PersesDatasource](func(ds *persesv1alpha1.PersesDatasource) bool {
		// Only reconcile the single datasource we manage, in operator namespace
		return ds.Namespace == hcoutil.GetOperatorNamespaceFromEnv() && ds.Name == datasourceName
	})
)

func setManagedDashboards(names []string) {
	managed := map[string]struct{}{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		managed[n] = struct{}{}
	}
	managedDashboards.Lock()
	managedDashboards.names = managed
	managedDashboards.Unlock()
}
