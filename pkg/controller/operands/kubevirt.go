package operands

import (
	"fmt"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"os"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/generated/network-attachment-definition-client/clientset/versioned/scheme"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	kubevirtDefaultNetworkInterfaceValue = "masquerade"
)

// ===== KubeVirt =====

type kubeVirtHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newKubeVirtHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *kubeVirtHandler {
	return &kubeVirtHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (kv kubeVirtHandler) ensure(req *common.HcoRequest) *EnsureResult {

	virt := req.Instance.NewKubeVirt()
	res := NewEnsureResult(virt)

	err := kv.ensureKubeVirtConfig(req)
	if err != nil {
		return res.Error(err)
	}

	if err = controllerutil.SetControllerReference(req.Instance, virt, kv.scheme); err != nil {
		return res.Error(err)
	}

	key, err := client.ObjectKeyFromObject(virt)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt")
	}

	res.SetName(key.Name)
	found := &kubevirtv1.KubeVirt{}
	err = kv.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating kubevirt")
			err = kv.client.Create(req.Ctx, virt)
			if err == nil {
				kv.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(virt), key.Name)
				return res.SetCreated().SetName(virt.Name)
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("KubeVirt already exists", "KubeVirt.Namespace", found.Namespace, "KubeVirt.Name", found.Name)

	if !reflect.DeepEqual(found.Spec, virt.Spec) {
		if found.Spec.UninstallStrategy == "" {
			req.Logger.Info("Updating UninstallStrategy on existing KubeVirt to its default value")
			found.Spec.UninstallStrategy = virt.Spec.UninstallStrategy
		}
		err = kv.client.Update(req.Ctx, found)
		if err != nil {
			return res.Error(err)
		}
		kv.eventEmitter.EmitUpdatedEvent(req.Instance, getResourceType(virt), key.Name)
		return res.SetUpdated()
	}

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(kv.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	// Handle KubeVirt resource conditions
	isReady := handleComponentConditions(req, "KubeVirt", translateKubeVirtConds(found.Status.Conditions))

	upgradeDone := req.ComponentUpgradeInProgress && isReady && checkComponentVersion(hcoutil.KubevirtVersionEnvV, found.Status.ObservedKubeVirtVersion)

	return res.SetUpgradeDone(upgradeDone)
}

func (kv kubeVirtHandler) ensureKubeVirtConfig(req *common.HcoRequest) error {
	kubevirtConfig := NewKubeVirtConfigForCR(req.Instance, req.Namespace)
	err := controllerutil.SetControllerReference(req.Instance, kubevirtConfig, kv.scheme)
	if err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtConfig)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for kubevirt config")
	}

	found := &corev1.ConfigMap{}
	err = kv.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating kubevirt config")
			err = kv.client.Create(req.Ctx, kubevirtConfig)
			if err == nil {
				kv.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kubevirtConfig), key.Name)
				return nil
			}
		}
		return err
	}

	req.Logger.Info("KubeVirt config already exists", "KubeVirtConfig.Namespace", found.Namespace, "KubeVirtConfig.Name", found.Name)
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(kv.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	if req.UpgradeMode {
		// only virtconfig.SmbiosConfigKey, virtconfig.MachineTypeKey, virtconfig.SELinuxLauncherTypeKey and
		// virtconfig.UseEmulationKey are going to be manipulated and only on HCO upgrades
		for _, k := range []string{
			virtconfig.SmbiosConfigKey,
			virtconfig.MachineTypeKey,
			virtconfig.SELinuxLauncherTypeKey,
			virtconfig.UseEmulationKey,
		} {
			if found.Data[k] != kubevirtConfig.Data[k] {
				req.Logger.Info(fmt.Sprintf("Updating %s on existing KubeVirt config", k))
				found.Data[k] = kubevirtConfig.Data[k]
				err = kv.client.Update(req.Ctx, found)
				if err != nil {
					req.Logger.Error(err, fmt.Sprintf("Failed updating %s on an existing kubevirt config", k))
					return err
				}
			}
		}
	}

	return nil
}

