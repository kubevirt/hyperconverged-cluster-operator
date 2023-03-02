package tests

import (
	"context"
	"flag"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	kvtests "kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/flags"
	kvtutil "kubevirt.io/kubevirt/tests/util"

	g "github.com/onsi/ginkgo/v2"
)

var _ = g.Describe("virt-launcher mutating webhook", func() {
	flag.Parse()

	var hcoOriginalAnnotations map[string]string

	g.BeforeEach(func() {
		BeforeEach()

		hco := getHco()
		hcoOriginalAnnotations = hco.Annotations
	})

	g.AfterEach(func() {
		hco := getHco()
		hco.Annotations = hcoOriginalAnnotations
		hcoOriginalAnnotations = map[string]string{}

		setHco(hco)
	})

	g.Context("guest-to-request memory headroom", func() {

		enableWebhook := func(customHeadroom *string) {
			hco := getHco()
			if hco.Annotations == nil {
				hco.Annotations = map[string]string{}
			}

			hco.Annotations["kubevirt.io/enable-guest-to-request-memory-headroom"] = "true"
			if customHeadroom != nil {
				hco.Annotations["kubevirt.io/custom-guest-to-request-memory-headroom"] = *customHeadroom
			}

			setHco(hco)
		}

		g.It("with default headroom", func() {
			enableWebhook(nil)

			vmi := kvtests.NewRandomVMI()
			vmi = kvtests.RunVMIAndExpectLaunch(vmi, 60)
			launcherPod := kvtests.GetRunningPodByVirtualMachineInstance(vmi, vmi.Namespace)

			vmiMemRequest := vmi.Spec.Domain.Resources.Requests[v1.ResourceMemory]

			expectedPodRequest := vmiMemRequest.DeepCopy()
			expectedPodRequest.Add(resource.MustParse("2G"))

			actualPodRequest := getComputeContainer(launcherPod).Resources.Requests[v1.ResourceMemory]

			Expect(actualPodRequest.Cmp(expectedPodRequest)).To(BeTrue(), fmt.Sprintf("expecting pod requests (%v) to equal %v", actualPodRequest, expectedPodRequest))
		})

	})

})

func getHco() *hcov1beta1.HyperConverged {
	virtCli, err := kubecli.GetKubevirtClient()
	Expect(err).ToNot(HaveOccurred())

	SkipIfNotOpenShift(virtCli, "DataImportCronTemplate")

	cli, err := kubecli.GetKubevirtClientFromRESTConfig(virtCli.Config())
	Expect(err).ToNot(HaveOccurred())

	hc := &hcov1beta1.HyperConverged{}
	Expect(cli.RestClient().
		Get().
		Resource("hyperconvergeds").
		Name("kubevirt-hyperconverged").
		Namespace(flags.KubeVirtInstallNamespace).
		AbsPath("/apis", hcov1beta1.SchemeGroupVersion.Group, hcov1beta1.SchemeGroupVersion.Version).
		Timeout(10 * time.Second).
		Do(context.TODO()).
		Into(hc),
	).To(Succeed())

	return hc
}

func setHco(hc *hcov1beta1.HyperConverged) *hcov1beta1.HyperConverged {
	virtCli, err := kubecli.GetKubevirtClient()
	Expect(err).ToNot(HaveOccurred())

	SkipIfNotOpenShift(virtCli, "DataImportCronTemplate")

	cli, err := kubecli.GetKubevirtClientFromRESTConfig(virtCli.Config())
	Expect(err).ToNot(HaveOccurred())

	res := cli.RestClient().Put().
		Resource("hyperconvergeds").
		Name(hcov1beta1.HyperConvergedName).
		Namespace(flags.KubeVirtInstallNamespace).
		AbsPath("/apis", hcov1beta1.SchemeGroupVersion.Group, hcov1beta1.SchemeGroupVersion.Version).
		Timeout(10 * time.Second).
		Body(hc).Do(context.TODO())

	Expect(res.Error()).ToNot(HaveOccurred())
	newHC := &hcov1beta1.HyperConverged{}
	err = res.Into(newHC)
	Expect(err).ShouldNot(HaveOccurred())

	return newHC
}

func createAndRunVmi() *kubevirtcorev1.VirtualMachineInstance {
	virtCli, err := kubecli.GetKubevirtClient()
	Expect(err).ToNot(HaveOccurred())

	vmi := kvtests.NewRandomVMI()
	g.By(fmt.Sprintf("Creating VMI %s", vmi.Name))
	EventuallyWithOffset(1, func() error {
		var err error
		vmi, err = virtCli.VirtualMachineInstance(kvtutil.NamespaceTestDefault).Create(context.Background(), vmi)
		return err
	}, 60*time.Second, 2*time.Second).Should(Succeed(), "failed to create a vmi")

	g.By("Verifying VMI is running")
	EventuallyWithOffset(1, func(g Gomega) kubevirtcorev1.VirtualMachineInstancePhase {
		var err error
		vmi, err = virtCli.VirtualMachineInstance(kvtutil.NamespaceTestDefault).Get(context.Background(), vmi.Name, &k8smetav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		return vmi.Status.Phase
	}, 60*time.Second, 2*time.Second).Should(Equal(kubevirtcorev1.Running), "failed to get the vmi Running")

	return vmi
}

func getComputeContainer(launcherPod *v1.Pod) v1.Container {
	for _, container := range launcherPod.Spec.Containers {
		if container.Name == "compute" {
			return container
		}
	}

	Expect(true).To(BeFalse(), "could not find compute container")
	return v1.Container{}
}
