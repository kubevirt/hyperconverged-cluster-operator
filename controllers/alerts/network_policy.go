package alerts

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	policyName = "hco-allow-egress-to-alert-manager"
)

type AlertManagerNetworkPolicyReconciler struct {
	thePolicy *networkingv1.NetworkPolicy
}

// newAlertRuleReconciler creates new AlertRuleReconciler instance and returns a pointer to it.
func newAlertManagerNetworkPolicyReconciler(namespace string, owner metav1.OwnerReference, ci hcoutil.ClusterInfo) *AlertManagerNetworkPolicyReconciler {
	return &AlertManagerNetworkPolicyReconciler{
		thePolicy: newAlertManagerNetworkPolicy(namespace, owner, ci),
	}
}

func newAlertManagerNetworkPolicy(namespace string, owner metav1.OwnerReference, ci hcoutil.ClusterInfo) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            policyName,
			Namespace:       namespace,
			Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
			OwnerReferences: []metav1.OwnerReference{*owner.DeepCopy()},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": operatorName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": getMonitoringNamespace(ci),
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"alertmanager":                "main",
									"app.kubernetes.io/component": "alert-router",
									"app.kubernetes.io/instance":  "main",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *AlertManagerNetworkPolicyReconciler) Kind() string {
	return "NetworkPolicy"
}

func (r *AlertManagerNetworkPolicyReconciler) ResourceName() string {
	return policyName
}

func (r *AlertManagerNetworkPolicyReconciler) GetFullResource() client.Object {
	return r.thePolicy.DeepCopy()
}

func (r *AlertManagerNetworkPolicyReconciler) EmptyObject() client.Object {
	return &networkingv1.NetworkPolicy{}
}

func (r *AlertManagerNetworkPolicyReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, existing client.Object, logger logr.Logger) (client.Object, bool, error) {
	needUpdate := false
	np := existing.(*networkingv1.NetworkPolicy)
	if !reflect.DeepEqual(r.thePolicy.Spec, np.Spec) {
		needUpdate = true
		r.thePolicy.Spec.DeepCopyInto(&np.Spec)
	}

	needUpdate = updateCommonDetails(&r.thePolicy.ObjectMeta, &np.ObjectMeta) || needUpdate

	if needUpdate {
		logger.Info("updating the PrometheusRule")
		err := cl.Update(ctx, np)
		if err != nil {
			logger.Error(err, "failed to update the PrometheusRule")
			return nil, false, err
		}
		logger.Info("successfully updated the PrometheusRule")
	}

	return np, needUpdate, nil
}
