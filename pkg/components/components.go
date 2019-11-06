package components

import (
	"fmt"

	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeployment(repository string, tag string, imagePullPolicy string, ConversionContainer string, VMWareContainer string) *appsv1.Deployment {
	hco_name := "hyperconverged-cluster-operator"
	registry_name := repository + "/kubevirt/" + hco_name
	image := fmt.Sprintf("%s:%s", registry_name, tag)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: hco_name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": hco_name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": hco_name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: hco_name,
					Containers: []corev1.Container{
						{
							Name:            hco_name,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							// TODO: command being name is artifact of operator-sdk usage
							Command: []string{hco_name},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"stat",
											"/tmp/operator-sdk-ready",
										},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								FailureThreshold:    1,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "OPERATOR_IMAGE",
									Value: image,
								},
								{
									Name:  "OPERATOR_NAME",
									Value: hco_name,
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "WATCH_NAMESPACE",
									Value: "",
								},
								{
									Name:  "CONVERSION_CONTAINER",
									Value: ConversionContainer,
								},
								{
									Name:  "VMWARE_CONTAINER",
									Value: VMWareContainer,
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func GetClusterRole() *rbacv1.ClusterRole {
	name := "hyperconverged-cluster-operator"
	role := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"name": name,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"hco.kubevirt.io",
				},
				Resources: []string{
					"*",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"kubevirt.io",
				},
				Resources: []string{
					"*",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"cdi.kubevirt.io",
				},
				Resources: []string{
					"*",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"networkaddonsoperator.network.kubevirt.io",
				},
				Resources: []string{
					"*",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"machineremediation.kubevirt.io",
				},
				Resources: []string{
					"machineremediationoperators",
					"machineremediationoperators/status",
				},
				Verbs: []string{
					rbacv1.VerbAll,
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
					"services",
					"services/finalizers",
					"endpoints",
					"persistentvolumeclaims",
					"events",
					"configmaps",
					"secrets",
					"serviceaccounts",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"apps",
				},
				Resources: []string{
					"deployments",
					"deployments/finalizers",
					"daemonsets",
					"replicasets",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"delete",
					"update",
				},
			},
			{
				APIGroups: []string{
					"batch",
				},
				Resources: []string{
					"jobs",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"delete",
				},
			},
			{
				APIGroups: []string{
					"rbac.authorization.k8s.io",
				},
				Resources: []string{
					"clusterroles",
					"clusterrolebindings",
					"roles",
					"rolebindings",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"delete",
				},
			},
			{
				APIGroups: []string{
					"apiextensions.k8s.io",
				},
				Resources: []string{
					"customresourcedefinitions",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
					"create",
					"delete",
					"patch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				Resources: []string{
					"securitycontextconstraints",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				Resources: []string{
					"securitycontextconstraints",
				},
				ResourceNames: []string{
					"privileged",
				},
				Verbs: []string{
					"get",
					"patch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"monitoring.coreos.com",
				},
				Resources: []string{
					"servicemonitors",
				},
				Verbs: []string{
					"get",
					"create",
				},
			},
		},
	}
	return role
}

func GetCrd() *extv1beta1.CustomResourceDefinition {
	crd := &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hyperconvergeds.hco.kubevirt.io",
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "hco.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Namespaced",

			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Plural:     "hyperconvergeds",
				Singular:   "hyperconverged",
				Kind:       "HyperConverged",
				ShortNames: []string{"hco", "hcos"},
			},

			AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
				{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
				{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
			},

			Subresources: &extv1beta1.CustomResourceSubresources{
				Status: &extv1beta1.CustomResourceSubresourceStatus{},
			},
		},
	}
	return crd
}

func GetCR() *hcov1alpha1.HyperConverged {
	return &hcov1alpha1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "hco.kubevirt.io/v1alpha1",
			Kind:       "HyperConverged",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hyperconverged-cluster",
		},
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
