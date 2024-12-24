package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	openshiftConsoleNamespace         = "openshift-console"
	expectedKubevirtConsolePluginName = "kubevirt-plugin"
)

var _ = Describe("kubevirt console plugin", Label(tests.OpenshiftLabel, "consolePlugin"), func() {

	var (
		cli          client.Client
		k8sClientSet *kubernetes.Clientset
	)

	tests.FlagParse()

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		tests.FailIfNotOpenShift(ctx, cli, "kubevirt console plugin")

		hco := tests.GetHCO(ctx, cli)
		originalInfra := hco.Spec.Infra

		k8sClientSet = tests.GetK8sClientSet()

		DeferCleanup(func(ctx context.Context) {
			hco.Spec.Infra = originalInfra
			tests.UpdateHCORetry(ctx, cli, hco)
		})
	})

	It("console should reach kubevirt-plugin manifests", func(ctx context.Context) {
		kubevirtPlugin := &consolev1.ConsolePlugin{
			ObjectMeta: metav1.ObjectMeta{
				Name: expectedKubevirtConsolePluginName,
			},
		}

		Expect(cli.Get(ctx, client.ObjectKeyFromObject(kubevirtPlugin), kubevirtPlugin)).To(Succeed())

		pluginServiceName := kubevirtPlugin.Spec.Backend.Service.Name
		pluginServicePort := kubevirtPlugin.Spec.Backend.Service.Port

		hcoPods := &corev1.PodList{}
		Expect(cli.List(ctx, hcoPods, client.MatchingLabels{
			"name": "hyperconverged-cluster-operator",
		}, client.InNamespace(tests.InstallNamespace))).To(Succeed())

		Expect(hcoPods.Items).ToNot(BeEmpty())

		testConsolePod := hcoPods.Items[0]
		command := fmt.Sprintf(`curl -ks https://%s:%d/plugin-manifest.json`,
			pluginServiceName, pluginServicePort)

		Eventually(func(g Gomega, ctx context.Context) string {
			stdout, stderr, err := executeCommandOnPod(ctx, k8sClientSet, &testConsolePod, command)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(stdout).ToNot(BeEmpty())
			g.Expect(stderr).To(BeEmpty())

			var pluginManifests map[string]interface{}
			err = json.Unmarshal([]byte(stdout), &pluginManifests)
			g.Expect(err).ToNot(HaveOccurred())

			pluginName, ok := pluginManifests["name"]
			g.Expect(ok).To(BeTrue())
			return pluginName.(string)
		}).
			WithContext(ctx).
			WithTimeout(60 * time.Second).
			WithPolling(time.Second).
			Should(Equal(expectedKubevirtConsolePluginName))
	})

	It("nodePlacement should be propagated from HyperConverged CR to console-plugin and apiserver-proxy Deployments", Serial, func(ctx context.Context) {

		expectedNodeSelector := map[string]string{
			"foo": "bar",
		}
		expectedNodeSelectorBytes, err := json.Marshal(expectedNodeSelector)
		Expect(err).ToNot(HaveOccurred())
		expectedNodeSelectorStr := string(expectedNodeSelectorBytes)
		addNodeSelectorPatch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/spec/infra", "value": {"nodePlacement": {"nodeSelector": %s}}}]`, expectedNodeSelectorStr))

		Eventually(func(ctx context.Context) error {
			err = tests.PatchHCO(ctx, cli, addNodeSelectorPatch)
			return err
		}).WithTimeout(1 * time.Minute).
			WithPolling(1 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			consoleUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIPlugin),
					Namespace: tests.InstallNamespace,
				},
			}

			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(consoleUIDeployment), consoleUIDeployment)).To(Succeed())

			g.Expect(consoleUIDeployment.Spec.Template.Spec.NodeSelector).To(Equal(expectedNodeSelector))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			proxyUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIProxy),
					Namespace: tests.InstallNamespace,
				},
			}
			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(proxyUIDeployment), proxyUIDeployment)).To(Succeed())
			g.Expect(proxyUIDeployment.Spec.Template.Spec.NodeSelector).To(Equal(expectedNodeSelector))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		// clear node placement from HyperConverged CR and verify the nodeSelector has been cleared as well from the UI Deployments
		removeNodeSelectorPatch := []byte(`[{"op": "replace", "path": "/spec/infra", "value": {}}]`)
		Eventually(func(ctx context.Context) error {
			err = tests.PatchHCO(ctx, cli, removeNodeSelectorPatch)
			return err
		}).WithTimeout(1 * time.Minute).
			WithPolling(1 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			consoleUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIPlugin),
					Namespace: tests.InstallNamespace,
				},
			}

			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(consoleUIDeployment), consoleUIDeployment)).To(Succeed())
			g.Expect(consoleUIDeployment.Spec.Template.Spec.NodeSelector).To(BeEmpty())
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			proxyUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIProxy),
					Namespace: tests.InstallNamespace,
				},
			}
			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(proxyUIDeployment), proxyUIDeployment)).To(Succeed())
			g.Expect(proxyUIDeployment.Spec.Template.Spec.NodeSelector).To(BeEmpty())
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())
	})

	It("console-plugin and apiserver-proxy Deployments should have 2 replicas in Highly Available clusters", Label(tests.HighlyAvailableClusterLabel), func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) {
			consoleUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIPlugin),
					Namespace: tests.InstallNamespace,
				},
			}

			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(consoleUIDeployment), consoleUIDeployment)).To(Succeed())

			g.Expect(consoleUIDeployment.Spec.Replicas).To(HaveValue(Equal(int32(2))))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			proxyUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIProxy),
					Namespace: tests.InstallNamespace,
				},
			}
			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(proxyUIDeployment), proxyUIDeployment)).To(Succeed())
			g.Expect(proxyUIDeployment.Spec.Replicas).To(HaveValue(Equal(int32(2))))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())
	})

	It("console-plugin and apiserver-proxy Deployments should have 1 replica in single node clusters", Label(tests.SingleNodeLabel), func(ctx context.Context) {
		Eventually(func(g Gomega, ctx context.Context) {
			consoleUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIPlugin),
					Namespace: tests.InstallNamespace,
				},
			}

			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(consoleUIDeployment), consoleUIDeployment)).To(Succeed())

			g.Expect(consoleUIDeployment.Spec.Replicas).To(HaveValue(Equal(int32(1))))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			proxyUIDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(hcoutil.AppComponentUIProxy),
					Namespace: tests.InstallNamespace,
				},
			}
			g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(proxyUIDeployment), proxyUIDeployment)).To(Succeed())
			g.Expect(proxyUIDeployment.Spec.Replicas).To(HaveValue(Equal(int32(1))))
		}).WithTimeout(1 * time.Minute).
			WithPolling(100 * time.Millisecond).
			WithContext(ctx).
			Should(Succeed())
	})
})

func executeCommandOnPod(ctx context.Context, k8scli *kubernetes.Clientset, pod *corev1.Pod, command string) (string, string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := k8scli.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", command},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(tests.GetClientConfig(), "POST", request.URL())
	if err != nil {
		return "", "", fmt.Errorf("%w: failed to create pod executor for %v/%v", err, pod.Namespace, pod.Name)
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", fmt.Errorf("%w Failed executing command %s on %v/%v", err, command, pod.Namespace, pod.Name)
	}
	return buf.String(), errBuf.String(), nil
}
