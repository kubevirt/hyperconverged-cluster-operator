package operands

import (
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type cdiHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newCdiHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *cdiHandler {
	return &cdiHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (cdih cdiHandler) ensure(req *common.HcoRequest) *EnsureResult {
	cdi := req.Instance.NewCDI()
	res := NewEnsureResult(cdi)

	key, err := client.ObjectKeyFromObject(cdi)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for CDI")
	}

	res.SetName(key.Name)
	found := &cdiv1alpha1.CDI{}
	err = cdih.client.Get(req.Ctx, key, found)

	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating CDI")
			err = cdih.client.Create(req.Ctx, cdi)
			if err == nil {
				cdih.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(cdi), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("CDI already exists", "CDI.Namespace", found.Namespace, "CDI.Name", found.Name)

	err = cdih.ensureKubeVirtStorageConfig(req)
	if err != nil {
		return res.Error(err)
	}

	err = cdih.ensureKubeVirtStorageRole(req)
	if err != nil {
		return res.Error(err)
	}

	err = cdih.ensureKubeVirtStorageRoleBinding(req)
	if err != nil {
		return res.Error(err)
	}

	existingOwners := found.GetOwnerReferences()

	// Previous versions used to have HCO-operator (scope namespace)
	// as the owner of CDI (scope cluster).
	// It's not legal, so remove that.
	if len(existingOwners) > 0 {
		req.Logger.Info("CDI has owners, removing...")
		found.SetOwnerReferences([]metav1.OwnerReference{})
		err = cdih.client.Update(req.Ctx, found)
		if err != nil {
			req.Logger.Error(err, "Failed to remove CDI's previous owners")
		}
	}

	if !reflect.DeepEqual(found.Spec, cdi.Spec) {
		if found.Spec.UninstallStrategy == nil {
			req.Logger.Info("Updating UninstallStrategy on existing CDI to its default value")
			defaultUninstallStrategy := cdiv1alpha1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist
			found.Spec.UninstallStrategy = &defaultUninstallStrategy
		}
		err = cdih.client.Update(req.Ctx, found)
		if err != nil {
			return res.Error(err)
		}
		cdih.eventEmitter.EmitUpdatedEvent(req.Instance, getResourceType(cdi), key.Name)
		return res.SetUpdated()
	}

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(cdih.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	// Handle CDI resource conditions
	isReady := handleComponentConditions(req, "CDI", found.Status.Conditions)

	upgradeDone := req.ComponentUpgradeInProgress && isReady && checkComponentVersion(hcoutil.CdiVersionEnvV, found.Status.ObservedVersion)

	return res.SetUpgradeDone(upgradeDone)
}

func (cdih cdiHandler) ensureKubeVirtStorageConfig(req *common.HcoRequest) error {
	kubevirtStorageConfig := NewKubeVirtStorageConfigForCR(req.Instance, req.Namespace)
	if err := controllerutil.SetControllerReference(req.Instance, kubevirtStorageConfig, cdih.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtStorageConfig)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for kubevirt storage config")
	}

	found := &corev1.ConfigMap{}
	err = cdih.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating kubevirt storage config")
			err = cdih.client.Create(req.Ctx, kubevirtStorageConfig)
			if err == nil {
				cdih.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kubevirtStorageConfig), key.Name)
				return nil
			}
		}

		return err
	}

	req.Logger.Info("KubeVirt storage config already exists", "KubeVirtConfig.Namespace", found.Namespace, "KubeVirtConfig.Name", found.Name)
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(cdih.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	return nil
}

func (cdih cdiHandler) ensureKubeVirtStorageRole(req *common.HcoRequest) error {
	kubevirtStorageRole := NewKubeVirtStorageRoleForCR(req.Instance, req.Namespace)
	if err := controllerutil.SetControllerReference(req.Instance, kubevirtStorageRole, cdih.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtStorageRole)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for kubevirt storage role")
	}

	found := &rbacv1.Role{}
	err = cdih.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating kubevirt storage role")
			if err = cdih.client.Create(req.Ctx, kubevirtStorageRole); err == nil {
				cdih.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kubevirtStorageRole), key.Name)
				return nil
			}
		}

		return err
	}

	req.Logger.Info("KubeVirt storage role already exists", "KubeVirtConfig.Namespace", found.Namespace, "KubeVirtConfig.Name", found.Name)
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(cdih.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	return nil
}

func (cdih cdiHandler) ensureKubeVirtStorageRoleBinding(req *common.HcoRequest) error {
	kubevirtStorageRoleBinding := NewKubeVirtStorageRoleBindingForCR(req.Instance, req.Namespace)
	if err := controllerutil.SetControllerReference(req.Instance, kubevirtStorageRoleBinding, cdih.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtStorageRoleBinding)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for kubevirt storage rolebinding")
	}

	found := &rbacv1.RoleBinding{}
	err = cdih.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating kubevirt storage rolebinding")
			if err = cdih.client.Create(req.Ctx, kubevirtStorageRoleBinding); err == nil {
				cdih.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kubevirtStorageRoleBinding), key.Name)
				return nil
			}
		}

		return err
	}

	req.Logger.Info("KubeVirt storage rolebinding already exists", "KubeVirtConfig.Namespace", found.Namespace, "KubeVirtConfig.Name", found.Name)
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(cdih.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	return nil
}

func NewKubeVirtStorageConfigForCR(cr *hcov1beta1.HyperConverged, namespace string) *corev1.ConfigMap {
	localSC := "local-sc"
	if *(&cr.Spec.LocalStorageClassName) != "" {
		localSC = *(&cr.Spec.LocalStorageClassName)
	}

	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-storage-class-defaults",
			Labels:    labels,
			Namespace: namespace,
		},
		Data: map[string]string{
			"accessMode":            "ReadWriteOnce",
			"volumeMode":            "Filesystem",
			localSC + ".accessMode": "ReadWriteOnce",
			localSC + ".volumeMode": "Filesystem",
		},
	}
}

func NewKubeVirtStorageRoleBindingForCR(cr *hcov1beta1.HyperConverged, namespace string) *rbacv1.RoleBinding {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hco.kubevirt.io:config-reader",
			Labels:    labels,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "hco.kubevirt.io:config-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			},
		},
	}
}

func NewKubeVirtStorageRoleForCR(cr *hcov1beta1.HyperConverged, namespace string) *rbacv1.Role {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hco.kubevirt.io:config-reader",
			Labels:    labels,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"kubevirt-storage-class-defaults"},
				Verbs:         []string{"get", "watch", "list"},
			},
		},
	}
}
