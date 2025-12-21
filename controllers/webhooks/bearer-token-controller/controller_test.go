package bearer_token_controller

import (
	"context"
	"maps"
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
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
		fakeownresources.OLMV0OwnResourcesMock()

		DeferCleanup(func() {
			fakeownresources.ResetOwnResources()
		})
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
		const nsName = "wb-bearer-token-test-ns"

		var (
			secret    *corev1.Secret
			resources []client.Object
			cl        *commontestutils.HcoTestClient
			mgrIntf   manager.Manager
			r         reconcile.Reconciler
			request   reconcile.Request
		)

		JustBeforeEach(func() {
			origNS, hadEnvVar := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, nsName)).To(Succeed())

			resources = []client.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}},
			}
			if secret != nil {
				resources = append(resources, secret)
			}
			cl = commontestutils.InitClient(resources)

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
			ctx = logr.NewContext(ctx, GinkgoLogr)
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
			ctx = logr.NewContext(ctx, GinkgoLogr)
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

		It("should recreate Secret, and delete the ServiceMonitor, if token is changed", func(ctx context.Context) {
			ctx = logr.NewContext(ctx, GinkgoLogr)
			res, err := r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(5 * time.Minute))

			// Secret
			sec := &corev1.Secret{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: secretName}, sec)).To(Succeed())
			Expect(sec.StringData).To(HaveKey("token"))

			origToken := sec.StringData["token"]
			sec.StringData["token"] = "some-wrong-token"
			Expect(cl.Update(ctx, sec)).To(Succeed())

			newSec := &corev1.Secret{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: secretName}, newSec)).To(Succeed())
			Expect(newSec.StringData).To(HaveKey("token"))
			Expect(newSec.StringData["token"]).To(Equal("some-wrong-token"))

			// Reconcile should delete the old secret and create a new one
			res, err = r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(5 * time.Minute))

			newSec = &corev1.Secret{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: secretName}, newSec)).To(Succeed())
			Expect(newSec.StringData).To(HaveKey("token"))
			Expect(newSec.StringData["token"]).To(Equal(origToken))

			newSM := &monitoringv1.ServiceMonitor{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: serviceName}, newSM)).To(MatchError(apierrors.IsNotFound, "not found error"))

			res, err = r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter).To(Equal(5 * time.Minute))
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: serviceName}, newSM)).To(Succeed())
		})

		Context("secret labels", func() {
			When("token was not changed", func() {
				BeforeEach(func() {
					token, err := authorization.CreateToken()
					Expect(err).ToNot(HaveOccurred())

					secret = newSecret(nsName, ownresources.GetDeploymentRef(), token)
					secret.Data = map[string][]byte{"token": []byte(token)}
				})

				When("the secret labels were modified", func() {
					BeforeEach(func() {
						secret.Labels[hcoutil.AppLabelManagedBy] = "something else"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
					})
				})

				When("the secret label was removed", func() {
					BeforeEach(func() {
						delete(secret.Labels, hcoutil.AppLabelManagedBy)
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
					})
				})

				When("the secret has no labels", func() {
					var origLabels map[string]string
					BeforeEach(func() {
						origLabels = maps.Clone(secret.Labels)
						secret.Labels = nil
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.Labels).To(Equal(origLabels))
					})
				})

				When("the secret has custom labels", func() {
					var origLabels map[string]string
					BeforeEach(func() {
						origLabels = maps.Clone(secret.Labels)
						secret.Labels["custom-label1"] = "custom-label1"
						secret.Labels["custom-label2"] = "custom-label2"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						for k, v := range origLabels {
							Expect(found.Labels).To(HaveKeyWithValue(k, v))
						}
						Expect(found.Labels).To(HaveKeyWithValue("custom-label1", "custom-label1"))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label2", "custom-label2"))
					})
				})

				When("the secret has both modified and custom labels", func() {
					BeforeEach(func() {
						secret.Labels[hcoutil.AppLabelManagedBy] = "something else"
						secret.Labels["custom-label1"] = "custom-label1"
						secret.Labels["custom-label2"] = "custom-label2"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label1", "custom-label1"))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label2", "custom-label2"))
					})
				})
			})

			When("token was changed", func() {
				var token string

				BeforeEach(func() {
					var err error
					token, err = authorization.CreateToken()
					Expect(err).ToNot(HaveOccurred())

					secret = newSecret(nsName, ownresources.GetDeploymentRef(), "something-else")
					secret.Data = map[string][]byte{"token": []byte("something-else")}
				})

				When("the secret labels were modified", func() {
					BeforeEach(func() {
						secret.Labels[hcoutil.AppLabelManagedBy] = "something else"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.StringData).To(HaveKeyWithValue("token", token))
						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
					})
				})

				When("a secret label was removed", func() {
					BeforeEach(func() {
						delete(secret.Labels, hcoutil.AppLabelManagedBy)
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.StringData).To(HaveKeyWithValue("token", token))
						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
					})
				})

				When("the secret has no labels", func() {
					var origLabels map[string]string
					BeforeEach(func() {
						origLabels = maps.Clone(secret.Labels)
						secret.Labels = nil
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.StringData).To(HaveKeyWithValue("token", token))
						Expect(found.Labels).To(Equal(origLabels))
					})
				})

				When("the secret has custom labels", func() {
					var origLabels map[string]string
					BeforeEach(func() {
						origLabels = maps.Clone(secret.Labels)
						secret.Labels["custom-label1"] = "custom-label1"
						secret.Labels["custom-label2"] = "custom-label2"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.StringData).To(HaveKeyWithValue("token", token))
						for k, v := range origLabels {
							Expect(found.Labels).To(HaveKeyWithValue(k, v))
						}
						Expect(found.Labels).To(HaveKeyWithValue("custom-label1", "custom-label1"))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label2", "custom-label2"))
					})
				})

				When("the secret has both modified and custom labels", func() {
					BeforeEach(func() {
						secret.Labels[hcoutil.AppLabelManagedBy] = "something else"
						secret.Labels["custom-label1"] = "custom-label1"
						secret.Labels["custom-label2"] = "custom-label2"
					})

					It("should reconcile secret's labels", func(ctx context.Context) {
						ctx = logr.NewContext(ctx, GinkgoLogr)
						res, err := r.Reconcile(ctx, request)

						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(Equal(requeueDurationForNextRequest))

						found := &corev1.Secret{}
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(secret), found)).To(Succeed())
						Expect(found.StringData).To(HaveKeyWithValue("token", token))

						Expect(found.Labels).To(HaveKeyWithValue(hcoutil.AppLabelManagedBy, hcoutil.OperatorName))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label1", "custom-label1"))
						Expect(found.Labels).To(HaveKeyWithValue("custom-label2", "custom-label2"))
					})
				})
			})
		})
	})
})
