package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var _ = Describe("", func() {
	Context("test UpdateClient", func() {
		const (
			rsName    = "hco-operator"
			podName   = rsName + "-12345"
			namespace = "kubevirt-hyperconverged"
		)

		var recorder *EventRecorderMock
		BeforeEach(func() {
			recorder = newEventRecorderMock()
		})

		ee := eventEmitter{
			pod:      nil,
			ownerRef: nil,
		}

		It("should not update pod if the pod not found", func() {
			justACmForTest := &corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "justACmForTest", Namespace: namespace},
			}

			ee.Init(nil, nil, recorder)
			Expect(ee.pod).To(BeNil())
			Expect(ee.ownerRef).To(BeNil())

			By("should emit event for all three resources", func() {
				// we'll use the replica set as object, because we just need one. Originally we would use the HyperConverged
				// resource, but this is not accessible (cyclic import)
				expectedEvent := eventMock{
					eventType: corev1.EventTypeNormal,
					reason:    "justTesting",
					message:   "this is a test message",
				}

				ee.EmitEvent(justACmForTest, corev1.EventTypeNormal, "justTesting", "this is a test message")
				mock := ee.recorder.(*EventRecorderMock)

				Expect(mock.events).To(HaveLen(1))
				rsEvent, found := mock.events["ConfigMap"]
				Expect(found).To(BeTrue())
				Expect(rsEvent).To(Equal(expectedEvent))

				_, found = mock.events["Pod"]
				Expect(found).To(BeFalse())

				_, found = mock.events["ClusterServiceVersion"]
				Expect(found).To(BeFalse())
			})
		})

		It("should update pod and ownerRef if they are found", func() {
			csvRef := &corev1.ObjectReference{
				Kind:            "ClusterServiceVersion",
				Namespace:       namespace,
				Name:            rsName,
				UID:             "0266392e-0153-519e-ce97-4eb5b90020e8",
				APIVersion:      "operators.coreos.com/v1alpha1",
				ResourceVersion: "",
			}

			pod := &corev1.Pod{
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

			ee.Init(pod, csvRef, recorder)

			Expect(ee.pod).ToNot(BeNil())
			Expect(ee.ownerRef).ToNot(BeNil())

			By("should emit event for all three resources")
			// we'll use the replica set as object, because we just need one. Originally we would use the HyperConverged
			// resource, but this is not accessible (cyclic import)
			expectedEvent := eventMock{
				eventType: corev1.EventTypeNormal,
				reason:    "justTesting",
				message:   "this is a test message",
			}

			hc := &v1beta1.HyperConverged{
				TypeMeta: metav1.TypeMeta{Kind: "HyperConverged", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      HyperConvergedName,
					Namespace: namespace,
				},
			}

			ee.EmitEvent(hc, corev1.EventTypeNormal, "justTesting", "this is a test message")
			mock := ee.recorder.(*EventRecorderMock)

			Expect(mock.events).To(HaveLen(3))

			rsEvent, found := mock.events["HyperConverged"]
			Expect(found).To(BeTrue())
			Expect(rsEvent).To(Equal(expectedEvent))

			rsEvent, found = mock.events["Pod"]
			Expect(found).To(BeTrue())
			Expect(rsEvent).To(Equal(expectedEvent))

			rsEvent, found = mock.events["ClusterServiceVersion"]
			Expect(found).To(BeTrue())
			Expect(rsEvent).To(Equal(expectedEvent))
		})

		It("should not update resource if it's nil", func() {
			ee.Init(nil, nil, recorder)

			By("should not emit event for all three resources", func() {
				ee.EmitEvent(nil, corev1.EventTypeNormal, "justTesting", "this is a test message")
				mock := ee.recorder.(*EventRecorderMock)
				Expect(mock.events).To(BeEmpty())
			})

		})

		It("should not update resource if it's empty", func() {
			var rs *appsv1.ReplicaSet = nil

			ee.Init(nil, nil, recorder)

			By("should not emit event for all three resources", func() {
				ee.EmitEvent(rs, corev1.EventTypeNormal, "justTesting", "this is a test message")
				mock := ee.recorder.(*EventRecorderMock)
				Expect(mock.events).To(BeEmpty())
			})
		})
	})
})

type eventMock struct {
	eventType string
	reason    string
	message   string
}

type EventRecorderMock struct {
	events map[string]eventMock
}

func newEventRecorderMock() *EventRecorderMock {
	return &EventRecorderMock{
		events: make(map[string]eventMock),
	}
}

func (mock EventRecorderMock) Event(object runtime.Object, eventType, reason, message string) {
	kind := object.GetObjectKind().GroupVersionKind().Kind
	mock.events[kind] = eventMock{eventType: eventType, reason: reason, message: message}
}
func (mock EventRecorderMock) Eventf(_ runtime.Object, _, _, _ string, _ ...interface{}) {
	/* not implemented */
}
func (mock EventRecorderMock) AnnotatedEventf(_ runtime.Object, _ map[string]string, _, _, _ string, _ ...interface{}) {
	/* not implemented */
}
