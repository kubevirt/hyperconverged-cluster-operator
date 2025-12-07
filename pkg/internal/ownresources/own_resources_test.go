package ownresources

import (
	"context"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	fakeclusterinfo "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Util Suite")
}

const (
	rsName    = "hco-operator"
	podName   = rsName + "-12345"
	namespace = "kubevirt-hyperconverged"
)

var _ = Describe("Test OwnResources", func() {

	var (
		testScheme *runtime.Scheme
	)

	BeforeEach(func() {
		var (
			origThePod        = thePod
			origDeploymentRef = deploymentRef
			origCSV           = csvRef

			origNamespcase     = os.Getenv(hcoutil.OperatorNamespaceEnv)
			origPodName        = os.Getenv(hcoutil.PodNameEnvVar)
			origGetClusterInfo = hcoutil.GetClusterInfo
		)

		//reset initOnce
		initOnce = &sync.Once{}

		thePod = nil
		deploymentRef = nil
		csvRef = nil

		Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, namespace)).To(Succeed())
		Expect(os.Setenv(hcoutil.PodNameEnvVar, podName)).To(Succeed())

		testScheme = scheme.Scheme
		Expect(csvv1alpha1.AddToScheme(testScheme)).To(Succeed())

		DeferCleanup(func() {
			thePod = origThePod
			deploymentRef = origDeploymentRef
			csvRef = origCSV

			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, origNamespcase)).To(Succeed())
			Expect(os.Setenv(hcoutil.PodNameEnvVar, origPodName)).To(Succeed())
			hcoutil.GetClusterInfo = origGetClusterInfo
		})
	})

	It("should update pod and csv if they are found", func(ctx context.Context) {
		hcoutil.GetClusterInfo = fakeclusterinfo.NewGetClusterInfo(
			fakeclusterinfo.WithIsOpenshift(true),
			fakeclusterinfo.WithIsManagedByOLM(true),
			fakeclusterinfo.WithRunningLocally(false),
		)

		csv := cretateCSV()

		csvOwnerRef := &metav1.OwnerReference{
			APIVersion: csvv1alpha1.ClusterServiceVersionAPIVersion,
			Kind:       csvv1alpha1.ClusterServiceVersionKind,
			Name:       rsName,
			Controller: ptr.To(true),
		}

		dep := createDeployment(csvOwnerRef)

		rs := createReplicaSet()

		pod := createPod()

		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(csv, dep, rs, pod).
			WithStatusSubresource(csv, dep, rs, pod).
			Build()

		Init(ctx, cl, testScheme, GinkgoLogr)
		Expect(GetPod()).To(Equal(pod))
		Expect(GetDeploymentRef()).To(Equal(*buildOwnerReference(dep)))
		csvObj := GetCSVRef()
		ref, err := reference.GetReference(testScheme, csvObj)
		Expect(err).NotTo(HaveOccurred())
		Expect(ref).To(HaveValue(Equal(*csvRef)))
	})

	It("should run locally", func(ctx context.Context) {
		hcoutil.GetClusterInfo = fakeclusterinfo.NewGetClusterInfo(
			fakeclusterinfo.WithIsOpenshift(false),
			fakeclusterinfo.WithIsManagedByOLM(false),
			fakeclusterinfo.WithRunningLocally(true),
		)

		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hyperconverged-cluster-operator",
				Namespace: namespace,
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(dep).
			Build()

		Init(ctx, cl, testScheme, GinkgoLogr)
		Expect(GetPod()).To(BeNil())
		Expect(GetDeploymentRef()).To(Equal(*buildOwnerReference(dep)))
		Expect(GetCSVRef()).To(BeNil())
	})
})

func createPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       rsName,
					Controller: ptr.To(true),
				},
			},
		},
	}
}

func createReplicaSet() *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       rsName,
					Controller: ptr.To(true),
				},
			},
		},
	}
}

func createDeployment(ownerReference *metav1.OwnerReference) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsName,
			Namespace: namespace,
		},
	}

	if ownerReference != nil {
		dep.SetOwnerReferences([]metav1.OwnerReference{*ownerReference})
	}

	return dep
}

func cretateCSV() *csvv1alpha1.ClusterServiceVersion {
	return &csvv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsName,
			Namespace: namespace,
		},
	}
}
