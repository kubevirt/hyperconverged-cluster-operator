package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("ServiceMonitor handler", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	newServiceMonitor := func() *monitoringv1.ServiceMonitor {
		return &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: commontestutils.Namespace,
				Labels:    GetLabels(hcoutil.AppComponentDeployment),
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector:  metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Endpoints: []monitoringv1.Endpoint{{Port: "http-metrics"}},
			},
		}
	}

	It("should create the ServiceMonitor", func() {
		cl := commontestutils.InitClient([]client.Object{hco})
		res := NewServiceMonitorHandler(cl, commontestutils.GetScheme(), newServiceMonitor()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Created).To(BeTrue())
	})

	It("should not update an unchanged ServiceMonitor", func() {
		cl := commontestutils.InitClient([]client.Object{hco, newServiceMonitor()})
		res := NewServiceMonitorHandler(cl, commontestutils.GetScheme(), newServiceMonitor()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeFalse())
	})

	It("should restore a changed spec", func() {
		existing := newServiceMonitor()
		existing.Spec.Endpoints[0].Port = "other"
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewServiceMonitorHandler(cl, commontestutils.GetScheme(), newServiceMonitor()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())

		found := &monitoringv1.ServiceMonitor{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
		Expect(found.Spec.Endpoints[0].Port).To(Equal("http-metrics"))
	})
})
