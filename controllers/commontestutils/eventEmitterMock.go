package commontestutils

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type MockEvent struct {
	EventType string
	Reason    string
	Msg       string
}

type EventEmitterMock struct {
	storedEvents []MockEvent
	lock         *sync.Mutex
}

var _ hcoutil.EventEmitter = &EventEmitterMock{}

func NewEventEmitterMock() *EventEmitterMock {
	return &EventEmitterMock{
		storedEvents: make([]MockEvent, 0),
		lock:         &sync.Mutex{},
	}
}

func (eem *EventEmitterMock) Reset() {
	eem.lock.Lock()
	defer eem.lock.Unlock()

	eem.storedEvents = make([]MockEvent, 0)
}

func (*EventEmitterMock) Init(_ *corev1.Pod, _ *corev1.ObjectReference, _ record.EventRecorder) {
	/* not implemented; mock only */
}

func (eem *EventEmitterMock) EmitEvent(_ runtime.Object, eventType, reason, msg string) {
	event := MockEvent{
		EventType: eventType,
		Reason:    reason,
		Msg:       msg,
	}

	eem.lock.Lock()
	defer eem.lock.Unlock()

	eem.storedEvents = append(eem.storedEvents, event)
}

func (*EventEmitterMock) UpdateClient(_ context.Context, _ client.Reader, _ logr.Logger) {
	/* not implemented; mock only */
}

func (eem *EventEmitterMock) CheckEvents(expectedEvents []MockEvent) bool {
	eem.lock.Lock()
	defer eem.lock.Unlock()

	for _, expectedEvent := range expectedEvents {
		if !eventInArray(eem.storedEvents, expectedEvent) {
			return false
		}
	}

	return true
}

func (eem *EventEmitterMock) CheckNoEventEmitted() bool {
	eem.lock.Lock()
	defer eem.lock.Unlock()

	return len(eem.storedEvents) == 0
}

func eventInArray(eventList []MockEvent, event MockEvent) bool {
	for _, expectedEvent := range eventList {
		if reflect.DeepEqual(event, expectedEvent) {
			return true
		}
	}
	return false
}
