package tests

import (
	"flag"
	"sync"

	"github.com/onsi/ginkgo/v2"
	consolev1 "github.com/openshift/api/console/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	aaqv1alpha1 "kubevirt.io/application-aware-quota/staging/src/kubevirt.io/application-aware-quota-api/pkg/apis/core/v1alpha1"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var (
	k8sCli       client.Client
	k8sClientSet *kubernetes.Clientset
	cfg          *rest.Config
)

func GetClientConfig() *rest.Config {
	k8sconfig.RegisterFlags(flag.CommandLine)
	logf.SetLogger(ginkgo.GinkgoLogr)
	
	once := sync.Once{}
	once.Do(func() {
		cfg = k8sconfig.GetConfigOrDie()
	})
	return cfg
}

func GetK8sClientSet() *kubernetes.Clientset {
	once := sync.Once{}
	once.Do(func() {
		var err error
		k8sClientSet, err = kubernetes.NewForConfig(GetClientConfig())
		if err != nil {
			panic("can't get  client: " + err.Error())
		}
	})
	return k8sClientSet
}

func GetK8sClient() client.Client {
	once := sync.Once{}
	once.Do(func() {
		var err error

		k8sCli, err = client.New(GetClientConfig(), client.Options{})
		if err != nil {
			panic("can't get  client: " + err.Error())
		}
		setScheme(k8sCli)
	})

	return k8sCli
}

func setScheme(cli client.Client) {
	once := sync.Once{}
	once.Do(func() {
		funcs := []func(scheme2 *runtime.Scheme) error{
			corev1.AddToScheme,
			appsv1.AddToScheme,
			v1beta1.AddToScheme,
			aaqv1alpha1.AddToScheme,
			consolev1.AddToScheme,
		}

		for _, f := range funcs {
			err := f(cli.Scheme())
			if err != nil {
				panic(err)
			}
		}
	})
}
