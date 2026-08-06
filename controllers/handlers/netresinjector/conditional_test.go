package netresinjector

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("Network Resources Injector Conditional Handlers", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("Conditional ClusterRole Handler", func() {
		It("should create ClusterRole when deployNetworkResourcesInjector is true", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCR := &rbacv1.ClusterRole{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName}, foundCR)).To(Succeed())
			Expect(foundCR.Name).To(Equal(clusterRoleName))
		})

		It("should create ClusterRole when deployNetworkResourcesInjector is not set (default true)", func() {
			// Don't set deployNetworkResourcesInjector - should default to true
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCR := &rbacv1.ClusterRole{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName}, foundCR)).To(Succeed())
		})

		It("should delete ClusterRole when deployNetworkResourcesInjector is false", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			cr := newClusterRole()
			cl = commontestutils.InitClient([]client.Object{hco, cr})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())

			foundCR := &rbacv1.ClusterRole{}
			err := cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName}, foundCR)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
		})
	})

	Context("Conditional ClusterRoleBinding Handler", func() {
		It("should create ClusterRoleBinding when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewClusterRoleBindingHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())
		})

		It("should delete ClusterRoleBinding when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			crb := newClusterRoleBinding()
			cl = commontestutils.InitClient([]client.Object{hco, crb})

			handler := NewClusterRoleBindingHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})

	Context("Conditional ServiceAccount Handler", func() {
		It("should create ServiceAccount when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundSA := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: serviceAccountName, Namespace: hco.Namespace}, foundSA)).To(Succeed())
		})

		It("should delete ServiceAccount when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			sa := newServiceAccount()
			sa.Namespace = hco.Namespace
			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := NewServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})

	Context("Conditional Service Handler", func() {
		It("should create Service when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewServiceHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundSvc := &corev1.Service{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: serviceName, Namespace: hco.Namespace}, foundSvc)).To(Succeed())
		})

		It("should delete Service when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			svc := newService()
			svc.Namespace = hco.Namespace
			cl = commontestutils.InitClient([]client.Object{hco, svc})

			handler := NewServiceHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})

	Context("Conditional Deployment Handler", func() {
		It("should create Deployment when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: deploymentName, Namespace: hco.Namespace}, foundDep)).To(Succeed())
		})

		It("should delete Deployment when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			dep := newDeployment(hco)
			cl = commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})

	Context("Conditional PodDisruptionBudget Handler", func() {
		It("should create PDB when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundPDB := &policyv1.PodDisruptionBudget{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: deploymentName + "-pdb", Namespace: hco.Namespace}, foundPDB)).To(Succeed())
		})

		It("should delete PDB when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			pdb := newPDB()
			pdb.Namespace = hco.Namespace
			cl = commontestutils.InitClient([]client.Object{hco, pdb})

			handler := NewPDBHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})

	Context("Conditional MutatingWebhookConfiguration Handler", func() {
		It("should create MutatingWebhookConfiguration when enabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundMWC := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: webhookConfigName}, foundMWC)).To(Succeed())
		})

		It("should delete MutatingWebhookConfiguration when disabled", func() {
			hco.Spec.Deployment.DeployNetworkResourcesInjector = ptr.To(false)
			mwc := newMutatingWebhookConfiguration()
			cl = commontestutils.InitClient([]client.Object{hco, mwc})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())
		})
	})
})
