package mutator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commonTestUtils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	hcoNamespace = HcoValidNamespace
	podNamespace = "fake-namespace"
)

var _ = Describe("virt-launcher webhook mutator", func() {

	It("TODO", func() {
		hco := getHco()
		mutator := getVirtLauncherMutator()
		launcherPod := getFakeLauncherPod()

		_ = mutator.handleVirtLauncherCreation(launcherPod, hco)
		Expect(true).To(BeTrue())
	})

})

func getVirtLauncherMutator() *VirtLauncherMutator {
	hco := getHco()
	cli := commonTestUtils.InitClient([]runtime.Object{hco})

	return NewVirtLauncherMutator(cli, hcoNamespace)
}

func getHco() *v1beta1.HyperConverged {
	return &v1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.HyperConvergedName,
			Namespace: HcoValidNamespace,
		},
		Spec: v1beta1.HyperConvergedSpec{},
	}
}

func getFakeLauncherPod() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "virt-launcher-vmi-" + rand.String(5),
			Namespace: podNamespace,
		},
	}
}
