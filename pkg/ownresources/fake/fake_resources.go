package fakeownresources

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RSName    = "hco-operator"
	Namespace = "kubevirt-hyperconverged"
	podName   = RSName + "-12345"
)

func fakeInit(_ context.Context, _ client.Reader, _ *runtime.Scheme, _ logr.Logger) { /* no implementation */
}

func fakeGetPod() *corev1.Pod {
	return pod.DeepCopy()
}

func GetFakeDeploymentRef() metav1.OwnerReference {
	return *deploymentRef.DeepCopy()
}

func GetCSVRef() *corev1.ObjectReference {
	return csvRef.DeepCopy()
}

var ( // own resources

	csvRef = &corev1.ObjectReference{
		Kind:            "ClusterServiceVersion",
		Namespace:       Namespace,
		Name:            RSName,
		UID:             "0266392e-0153-519e-ce97-4eb5b90020e8",
		APIVersion:      "operators.coreos.com/v1alpha1",
		ResourceVersion: "",
	}

	deploymentRef = metav1.OwnerReference{
		APIVersion:         appsv1.SchemeGroupVersion.String(),
		Kind:               "Deployment",
		Name:               RSName,
		UID:                "0155282d-9042-408d-bd86-3da4a89919d7",
		BlockOwnerDeletion: ptr.To(true),
		Controller:         ptr.To(true),
	}

	pod = &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       RSName,
					Controller: ptr.To(true),
				},
			},
		},
	}
)
