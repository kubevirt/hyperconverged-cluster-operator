package virtimport

import (
	"errors"
	"fmt"
	"slices"

	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

// RemoveStaleObjects removes the virt-import objects that are no longer in the bundle.
func RemoveStaleObjects(req *common.HcoRequest, cli client.Client) error {
	b, err := virtimport.GetBundle()
	if err != nil {
		req.Logger.Error(err, "can't remove stale virt-import objects")
		return nil
	}

	type objectList struct {
		list  client.ObjectList
		names []string
	}
	lists := []objectList{
		{&corev1.ServiceAccountList{}, names(b.ServiceAccounts)},
		{&rbacv1.RoleList{}, names(b.Roles)},
		{&rbacv1.RoleBindingList{}, names(b.RoleBindings)},
		{&rbacv1.ClusterRoleList{}, names(b.ClusterRoles)},
		{&rbacv1.ClusterRoleBindingList{}, names(b.ClusterRoleBindings)},
		{&admissionregistrationv1.ValidatingAdmissionPolicyList{}, names(b.ValidatingAdmissionPolicies)},
		{&admissionregistrationv1.ValidatingAdmissionPolicyBindingList{}, names(b.ValidatingAdmissionPolicyBindings)},
		{&appsv1.DeploymentList{}, []string{b.Deployment.Name}},
		{&networkingv1.NetworkPolicyList{}, append(names(b.NetworkPolicies), newNetworkPolicy(b.Deployment).Name)},
		{&corev1.SecretList{}, tokenSecretNames(b)},
		{&corev1.ServiceList{}, []string{b.MetricsService.Name}},
	}
	ci := hcoutil.GetClusterInfo()
	if ci.IsMonitoringAvailable() {
		lists = append(lists, objectList{&monitoringv1.ServiceMonitorList{}, []string{b.ServiceMonitor.Name}})
	}
	if !ci.IsOpenshift() {
		certs := &unstructured.UnstructuredList{}
		certs.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "CertificateList"})
		lists = append(lists, objectList{certs, []string{b.MetricsCertSecretName()}})
	}

	var errs []error
	for _, l := range lists {
		errs = append(errs, removeStaleObjectsOfList(req, cli, l.list, l.names))
	}
	return errors.Join(errs...)
}

func removeStaleObjectsOfList(req *common.HcoRequest, cli client.Client, list client.ObjectList, required []string) error {
	if err := cli.List(req.Ctx, list, client.MatchingLabels(selectorLabels())); err != nil {
		return fmt.Errorf("failed to list %T: %w", list, err)
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	operatorNamespace := hcoutil.GetOperatorNamespaceFromEnv()
	var errs []error
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok || slices.Contains(required, obj.GetName()) {
			continue
		}
		if ns := obj.GetNamespace(); ns != "" && ns != operatorNamespace {
			continue
		}

		req.Logger.Info("Removing a virt-import object that is no longer in the bundle", "type", fmt.Sprintf("%T", obj), "name", obj.GetName())
		if _, err = hcoutil.EnsureDeleted(req.Ctx, cli, obj, req.Instance.Name, req.Logger, false, false, true); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, removeRelatedObject(req, cli, obj))
	}
	return errors.Join(errs...)
}

func removeRelatedObject(req *common.HcoRequest, cli client.Client, obj client.Object) error {
	ref, err := reference.GetReference(cli.Scheme(), obj)
	if err != nil {
		return err
	}
	found, err := objectreferencesv1.FindObjectReference(req.Instance.Status.RelatedObjects, *ref)
	if err != nil || found == nil {
		return err
	}
	if err = objectreferencesv1.RemoveObjectReference(&req.Instance.Status.RelatedObjects, *ref); err != nil {
		return err
	}
	req.StatusDirty = true
	return nil
}

func names[T client.Object](objs []T) []string {
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		out = append(out, o.GetName())
	}
	return out
}

// tokenSecretNames returns the bundle Secrets HCO deploys, none on OpenShift.
func tokenSecretNames(b *virtimport.Bundle) []string {
	if hcoutil.GetClusterInfo().IsOpenshift() {
		return nil
	}
	return names(b.Secrets)
}
