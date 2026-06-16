package tests

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega" //nolint dot-imports
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	KubeVirtStorageClassLocal string
	InstallNamespace          string
	cdiNS                     string
)

// labels
const (
	SingleNodeLabel             = "SINGLE_NODE_ONLY"
	HighlyAvailableClusterLabel = "HIGHLY_AVAILABLE_CLUSTER"
	DestructiveLabel            = "DESTRUCTIVE"
	OpenshiftLabel              = "OpenShift"
	DeprecatedArchLabel         = "DEPRECATED_ARCH"

	TestNamespace = "hco-test-default"
)

func init() {
	flag.StringVar(&KubeVirtStorageClassLocal, "storage-class-local", "local", "Storage provider to use for tests which want local storage")
	flag.StringVar(&InstallNamespace, "installed-namespace", "", "Set the namespace KubeVirt is installed in")
	flag.StringVar(&cdiNS, "cdi-namespace", "", "ignored")
}

func FlagParse() {
	flag.Parse()
}

func BeforeEach(ctx context.Context) {
	cli := GetK8sClientSet().RESTClient()

	deleteAllResources(ctx, cli, "virtualmachines")
	deleteAllResources(ctx, cli, "virtualmachineinstances")
	deleteAllResources(ctx, cli, "persistentvolumeclaims")
}

func FailIfNotOpenShift(ctx context.Context, cli client.Client, testName string) {
	isOpenShift := false
	Eventually(func(ctx context.Context) error {
		var err error
		isOpenShift, err = IsOpenShift(ctx, cli)
		return err
	}).WithTimeout(10*time.Second).WithPolling(time.Second).WithContext(ctx).Should(Succeed(), "failed to check if running on an openshift cluster")

	ExpectWithOffset(1, isOpenShift).To(BeTrue(), `the %q test must run on openshift cluster. Use the "!%s" label filter in order to skip this test`, testName, OpenshiftLabel)
}

func FailIfSingleNodeCluster(singleWorkerCluster bool) {
	ExpectWithOffset(1, singleWorkerCluster).To(BeFalse(), `this test requires a highly available cluster; use the "!%s" label filter to skip this test`, HighlyAvailableClusterLabel)
}

func FailIfHighAvailableCluster(singleWorkerCluster bool) {
	ExpectWithOffset(1, singleWorkerCluster).To(BeTrue(), `this test requires a single worker cluster; use the "!%s" label filter to skip this test`, SingleNodeLabel)
}

type cacheIsOpenShift struct {
	isOpenShift bool
	hasSet      bool
	lock        sync.Mutex
}

func (c *cacheIsOpenShift) IsOpenShift(ctx context.Context, cli client.Client) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.hasSet {
		return c.isOpenShift, nil
	}

	err := openshiftconfigv1.Install(cli.Scheme())
	if err != nil {
		panic("can't register scheme; " + err.Error())
	}

	clusterVersion := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}

	err = cli.Get(ctx, client.ObjectKeyFromObject(clusterVersion), clusterVersion)
	if err == nil {
		c.isOpenShift = true
		c.hasSet = true
		return c.isOpenShift, nil
	}

	discoveryErr := &discovery.ErrGroupDiscoveryFailed{}
	if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) || errors.As(err, &discoveryErr) {
		c.isOpenShift = false
		c.hasSet = true
		return c.isOpenShift, nil
	}

	return false, err
}

var isOpenShiftCache cacheIsOpenShift

func IsOpenShift(ctx context.Context, cli client.Client) (bool, error) {
	return isOpenShiftCache.IsOpenShift(ctx, cli)
}

func deleteAllResources(ctx context.Context, restClient rest.Interface, resourceName string) {
	Eventually(func() bool {
		err := restClient.Delete().Namespace(TestNamespace).Resource(resourceName).Do(ctx).Error()
		return err == nil || apierrors.IsNotFound(err)
	}).WithTimeout(time.Minute).
		WithPolling(time.Second).
		Should(BeTrue())
}

const (
	hcoRolloutTimeout = 5 * time.Minute
	hcoRolloutPolling = 5 * time.Second
)

// WaitForHCOOperatorRollout waits until the HCO operator deployment is ready.
func WaitForHCOOperatorRollout(ctx context.Context) {
	ginkgo.GinkgoHelper()

	cli := GetK8sClientSet()

	deployments, err := cli.AppsV1().Deployments(InstallNamespace).List(ctx, metav1.ListOptions{LabelSelector: "name=hyperconverged-cluster-operator"})
	Expect(err).ToNot(HaveOccurred())
	Expect(deployments.Items).To(HaveLen(1))
	deployName := deployments.Items[0].Name

	Eventually(func(g Gomega, gctx context.Context) {
		deployment, err := cli.AppsV1().Deployments(InstallNamespace).Get(gctx, deployName, metav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		desired := ptr.Deref(deployment.Spec.Replicas, 1)

		g.Expect(deployment.Status.ObservedGeneration).To(BeNumerically(">=", deployment.Generation))
		g.Expect(deployment.Status.UpdatedReplicas).To(BeNumerically(">=", desired))
		g.Expect(deployment.Status.ReadyReplicas).To(BeNumerically(">=", desired))
		g.Expect(deployment.Status.AvailableReplicas).To(BeNumerically(">=", desired))
	}).WithTimeout(hcoRolloutTimeout).WithPolling(hcoRolloutPolling).WithContext(ctx).Should(Succeed())
}

func ExecuteCommandOnPod(ctx context.Context, k8scli *kubernetes.Clientset, pod *corev1.Pod, command string, container ...string) (string, string, error) {
	cmd := []string{"/bin/sh", "-c", command}
	return executeCommandOnPod(ctx, k8scli, pod, cmd, true, container...)
}

func ExecuteUnwrappedCommandOnPod(ctx context.Context, k8scli *kubernetes.Clientset, pod *corev1.Pod, command []string, container ...string) (string, string, error) {
	return executeCommandOnPod(ctx, k8scli, pod, command, false, container...)
}

func executeCommandOnPod(ctx context.Context, k8scli *kubernetes.Clientset, pod *corev1.Pod, command []string, tty bool, container ...string) (string, string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	var cntr string
	if len(container) > 0 {
		cntr = container[0]
	}

	request := k8scli.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       tty,
			Container: cntr,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(GetClientConfig(), "POST", request.URL())
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
