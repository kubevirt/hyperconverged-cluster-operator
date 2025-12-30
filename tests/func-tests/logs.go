package tests

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	hcoOperatorLabelValue = "hyperconverged-cluster-operator"
	hcoWebhookLabelValue  = "hyperconverged-cluster-webhook"
)

// LogCaptureOptions controls how pod logs are collected for debugging.
type LogCaptureOptions struct {
	// Since limits logs to those generated after the given time.
	Since *time.Time
	// IncludePrevious also dumps logs from the previous container instance.
	IncludePrevious bool
}

// DumpHCOPodLogs prints HCO operator and webhook pod logs to the Ginkgo output.
// This is intended for temporary debugging when tests trigger pod restarts.
func DumpHCOPodLogs(ctx context.Context, stage string, options LogCaptureOptions) {
	ginkgo.GinkgoHelper()

	if InstallNamespace == "" {
		ginkgo.GinkgoLogr.Info("Skipping HCO log capture: install namespace is empty", "stage", stage)
		return
	}

	cli := GetK8sClientSet()
	dumpPodsLogs(ctx, cli, hcoOperatorLabelValue, stage, options)
	dumpPodsLogs(ctx, cli, hcoWebhookLabelValue, stage, options)
}

func dumpPodsLogs(ctx context.Context, cli *kubernetes.Clientset, labelValue, stage string, options LogCaptureOptions) {
	ginkgo.GinkgoHelper()

	selector := fmt.Sprintf("name=%s", labelValue)
	pods, err := cli.CoreV1().Pods(InstallNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "Failed to list pods for log capture", "stage", stage, "selector", selector)
		return
	}
	if len(pods.Items) == 0 {
		ginkgo.GinkgoLogr.Info("No pods found for log capture", "stage", stage, "selector", selector)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			streamLogs(ctx, cli, pod, container.Name, stage, options)
		}
	}
}

func streamLogs(ctx context.Context, cli *kubernetes.Clientset, pod v1.Pod, containerName, stage string, options LogCaptureOptions) {
	ginkgo.GinkgoHelper()

	logOpts := &v1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
	}
	if options.Since != nil {
		logOpts.SinceTime = &metav1.Time{Time: *options.Since}
	}

	writeLogStream(ctx, cli, pod, containerName, stage, logOpts, false)
	if options.IncludePrevious {
		previousOpts := *logOpts
		previousOpts.Previous = true
		writeLogStream(ctx, cli, pod, containerName, stage, &previousOpts, true)
	}
}

func writeLogStream(ctx context.Context, cli *kubernetes.Clientset, pod v1.Pod, containerName, stage string, logOpts *v1.PodLogOptions, previous bool) {
	stream, err := cli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts).Stream(ctx)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "Failed to stream pod logs", "stage", stage, "pod", pod.Name, "container", containerName, "previous", previous)
		return
	}
	defer func() {
		if err := stream.Close(); err != nil {
			ginkgo.GinkgoLogr.Error(err, "Failed to close pod log stream", "stage", stage, "pod", pod.Name, "container", containerName, "previous", previous)
		}
	}()

	fmt.Fprintf(ginkgo.GinkgoWriter, "\n===== HCO logs (%s) pod=%s container=%s previous=%t =====\n", stage, pod.Name, containerName, previous)
	if _, err := io.Copy(ginkgo.GinkgoWriter, stream); err != nil {
		ginkgo.GinkgoLogr.Error(err, "Failed to copy pod logs", "stage", stage, "pod", pod.Name, "container", containerName, "previous", previous)
		return
	}
	fmt.Fprintln(ginkgo.GinkgoWriter, "\n===== HCO logs end =====")
}
