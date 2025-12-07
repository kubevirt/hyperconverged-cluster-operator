package ownresources

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/go-logr/logr"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// OwnResources holds the running POd, Deployment and CSV, if exist
var (
	thePod        *corev1.Pod
	deploymentRef *metav1.OwnerReference
	csvRef        *corev1.ObjectReference

	initOnce = &sync.Once{}
)

// GetPod returns the running pod, or nil if not exists
func GetPod() *corev1.Pod {
	if thePod == nil {
		return nil
	}
	return thePod.DeepCopy()
}

// GetDeploymentRef returns the ObjectReference, pointing to the deployment that controls the running
// pod, or nil if not exists
func GetDeploymentRef() metav1.OwnerReference {
	if deploymentRef == nil {
		return metav1.OwnerReference{}
	}
	return *deploymentRef.DeepCopy()
}

// GetCSVRef returns the object reference of the CSV that defines the application, or nil if not exists
func GetCSVRef() *corev1.ObjectReference {
	if csvRef == nil {
		return nil
	}

	return csvRef.DeepCopy()
}

func Init(ctx context.Context, cl client.Reader, scheme *runtime.Scheme, logger logr.Logger) {
	initOnce.Do(doInit(ctx, cl, scheme, logger))
}

func doInit(ctx context.Context, cl client.Reader, scheme *runtime.Scheme, logger logr.Logger) func() {
	return func() {
		if !hcoutil.GetClusterInfo().IsRunningLocally() {
			var err error

			pod, err := getThePod(ctx, cl, logger)
			if err != nil {
				logger.Error(err, "Can't get self pod")
			}

			thePod = pod

			operatorNs := hcoutil.GetOperatorNamespaceFromEnv()
			deployment, err := getDeploymentFromPod(ctx, pod, cl, operatorNs, logger)
			if err != nil {
				logger.Error(err, "Can't get deployment")
				return
			}

			deploymentRef = buildOwnerReference(deployment)

			csvRef, err = getCSVFromDeployment(ctx, deployment, cl, scheme, logger)
			if err != nil {
				logger.Error(err, "Can't get CSV object reference")
			}

		} else {
			deployment := &appsv1.Deployment{}
			err := cl.Get(ctx, client.ObjectKey{
				Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
				Name:      "hyperconverged-cluster-operator",
			}, deployment)
			if err != nil {
				logger.Error(err, "Can't get deployment")
				return
			}

			thePod = nil
			deploymentRef = buildOwnerReference(deployment)
			csvRef = nil
		}
	}
}

func getThePod(ctx context.Context, c client.Reader, logger logr.Logger) (*corev1.Pod, error) {
	ci := hcoutil.GetClusterInfo()
	operatorNs := hcoutil.GetOperatorNamespaceFromEnv()

	// This is taken from k8sutil.getPod. This method only receives client. But the client is not always ready. We'll
	// use --- instead
	if ci.IsRunningLocally() {
		return nil, nil
	}
	podName := os.Getenv(hcoutil.PodNameEnvVar)
	if podName == "" {
		return nil, fmt.Errorf("required env %q not set, please configure downward API", hcoutil.PodNameEnvVar)
	}

	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: operatorNs, Name: podName}
	err := c.Get(ctx, key, pod)
	if err != nil {
		logger.Error(err, "Failed to get Pod", "Pod.Namespace", operatorNs, "Pod.Name", podName)
		return nil, err
	}

	// client.Get() clears the APIVersion and Kind,
	// so we need to set them before returning the object.
	pod.APIVersion = "v1"
	pod.Kind = "Pod"

	logger.Info("Found Pod", "Pod.Namespace", operatorNs, "Pod.Name", pod.Name)

	return pod, nil
}

func getDeploymentFromPod(ctx context.Context, pod *corev1.Pod, c client.Reader, operatorNs string, logger logr.Logger) (*appsv1.Deployment, error) {
	if pod == nil {
		return nil, nil
	}
	rsReference := metav1.GetControllerOf(pod)
	if rsReference == nil || rsReference.Kind != "ReplicaSet" {
		err := errors.New("failed getting HCO replicaSet reference")
		logger.Error(err, "Failed getting HCO replicaSet reference")
		return nil, err
	}
	rs := &appsv1.ReplicaSet{}
	err := c.Get(context.TODO(), client.ObjectKey{
		Namespace: operatorNs,
		Name:      rsReference.Name,
	}, rs)
	if err != nil {
		logger.Error(err, "Failed to get HCO ReplicaSet")
		return nil, err
	}

	dReference := metav1.GetControllerOf(rs)
	if dReference == nil || dReference.Kind != "Deployment" {
		err = errors.New("failed getting HCO deployment reference")
		logger.Error(err, "Failed getting HCO deployment reference")
		return nil, err
	}
	deployment := &appsv1.Deployment{}
	err = c.Get(ctx, client.ObjectKey{
		Namespace: operatorNs,
		Name:      dReference.Name,
	}, deployment)
	if err != nil {
		logger.Error(err, "Failed to get HCO Deployment")
		return nil, err
	}

	return deployment, nil
}

func getCSVFromDeployment(ctx context.Context, deploy *appsv1.Deployment, c client.Reader, scheme *runtime.Scheme, logger logr.Logger) (*corev1.ObjectReference, error) {
	idx := slices.IndexFunc(deploy.GetOwnerReferences(), func(owner metav1.OwnerReference) bool {
		return owner.Kind == csvv1alpha1.ClusterServiceVersionKind
	})

	if idx < 0 {
		err := fmt.Errorf("can't find CSV owner reference in deployment %q", deploy.Name)
		return nil, err
	}

	csvReference := deploy.GetOwnerReferences()[idx]

	theCSV := &csvv1alpha1.ClusterServiceVersion{}
	err := c.Get(ctx, client.ObjectKey{
		Namespace: deploy.Namespace,
		Name:      csvReference.Name,
	}, theCSV)

	if err != nil {
		logger.Error(err, "Failed to get HCO CSV")
		return nil, err
	}

	return reference.GetReference(scheme, theCSV)
}

func buildOwnerReference(ownerDeployment *appsv1.Deployment) *metav1.OwnerReference {
	if ownerDeployment == nil {
		return nil
	}

	return &metav1.OwnerReference{
		APIVersion:         appsv1.SchemeGroupVersion.String(),
		Kind:               "Deployment",
		Name:               ownerDeployment.GetName(),
		UID:                ownerDeployment.GetUID(),
		BlockOwnerDeletion: ptr.To(false),
		Controller:         ptr.To(false),
	}
}
