package bearer_token_controller

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type monitoringOff struct {
	commontestutils.ClusterInfoMock
}

func (monitoringOff) IsMonitoringAvailable() bool { return false }

var _ = Describe("Controller setup and reconcile", func() {
	var (
		ci hcoutil.ClusterInfo
		ee hcoutil.EventEmitter
	)
	BeforeEach(func() {
		ci = commontestutils.ClusterInfoMock{}
		ee = commontestutils.NewEventEmitterMock()
	})

	Describe("RegisterReconciler", func() {
		It("registers when monitoring is available", func() {
			cl := commontestutils.InitClient([]client.Object{})
			mgrIntf, err := commontestutils.NewManagerMock(&rest.Config{}, manager.Options{Scheme: commontestutils.GetScheme()}, cl, commontestutils.TestLogger)
			Expect(err).ToNot(HaveOccurred())
			mgr := mgrIntf.(*commontestutils.ManagerMock)

			Expect(RegisterReconciler(mgr, ci, ee)).To(Succeed())
			Expect(mgr.GetRunnables()).To(HaveLen(1))
		})

		It("skips registration when monitoring is not available", func() {
			cl := commontestutils.InitClient([]client.Object{})
			mgrIntf, err := commontestutils.NewManagerMock(&rest.Config{}, manager.Options{Scheme: commontestutils.GetScheme()}, cl, commontestutils.TestLogger)
			Expect(err).ToNot(HaveOccurred())
			mgr := mgrIntf.(*commontestutils.ManagerMock)

			Expect(RegisterReconciler(mgr, monitoringOff{}, ee)).To(Succeed())
			Expect(mgr.GetRunnables()).To(BeEmpty())
		})
	})

	Describe("Reconcile", func() {
		var (
			nsName  = "wb-bearer-token-test-ns"
			ns      *corev1.Namespace
			cl      *commontestutils.HcoTestClient
			mgrIntf manager.Manager
			r       reconcile.Reconciler
			request reconcile.Request
		)

		BeforeEach(func() {
			origNS, hadEnvVar := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, nsName)).To(Succeed())

			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			cl = commontestutils.InitClient([]client.Object{ns})
			var err error
			mgrIntf, err = commontestutils.NewManagerMock(&rest.Config{}, manager.Options{Scheme: commontestutils.GetScheme()}, cl, commontestutils.TestLogger)
			Expect(err).ToNot(HaveOccurred())
			r = newReconciler(mgrIntf, commontestutils.ClusterInfoMock{}, commontestutils.NewEventEmitterMock())
			request = reconcile.Request{NamespacedName: k8stypes.NamespacedName{Name: "irrelevant", Namespace: nsName}}

			DeferCleanup(func() {
				if hadEnvVar {
					_ = os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)
				} else {
					_ = os.Unsetenv(hcoutil.OperatorNamespaceEnv)
				}
			})
		})

		It("creates Service, Secret, and ServiceMonitor and requeues in 5 minutes", func(ctx context.Context) {
			res, err := r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(5 * time.Minute))

			// Service
			svc := &corev1.Service{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: serviceName}, svc)).To(Succeed())
			Expect(svc.Spec.Ports).ToNot(BeEmpty())
			Expect(svc.Labels).ToNot(BeEmpty())

			// Secret
			sec := &corev1.Secret{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: secretName}, sec)).To(Succeed())
			Expect(sec.StringData).To(HaveKey("token"))

			sm := &monitoringv1.ServiceMonitor{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: serviceName}, sm)).To(Succeed())
		})

		It("propagates error from underlying metric reconciler and requeues quickly", func(ctx context.Context) {
			// cause a create-error for Service to bubble up
			cl.InitiateCreateErrors(func(obj client.Object) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == "Service" {
					return context.DeadlineExceeded
				}
				return nil
			})

			r = newReconciler(mgrIntf, ci, ee)
			res, err := r.Reconcile(ctx, request)
			Expect(err).To(MatchError(context.DeadlineExceeded))
			Expect(res.RequeueAfter).To(Equal(100 * time.Millisecond))
		})
	})
})
