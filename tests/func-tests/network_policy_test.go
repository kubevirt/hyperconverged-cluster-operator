package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	clientPodName        = "client"
	testServerPodName    = "my-server"
	testServerPort       = int32(8790)
	allowAllNPPluginName = "test-allow-all-plugin-np"
	allowAllNPProxyName  = "test-allow-all-proxy-np"
)

var _ = Describe("Network Policy Tests", Serial, Label(tests.OpenshiftLabel, "network-policy"), func() {
	var (
		cli          client.Client
		k8sClientSet *kubernetes.Clientset
	)

	tests.FlagParse()

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		tests.FailIfNotOpenShift(ctx, cli, "kubevirt console plugin")

		k8sClientSet = tests.GetK8sClientSet()
	})

	Context("check the console plugin network policy", func() {
		It("should find the kubevirt-console-plugin-np network policy", func(ctx context.Context) {
			np, err := k8sClientSet.NetworkingV1().NetworkPolicies(tests.InstallNamespace).Get(ctx, "kubevirt-console-plugin-np", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(np).ToNot(BeNil())
		})

		// Note: positive test is done in console_plugin_test.go, where we send an HTTP request from the console pod,
		//       to the console plugin pods.

		Context("test ingress from unauthorized sources", func() {
			var clientPod *corev1.Pod
			BeforeEach(func(ctx context.Context) {
				clientPod = createClientPod()
				Expect(createPod(ctx, cli, clientPod)).To(Succeed())

				DeferCleanup(func(ctx context.Context) {
					Expect(deleteResource(ctx, cli, clientPod)).To(Succeed())
				})
			})

			Context("with allow-all network policy", func() {
				BeforeEach(func(ctx context.Context) {
					np := createAllowAllIngressNetworkPolicy()
					By(fmt.Sprintf("Creating NetworkPolicy %s/%s", tests.InstallNamespace, np.Name))
					Expect(cli.Create(ctx, np)).To(Succeed())

					DeferCleanup(func(ctx context.Context) {
						Expect(deleteResource(ctx, cli, np)).To(Succeed())
					})
				})

				It("should succeed to call the pod", func(ctx context.Context) {
					Eventually(func(g Gomega, ctx context.Context) {
						stdout, stderr, err := sendReqToConsolePlugin(ctx, cli, k8sClientSet, clientPod)
						g.Expect(err).ToNot(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
						g.Expect(stdout).ToNot(BeEmpty(), "stdout is empty")
						g.Expect(stderr).To(BeEmpty(), "stderr: %s", stderr)
					}).WithContext(ctx).
						WithTimeout(2 * time.Minute).
						WithPolling(10 * time.Second).
						Should(Succeed())
				})
			})

			Context("without allow-all network policy", func() {
				It("should fail to call the pod", func(ctx context.Context) {
					_, stderr, err := sendReqToConsolePlugin(ctx, cli, k8sClientSet, clientPod)
					Expect(err).To(HaveOccurred())
					GinkgoLogr.Error(err, "EXPECTED: should be blocked by the network policy", "stderr", stderr)
				})
			})
		})
	})

	Context("apiserver-proxy network policy", func() {
		It("should find the kubevirt-apiserver-proxy-np network policy", func(ctx context.Context) {
			np, err := k8sClientSet.NetworkingV1().NetworkPolicies(tests.InstallNamespace).Get(ctx, "kubevirt-apiserver-proxy-np", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(np).ToNot(BeNil())
		})

		Context("check the NetworkPolicy", func() {
			var (
				testServerSvc *corev1.Service
				pods          []corev1.Pod
			)

			BeforeEach(func(ctx context.Context) {
				pods = getAPIServerProxyPods()

				testServerPod := createTestServerPod()
				testServerSvc = createTestServerService()

				Expect(cli.Create(ctx, testServerPod)).To(Succeed())
				Expect(cli.Create(ctx, testServerSvc)).To(Succeed())

				DeferCleanup(func(ctx context.Context) {
					Expect(deleteResource(ctx, cli, testServerPod)).To(Succeed())
					Expect(deleteResource(ctx, cli, testServerSvc)).To(Succeed())
				})
			})

			It("should fail to send request from apiServer-proxy pod ", func(ctx context.Context) {
				for _, pod := range pods {
					stdout, stderr, err := sendReqToTestServer(ctx, k8sClientSet, &pod, testServerSvc)
					Expect(err).To(HaveOccurred(), "stdout: %q; stderr: %q", stdout, stderr)
				}
			})

			Context("with allow-all network policy", func() {
				BeforeEach(func(ctx context.Context) {
					np := createAllowAllEgressNetworkPolicy()
					By(fmt.Sprintf("Creating NetworkPolicy %s/%s", tests.InstallNamespace, np.Name))
					Expect(cli.Create(ctx, np)).To(Succeed())

					DeferCleanup(func(ctx context.Context) {
						Expect(deleteResource(ctx, cli, np)).To(Succeed())
					})
				})

				It("should be able to call the pod", func(ctx context.Context) {
					for _, pod := range pods {
						Eventually(func(ctx context.Context) error {
							_, _, err := sendReqToTestServer(ctx, k8sClientSet, &pod, testServerSvc)
							return err
						}).WithContext(ctx).WithTimeout(30 * time.Second).
							WithTimeout(30 * time.Second).
							WithPolling(5 * time.Second).
							Should(Succeed())
					}
				})
			})
		})
	})
})

func getAPIServerProxyPods() []corev1.Pod {
	GinkgoHelper()

	var pods = &corev1.PodList{}

	nsOpt := client.InNamespace(tests.InstallNamespace)
	matchLblOpt := client.MatchingLabels{
		hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIProxy),
	}

	Expect(
		tests.GetControllerRuntimeClient().List(context.Background(), pods, nsOpt, matchLblOpt),
	).To(Succeed())

	Expect(pods.Items).ToNot(BeEmpty(), "no APIServer-proxy pods found")

	return pods.Items
}

func createClientPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clientPodName,
			Namespace: tests.InstallNamespace,
			Labels: map[string]string{
				"run": clientPodName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Args:    []string{clientPodName},
					Command: []string{"sh", "-c", "sleep 3600"},
					Image:   "registry.fedoraproject.org/fedora:43",
					Name:    clientPodName,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  ptr.To[int64](1000),
				RunAsGroup: ptr.To[int64](3000),
				FSGroup:    ptr.To[int64](2000),
			},
		},
	}
}

func createTestServerPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerPodName,
			Namespace: tests.InstallNamespace,
			Labels: map[string]string{
				"run": testServerPodName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Args: []string{testServerPodName},
					Command: []string{"/bin/bash",
						"-c",
						fmt.Sprintf(`while true; do echo -e "HTTP/1.1 200 OK\\n\\nServer response $(date)" | \
nc -l -p %d; done`, testServerPort),
					},
					Image: "quay.io/openshift-cnv/qe-cnv-tests-net-util-container:centos-stream-9",
					Name:  testServerPodName,
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: testServerPort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  ptr.To[int64](1000),
				RunAsGroup: ptr.To[int64](3000),
				FSGroup:    ptr.To[int64](2000),
			},
		},
	}
}

func createPod(ctx context.Context, cli client.Client, pod *corev1.Pod) error {
	By(fmt.Sprintf("Creating the %s/%s pod", pod.Namespace, pod.Name))
	GinkgoHelper()

	err := cli.Create(ctx, pod)
	if err != nil {
		return err
	}

	By(fmt.Sprintf("The %s/%s pod was created; waiting for it to start running...", pod.Namespace, pod.Name))
	key := client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}
	Eventually(func(g Gomega, ctx context.Context) {
		resPod := &corev1.Pod{}
		g.Expect(cli.Get(ctx, key, resPod)).To(Succeed())

		g.Expect(resPod.Status.Phase).To(Equal(corev1.PodRunning))
	}).WithContext(ctx).
		WithTimeout(3 * time.Minute).
		WithPolling(10 * time.Second).
		Should(Succeed())

	return nil
}

