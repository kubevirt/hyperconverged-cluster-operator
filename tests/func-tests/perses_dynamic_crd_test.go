package tests_test

import (
	"bytes"
	"context"
	_ "embed"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

//go:embed assets/persesdashboards.crd.yaml
var dashboardsCRD []byte

//go:embed assets/persesdatasources.crd.yaml
var datasourcesCRD []byte

var _ = Describe("Perses dynamic CRD gating", Label(tests.OpenshiftLabel, "perses"), func() {
	var cli client.Client
	var crdsPreExisted bool
	var testStart time.Time

	BeforeEach(func(ctx context.Context) {
		testStart = time.Now()
		before := testStart.Add(-10 * time.Minute)
		tests.DumpHCOPodLogs(ctx, "before perses dynamic CRD gating", tests.LogCaptureOptions{Since: &before})
		DeferCleanup(func(ctx context.Context) {
			tests.DumpHCOPodLogs(ctx, "after perses dynamic CRD gating", tests.LogCaptureOptions{Since: &testStart, IncludePrevious: true})
		})

		cli = tests.GetControllerRuntimeClient()
		// Check if both dashboards and datasources CRDs already exist
		crdsPreExisted = areCRDsDeployed(ctx, cli)

		if !crdsPreExisted {
			// Apply required CRDs from embedded YAMLs: dashboards and datasources
			Expect(applyCRDFromBytes(ctx, cli, dashboardsCRD)).To(Succeed())
			Expect(applyCRDFromBytes(ctx, cli, datasourcesCRD)).To(Succeed())
			tests.WaitForHCOOperatorRollout(ctx)
		}

		DeferCleanup(func(ctx context.Context) {
			// Only clean up if we created the CRDs in this test run
			if !crdsPreExisted {
				Expect(deleteCRD(ctx, cli, hcoutil.PersesDashboardsCRDName)).To(Succeed())
				Expect(deleteCRD(ctx, cli, hcoutil.PersesDatasourcesCRDName)).To(Succeed())
			}
		})
	})

	It("creates dashboards and datasources when CRDs are available", func(ctx context.Context) {
		// Eventually, HCO should create at least one PersesDashboard and one PersesDatasource in its namespace
		Eventually(func(g Gomega, gctx context.Context) []persesv1alpha1.PersesDashboard {
			var list persesv1alpha1.PersesDashboardList
			g.Expect(cli.List(gctx, &list, &client.ListOptions{Namespace: tests.InstallNamespace})).To(Succeed())
			return list.Items
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).WithContext(ctx).ShouldNot(BeEmpty())

		Eventually(func(g Gomega, gctx context.Context) []persesv1alpha1.PersesDatasource {
			var list persesv1alpha1.PersesDatasourceList
			g.Expect(cli.List(gctx, &list, &client.ListOptions{Namespace: tests.InstallNamespace})).To(Succeed())
			return list.Items
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).WithContext(ctx).ShouldNot(BeEmpty())

		// And specifically, the expected names should be present
		Eventually(func(gctx context.Context) error {
			return cli.Get(
				gctx,
				client.ObjectKey{Namespace: tests.InstallNamespace, Name: "perses-dashboard-node-memory-overview"},
				&persesv1alpha1.PersesDashboard{},
			)
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

		Eventually(func(gctx context.Context) error {
			return cli.Get(
				gctx,
				client.ObjectKey{Namespace: tests.InstallNamespace, Name: "perses-thanos-datasource"},
				&persesv1alpha1.PersesDatasource{},
			)
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
	})
})

// areCRDsDeployed returns true if both dashboards and datasources CRDs exist.
func areCRDsDeployed(ctx context.Context, cli client.Client) bool {
	if err := cli.Get(ctx, client.ObjectKey{Name: hcoutil.PersesDashboardsCRDName}, &apiextensionsv1.CustomResourceDefinition{}); err != nil {
		return false
	}
	if err := cli.Get(ctx, client.ObjectKey{Name: hcoutil.PersesDatasourcesCRDName}, &apiextensionsv1.CustomResourceDefinition{}); err != nil {
		return false
	}
	return true
}

// applyCRDFromBytes applies the CRD contained in the given YAML bytes.
func applyCRDFromBytes(ctx context.Context, cli client.Client, data []byte) error {
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var crd apiextensionsv1.CustomResourceDefinition
	if err := dec.Decode(&crd); err != nil {
		return err
	}
	if err := cli.Create(ctx, &crd); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// deleteCRD deletes the CRD by name, ignoring NotFound errors.
func deleteCRD(ctx context.Context, cli client.Client, name string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := cli.Delete(ctx, crd); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
