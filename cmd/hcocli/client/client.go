package client

import (
	"flag"
	"fmt"

	openshiftcv1 "github.com/openshift/api/config/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cr "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

func GetClient(kubeconfig string) (crclient.Client, error) {
	if len(kubeconfig) > 0 {
		fs := &flag.FlagSet{}
		fs.String("kubeconfig", "", "")
		err := fs.Parse([]string{"-kubeconfig", kubeconfig})
		if err != nil {
			return nil, err
		}

		config.RegisterFlags(fs)
	}

	cfg, err := cr.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("can't get cluster configurations: %w", err)
	}

	s, err := createScheme()
	if err != nil {
		return nil, fmt.Errorf("can't initiate scheme: %w", err)
	}

	cl, err := crclient.New(cfg, crclient.Options{Scheme: s})
	if err != nil {
		return nil, fmt.Errorf("can't connect the kubernetes cluster: %w", err)
	}

	return cl, nil
}

func createScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	for _, fn := range []func(scheme *runtime.Scheme) error{
		corev1.AddToScheme,
		extv1.AddToScheme,
		appv1.AddToScheme,
		v1beta1.AddToScheme,
		openshiftcv1.Install,
	} {
		if err := fn(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}
