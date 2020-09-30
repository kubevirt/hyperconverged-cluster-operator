package hyperconverged

import (
	"context"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/operands"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CleanupNs(ctx context.Context, mgr manager.Manager, namespace string, logger logr.Logger) error {
	err := (&hcov1beta1.HyperConverged{}).RemoveWebhookWithManager(ctx, mgr)
	if err != nil {
		logger.Error(err, "unable to remove webhook", "webhook", "HyperConverged")
	}
	err = dropFinalizers(ctx, mgr, namespace, logger)
	if err != nil {
		logger.Error(err, "Failed removing HCO and KV finalizers")
	}
	err = dropCDIAPIServices(ctx, mgr, logger)
	if err != nil {
		logger.Error(err, "Failed removing CDI APIServices")
	}
	return err
}

func dropFinalizers(ctx context.Context, mgr manager.Manager, namespace string, logger logr.Logger) error {
	c := mgr.GetClient()
	hco := &hcov1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcov1beta1.HyperConvergedName,
			Namespace: namespace,
		},
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hyperconverged-cluster-operator", // TODO: avoid hard-coding it
			Namespace: namespace,
		},
	}
	for _, obj := range []runtime.Object{
		hco,
		operands.NewKubeVirt(hco),
		sa,
	} {
		key, err := client.ObjectKeyFromObject(obj)
		if err != nil {
			logger.Error(err, "Failed to get object key")
			break
		}
		// try in a loop, with 1 second delay, until the validating webhook gets removed
		// and we can successfully remove the finalizer or until terminationGracePeriodSeconds hard timeout
		for {
			err = c.Get(ctx, key, obj)
			if err != nil {
				logger.Error(err, "Failed to get the CR")
				return err
			}
			switch obj.(type) {
			case *kubevirtv1.KubeVirt:
				objt := obj.(*kubevirtv1.KubeVirt)
				objt.ObjectMeta.Finalizers = []string{}
			case *hcov1beta1.HyperConverged:
				objt := obj.(*hcov1beta1.HyperConverged)
				objt.ObjectMeta.Finalizers = []string{}
			}
			err = c.Update(ctx, obj)
			if err != nil {
				logger.Error(err, "Failed to remove the all the finalizers on the CR, trying again...")
			} else {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

func dropCDIAPIServices(ctx context.Context, mgr manager.Manager, logger logr.Logger) error {
	cdiapiservicenames := []string{
		"v1alpha1.upload.cdi.kubevirt.io",
		"v1beta1.upload.cdi.kubevirt.io",
	}
	var err error = nil
	for _, apiservicename := range cdiapiservicenames {
		derr := dropAPIService(ctx, mgr, logger, apiservicename)
		if derr != nil && err == nil {
			err = derr
		}
	}
	if err != nil {
		logger.Error(err, "Failed to removing one of the CDI ApiServices")
		return err
	}
	return nil
}

func dropAPIService(ctx context.Context, mgr manager.Manager, logger logr.Logger, apiservicename string) error {
	c := mgr.GetClient()
	cdiapiservice := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiservicename,
		},
	}
	key, err := client.ObjectKeyFromObject(cdiapiservice)
	if err != nil {
		logger.Error(err, "Failed to get object key")
		return err
	}
	err = c.Get(ctx, key, cdiapiservice)
	if err != nil {
		logger.Error(err, "Failed to get the APIService")
		return err
	}
	err = c.Delete(ctx, cdiapiservice)
	if err != nil {
		logger.Error(err, "Failed to remove ApiService")
	}

	return nil
}
