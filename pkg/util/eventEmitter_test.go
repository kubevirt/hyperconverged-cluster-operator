package util

import (
	"context"
	"k8s.io/client-go/kubernetes/scheme"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	namespace = "kubevirt-hyperconverged"
	podName   = "hco-operator"
)

var (
	rec = record.NewFakeRecorder(1)
	// Can't use HyperConverged because it causes cyclic import
	object = &corev1.Pod{}

	csv = &csvv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	dep = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(csv, schema.GroupVersionKind{
				Kind: "ClusterServiceVersion",
			})},
		},
	}

	rs = &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(dep, schema.GroupVersionKind{
				Kind: "Deployment",
			})},
		},
	}

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rs, schema.GroupVersionKind{
				Kind: "ReplicaSet",
			})},
		},
	}
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("Test Event Emitter")
)

var _ = Describe("Test Event Emitter", func() {
	origRunInOpenShift := clusterInfo.IsOpenshift()
	BeforeSuite(func() {
		ci := clusterInfo.(*ClusterInfoImp)
		ci.runningInOpenshift = true
	})

	AfterSuite(func() {
		ci := clusterInfo.(*ClusterInfoImp)
		ci.runningInOpenshift = origRunInOpenShift
	})

	Context("Test EmitEvent", func() {
		It("should do nothing if the pod, csv and objects are all null", func() {
			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
			}

			ee.EmitEvent(nil, "type", "reason", "message")
			Expect(rec.Events).To(BeEmpty())
		})

		It("should send object event if exists", func() {
			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
			}

			var event string
			go ee.EmitEvent(object, "Testing", "HCO", "Events")

			Eventually(rec.Events).Should(Receive(&event))

			Expect(event).Should(Equal("Testing HCO Events"))

			Expect(rec.Events).To(BeEmpty())
		})

		It("should send pod event if pod exists", func() {
			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
				pod:         pod,
			}

			var (
				podEvent    string
				objectEvent string
			)
			go ee.EmitEvent(object, "Testing", "HCO", "Events")
			Eventually(rec.Events).Should(Receive(&podEvent))
			Eventually(rec.Events).Should(Receive(&objectEvent))

			Expect(podEvent).Should(Equal("Testing HCO Events"))
			Expect(objectEvent).Should(Equal("Testing HCO Events"))

			Expect(rec.Events).To(BeEmpty())
		})

		It("should send csv event if csv exists", func() {
			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
				pod:         pod,
				csv:         &csvv1alpha1.ClusterServiceVersion{},
			}

			var (
				podEvent    string
				objectEvent string
				csvEvent    string
			)
			go ee.EmitEvent(object, "Testing", "HCO", "Events")
			Eventually(rec.Events).Should(Receive(&podEvent))
			Eventually(rec.Events).Should(Receive(&objectEvent))
			Eventually(rec.Events).Should(Receive(&csvEvent))

			Expect(podEvent).Should(Equal("Testing HCO Events"))
			Expect(objectEvent).Should(Equal("Testing HCO Events"))
			Expect(csvEvent).Should(Equal("Testing HCO Events"))

			Expect(rec.Events).To(BeEmpty())
		})
	})

	Context("Test UpdateClient", func() {
		testScheme := scheme.Scheme
		csvv1alpha1.AddToScheme(testScheme)

		origPodName := os.Getenv(PodNameEnvVar)
		AfterEach(func() {
			GetOperatorNamespace = getOperatorNamespace
			os.Setenv(PodNameEnvVar, origPodName)
		})

		It("Should get the pod and the CSV", func() {
			os.Setenv(PodNameEnvVar, podName)
			GetOperatorNamespace = func(_ logr.Logger) (string, error) {
				return namespace, nil
			}
			cl := fake.NewClientBuilder().
				WithRuntimeObjects(pod, rs, dep, csv).
				WithScheme(testScheme).
				Build()

			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
			}

			ee.UpdateClient(context.TODO(), cl, logger)

			Expect(ee.pod).ToNot(BeNil())
			Expect(ee.pod.Namespace).Should(Equal(namespace))
			Expect(ee.pod.Name).Should(Equal(podName))

			Expect(ee.csv).ToNot(BeNil())
			Expect(ee.csv.Namespace).Should(Equal(namespace))
			Expect(ee.csv.Name).Should(Equal(podName))
		})

		It("Should get the pod work even without a CSV", func() {
			os.Setenv(PodNameEnvVar, podName)
			GetOperatorNamespace = func(_ logr.Logger) (string, error) {
				return namespace, nil
			}
			cl := fake.NewClientBuilder().
				WithRuntimeObjects(pod, rs, dep).
				WithScheme(testScheme).
				Build()

			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
			}

			ee.UpdateClient(context.TODO(), cl, logger)

			Expect(ee.pod).ToNot(BeNil())
			Expect(ee.pod.Namespace).Should(Equal(namespace))
			Expect(ee.pod.Name).Should(Equal(podName))

			Expect(ee.csv).To(BeNil())
		})

		It("Should get the pod and the CSV", func() {
			os.Setenv(PodNameEnvVar, podName)
			GetOperatorNamespace = func(_ logr.Logger) (string, error) {
				return namespace, nil
			}
			cl := fake.NewClientBuilder().
				WithScheme(testScheme).
				Build()

			ee := eventEmitter{
				recorder:    rec,
				clusterInfo: clusterInfo,
			}

			ee.UpdateClient(context.TODO(), cl, logger)

			Expect(ee.pod).To(BeNil())
			Expect(ee.csv).To(BeNil())
		})
	})
})
