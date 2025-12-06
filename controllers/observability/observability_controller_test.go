package observability

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/observability/rules"
	fakeownreferences "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
)

const testNamespace = "observability_test"

var logger = logf.Log.WithName("observability-controller")

var _ = Describe("Observability Controller", func() {
	var mgr manager.Manager

	BeforeEach(func() {
		err := os.Setenv("OPERATOR_NAMESPACE", testNamespace)
		Expect(err).ToNot(HaveOccurred())

		cl := commontestutils.InitClient([]client.Object{})

		mgr, err = commontestutils.NewManagerMock(&rest.Config{}, manager.Options{}, cl, logger)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should successfully setup the controller", func() {
		err := SetupWithManager(mgr, fakeownreferences.GetFakeDeploymentRef())
		Expect(err).ToNot(HaveOccurred())
		Expect(rules.ListAlerts()).ToNot(BeEmpty())
	})

	It("Should successfully reconcile observability", func() {
		deployRef := fakeownreferences.GetFakeDeploymentRef()
		reconciler := NewReconciler(mgr, testNamespace, deployRef)
		Expect(reconciler.owner.Name).To(Equal(deployRef.Name))
		Expect(reconciler.namespace).To(Equal(testNamespace))
		Expect(reconciler.config).To(Equal(mgr.GetConfig()))
		Expect(reconciler.Client).To(Equal(mgr.GetClient()))
	})

	It("Should receive periodic events in reconciler events channel", func() {
		deployRef := fakeownreferences.GetFakeDeploymentRef()
		reconciler := NewReconciler(mgr, testNamespace, deployRef)
		reconciler.startEventLoop()

		Eventually(reconciler.events).
			WithTimeout(5 * time.Second).
			WithPolling(100 * time.Millisecond).
			Should(Receive())
	})
})
