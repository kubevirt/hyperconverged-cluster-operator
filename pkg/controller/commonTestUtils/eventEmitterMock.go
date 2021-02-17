package commonTestUtils

import (
	"context"
	"github.com/go-logr/logr"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
)

type MockEvent struct {
	EventType string
	Reason    string
	Msg       string
}

type EventEmitterMock struct {
	expectedEvents *sync.Map
}

func NewEventEmitterMock(expectedEvents ...MockEvent) *EventEmitterMock {
	eem := &EventEmitterMock{}
	eem.SetExpectedEvents(expectedEvents...)

	return eem
}

func (eem *EventEmitterMock) SetExpectedEvents(expectedEvents ...MockEvent) {
	events := &sync.Map{}
	for _, event := range expectedEvents {
		events.Store(event, false)
	}

	eem.expectedEvents = events
}

func (EventEmitterMock) Init(_ context.Context, _ manager.Manager, _ hcoutil.ClusterInfo, _ logr.Logger) {
	/* not implemented; mock only */
}

func (eem *EventEmitterMock) EmitEvent(_ runtime.Object, eventType, reason, msg string) {
	eem.expectedEvents.Store(MockEvent{
		EventType: eventType,
		Reason:    reason,
		Msg:       msg,
	}, true)
}

func (EventEmitterMock) UpdateClient(_ context.Context, _ client.Reader, _ logr.Logger) {
	/* not implemented; mock only */
}

func (eem EventEmitterMock) GotExpectedEvent() bool {
	gotAllEvents := true
	eem.expectedEvents.Range(func(key, value interface{}) bool {
		gotAllEvents = gotAllEvents && value.(bool)
		return value.(bool)
	})

	return gotAllEvents
}
