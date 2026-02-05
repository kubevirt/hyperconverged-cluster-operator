package apiserver

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
)

var _ = Describe("HyperconvergedController", func() {

	Describe("Controller setup", func() {

		Context("Setup", func() {

			It("Should setup the controller if on Openshift", func() {
				resources := []client.Object{}
				cl := commontestutils.InitClient(resources)

				mgr, err := commontestutils.NewManagerMock(&rest.Config{}, manager.Options{}, cl, logger)
				Expect(err).ToNot(HaveOccurred())
				mockmgr, ok := mgr.(*commontestutils.ManagerMock)
				Expect(ok).To(BeTrue())

				// we should have no runnable before registering the controller
				Expect(mockmgr.GetRunnables()).To(BeEmpty())

				// we should have one runnable after registering it on Openshift
				Expect(RegisterReconciler(mgr, nil)).To(Succeed())
				Expect(mockmgr.GetRunnables()).To(HaveLen(1))
			})
		})

	})

	Describe("Reconcile APIServer CR", func() {

		Context("APIServer CR", func() {

			It("Should refresh cached APIServer if the reconciliation is caused by a change there", func(ctx context.Context) {
				initialTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:         openshiftconfigv1.TLSProfileIntermediateType,
					Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
				}
				customTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileModernType,
					Modern: &openshiftconfigv1.ModernTLSProfile{},
				}

				apiServer := &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{
						TLSSecurityProfile: initialTLSSecurityProfile,
					},
				}

				cl := commontestutils.InitClient([]client.Object{apiServer})

				_, err := tlssecprofile.Refresh(ctx, cl)
				Expect(err).ToNot(HaveOccurred())

				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(initialTLSSecurityProfile), "should return the initial value)")

				notifier := make(chan event.GenericEvent, 1)
				DeferCleanup(func() {
					close(notifier)
				})

				r := ReconcileAPIServer{
					client:   cl,
					notifier: notifier,
				}

				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "cluster",
					},
				}

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(notifier).ToNot(Receive())

				// Update ApiServer CR
				apiServer.Spec.TLSSecurityProfile = customTLSSecurityProfile
				Expect(cl.Update(ctx, apiServer)).To(Succeed())
				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(initialTLSSecurityProfile), "should still return the cached value (initial value)")

				// Reconcile again to refresh ApiServer CR in memory
				res, err = r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(notifier).To(Receive())

				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(customTLSSecurityProfile), "should return the up-to-date value")
			})

			It("Should send notification if the TLS Security Profile was changed", func(ctx context.Context) {

				initialTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:         openshiftconfigv1.TLSProfileIntermediateType,
					Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
				}
				customTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileModernType,
					Modern: &openshiftconfigv1.ModernTLSProfile{},
				}

				apiServer := &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{
						TLSSecurityProfile: initialTLSSecurityProfile,
					},
				}

				cl := commontestutils.InitClient([]client.Object{apiServer})

				_, err := tlssecprofile.Refresh(ctx, cl)
				Expect(err).ToNot(HaveOccurred())

				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(initialTLSSecurityProfile), "should return the initial value)")

				notifier := make(chan event.GenericEvent, 1)
				DeferCleanup(func() {
					close(notifier)
				})

				r := ReconcileAPIServer{
					client:   cl,
					notifier: notifier,
				}

				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "cluster",
					},
				}

				By("should not notify if nothing has changed")
				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(notifier).ToNot(Receive())

				// Update ApiServer CR
				apiServer.Spec.TLSSecurityProfile = customTLSSecurityProfile
				Expect(cl.Update(ctx, apiServer)).To(Succeed())
				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(initialTLSSecurityProfile), "should still return the cached value (initial value)")

				By("Should notify when the TLS profile has changed")
				// Reconcile again to refresh ApiServer CR in memory
				res, err = r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(notifier).To(Receive())

				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(customTLSSecurityProfile), "should return the up-to-date value")

				By("should not notify if nothing has changed")
				res, err = r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(notifier).ToNot(Receive())

				Expect(tlssecprofile.GetTLSSecurityProfile(nil)).To(Equal(customTLSSecurityProfile), "should return the up-to-date value")
			})
		})

	})

})