func NewKubeVirtConfigForCR(cr *hcov1beta1.HyperConverged, namespace string) *corev1.ConfigMap {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-config",
			Labels:    labels,
			Namespace: namespace,
		},
		// only virtconfig.SmbiosConfigKey, virtconfig.MachineTypeKey, virtconfig.SELinuxLauncherTypeKey and
		// virtconfig.UseEmulationKey are going to be manipulated and only on HCO upgrades
		Data: map[string]string{
			virtconfig.FeatureGatesKey:        "DataVolumes,SRIOV,LiveMigration,CPUManager,CPUNodeDiscovery,Sidecar",
			virtconfig.MigrationsConfigKey:    `{"nodeDrainTaintKey" : "node.kubernetes.io/unschedulable"}`,
			virtconfig.SELinuxLauncherTypeKey: "virt_launcher.process",
			virtconfig.NetworkInterfaceKey:    kubevirtDefaultNetworkInterfaceValue,
		},
	}
	val, ok := os.LookupEnv("SMBIOS")
	if ok && val != "" {
		cm.Data[virtconfig.SmbiosConfigKey] = val
	}
	val, ok = os.LookupEnv("MACHINETYPE")
	if ok && val != "" {
		cm.Data[virtconfig.MachineTypeKey] = val
	}
	val, ok = os.LookupEnv("KVM_EMULATION")
	if ok && val != "" {
		cm.Data[virtconfig.UseEmulationKey] = val
	}
	return cm
}

func translateKubeVirtConds(orig []kubevirtv1.KubeVirtCondition) []conditionsv1.Condition {
	translated := make([]conditionsv1.Condition, len(orig))

	for i, origCond := range orig {
		translated[i] = conditionsv1.Condition{
			Type:    conditionsv1.ConditionType(origCond.Type),
			Status:  origCond.Status,
			Reason:  origCond.Reason,
			Message: origCond.Message,
		}
	}

	return translated
}

// ===== PriorityClass =====
type kvPriorityClassHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func NewKvPriorityClassHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *kvPriorityClassHandler {
	return &kvPriorityClassHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h kvPriorityClassHandler) ensure(req *common.HcoRequest) *EnsureResult {
	req.Logger.Info("Reconciling KubeVirt PriorityClass")
	pc := req.Instance.NewKubeVirtPriorityClass()
	res := NewEnsureResult(pc)
	key, err := client.ObjectKeyFromObject(pc)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt PriorityClass")
		return res.Error(err)
	}

	res.SetName(key.Name)
	found := &schedulingv1.PriorityClass{}
	err = h.client.Get(req.Ctx, key, found)

	typeForEvent := getResourceType(pc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create the new object
			err = h.client.Create(req.Ctx, pc, &client.CreateOptions{})
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, typeForEvent, key.Name)
				return res.SetCreated()
			}
		}

		return res.Error(err)
	}

	// at this point we found the object in the cache and we check if something was changed
	if pc.Name == found.Name && pc.Value == found.Value && pc.Description == found.Description {
		req.Logger.Info("KubeVirt PriorityClass already exists", "PriorityClass.Name", pc.Name)
		objectRef, err := reference.GetReference(scheme.Scheme, found)
		if err != nil {
			req.Logger.Error(err, "failed getting object reference for found object")
			return res.Error(err)
		}
		objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

		return res.SetUpgradeDone(req.ComponentUpgradeInProgress)
	}

	// something was changed but since we can't patch a priority class object, we remove it
	err = h.client.Delete(req.Ctx, found, &client.DeleteOptions{})
	if err != nil {
		return res.Error(err)
	}
	h.eventEmitter.EmitDeletedEvent(req.Instance, typeForEvent, key.Name)

	// create the new object
	err = h.client.Create(req.Ctx, pc, &client.CreateOptions{})
	if err != nil {
		return res.Error(err)
	}

	h.eventEmitter.EmitCreatedEvent(req.Instance, typeForEvent, key.Name)
	return res.SetUpdated()
}