func sendReqToConsolePlugin(ctx context.Context, cli client.Client, k8sClientSet *kubernetes.Clientset, clientPod *corev1.Pod) (string, string, error) {
	GinkgoHelper()

	kubevirtPlugin := &consolev1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: expectedKubevirtConsolePluginName,
		},
	}

	Expect(cli.Get(ctx, client.ObjectKeyFromObject(kubevirtPlugin), kubevirtPlugin)).To(Succeed())

	pluginServiceName := kubevirtPlugin.Spec.Backend.Service.Name
	pluginServicePort := kubevirtPlugin.Spec.Backend.Service.Port

	command := fmt.Sprintf(`curl -ks https://%s.%s:%d/plugin-manifest.json`,
		pluginServiceName, tests.InstallNamespace, pluginServicePort)

	toCtx, toCancel := context.WithTimeout(ctx, 5*time.Second)
	defer toCancel()

	By("Sending an HTTP request from the client pod to the console plugin pods")
	return executeCommandOnPod(toCtx, k8sClientSet, clientPod, command)
}

func sendReqToTestServer(ctx context.Context, k8sClientSet *kubernetes.Clientset, fromPod *corev1.Pod, toSvc *corev1.Service) (string, string, error) {
	command := fmt.Sprintf(`curl -ks http://%s.%s.svc:%d`,
		toSvc.Name, toSvc.Namespace, toSvc.Spec.Ports[0].Port)

	toCtx, toCancel := context.WithTimeout(ctx, 5*time.Second)
	defer toCancel()

	By("Sending an HTTP request from the client pod to the console plugin pods")
	return executeCommandOnPod(toCtx, k8sClientSet, fromPod, command)
}

func createAllowAllIngressNetworkPolicy() *networkingv1.NetworkPolicy {
	hc := &hcov1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcov1beta1.HyperConvergedName,
			Namespace: tests.InstallNamespace,
		},
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      allowAllNPPluginName,
			Namespace: tests.InstallNamespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
		},

		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					hcoutil.AppLabel:          hc.Name,
					hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIPlugin),
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}

func createAllowAllEgressNetworkPolicy() *networkingv1.NetworkPolicy {
	hc := &hcov1beta1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcov1beta1.HyperConvergedName,
			Namespace: tests.InstallNamespace,
		},
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      allowAllNPProxyName,
			Namespace: tests.InstallNamespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIProxy),
		},

		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					hcoutil.AppLabel:          hc.Name,
					hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIProxy),
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	}
}

func createTestServerService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testServerPodName + "-service",
			Namespace: tests.InstallNamespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       testServerPort,
					TargetPort: intstr.FromInt32(testServerPort),
				},
			},
			Selector: map[string]string{
				"run": testServerPodName,
			},
		},
	}
}

func deleteResource(ctx context.Context, cli client.Client, resource client.Object) error {
	GinkgoHelper()

	key := client.ObjectKeyFromObject(resource)

	gvk, err := cli.GroupVersionKindFor(resource)
	if err != nil {
		return err
	}

	By(fmt.Sprintf("Removing the %s/%s %s", resource.GetNamespace(), resource.GetName(), gvk.Kind))

	err = cli.Delete(ctx, resource)
	if err != nil {
		return err
	}

	Eventually(func(g Gomega, ctx context.Context) error {
		return cli.Get(ctx, key, resource)
	}).WithContext(ctx).
		WithTimeout(2 * time.Minute).
		WithPolling(10 * time.Second).
		Should(MatchError(k8serrors.IsNotFound, "is not found error"))

	return nil
}
