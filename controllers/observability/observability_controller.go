package observability

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log         = logf.Log.WithName("controller_observability")
	periodicity = 5 * time.Second
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	events chan event.GenericEvent
}

func (r *Reconciler) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log.Info("Reconciling Observability")

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	log.Info("Setting up controller")

	r.events = make(chan event.GenericEvent)
	go func() {
		for {
			r.events <- event.GenericEvent{
				Object: &metav1.PartialObjectMetadata{},
			}
			time.Sleep(periodicity)
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		Named("observability").
		WatchesRawSource(source.Channel(
			r.events,
			&handler.EnqueueRequestForObject{},
		)).
		Complete(r)
}
