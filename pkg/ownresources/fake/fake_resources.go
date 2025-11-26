package fakeownreferences

import (
	"context"

	"github.com/go-logr/logr"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
)

const (
	RSName    = "hco-operator"
	Namespace = "kubevirt-hyperconverged"
	podName   = RSName + "-12345"
)

func fakeInit(_ context.Context, _ client.Reader, _ logr.Logger) { /* no implementation */ }

func fakeGetPod() *corev1.Pod {
	return pod.DeepCopy()
}

func GetFakeDeploymentRef() metav1.OwnerReference {
	return *deploymentRef.DeepCopy()
}

func GetCSV() *csvv1alpha1.ClusterServiceVersion {
	return csv.DeepCopy()
}

var ( // own resources
	csv = &csvv1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterServiceVersion",
			APIVersion: "operators.coreos.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      RSName,
			Namespace: Namespace,
			Annotations: map[string]string{
				components.DisableOperandDeletionAnnotation: "true",
			},
		},
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
