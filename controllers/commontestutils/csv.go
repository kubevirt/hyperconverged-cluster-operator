package commontestutils

import (
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func GetCSV() *csvv1alpha1.ClusterServiceVersion {
	return &csvv1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterServiceVersion",
			APIVersion: "operators.coreos.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hco-operator",
			Namespace: Namespace,
			Annotations: map[string]string{
				hcoutil.DisableOperandDeletionAnnotation: "true",
			},
		},
	}

}
