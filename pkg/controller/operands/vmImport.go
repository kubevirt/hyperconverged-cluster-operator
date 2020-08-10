package operands

import (
	"errors"
	"os"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	vmimportv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type vmImportHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newVMImportHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *vmImportHandler {
	return &vmImportHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h vmImportHandler) ensure(req *common.HcoRequest) *EnsureResult {
	vmImport := NewVMImportForCR(req.Instance, req.Namespace)
	res := NewEnsureResult(vmImport)
	err := controllerutil.SetControllerReference(req.Instance, vmImport, h.scheme)
	if err != nil {
		return res.Error(err)
	}

	err = h.ensureIMSConfig(req)
	if err != nil {
		return res.Error(err)
	}

	key := client.ObjectKey{Namespace: "", Name: vmImport.GetName()}
	res.SetName(vmImport.GetName())

	found := &vmimportv1alpha1.VMImportConfig{}
	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating vm import")
			err = h.client.Create(req.Ctx, vmImport)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(vmImport), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("VM import exists", "vmImport.Namespace", found.Namespace, "vmImport.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	// Handle VMimport resource conditions
	isReady := handleComponentConditions(req, "VMimport", found.Status.Conditions)

	upgradeDone := req.ComponentUpgradeInProgress && isReady && checkComponentVersion(hcoutil.VMImportEnvV, found.Status.ObservedVersion)
	return res.SetUpgradeDone(upgradeDone)
}

func NewVMImportForCR(cr *hcov1beta1.HyperConverged, namespace string) *vmimportv1alpha1.VMImportConfig {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}

	return &vmimportv1alpha1.VMImportConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vmimport-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (h vmImportHandler) ensureIMSConfig(req *common.HcoRequest) error {
	conversionContainer, ok := os.LookupEnv("CONVERSION_CONTAINER")
	if !ok {
		return errors.New("ims-conversion-container not specified")
	}

	vmwareContainer, ok := os.LookupEnv("VMWARE_CONTAINER")
	if !ok {
		return errors.New("ims-vmware-container not specified")
	}

	imsConfig := NewIMSConfigForCR(req.Instance, req.Namespace, conversionContainer, vmwareContainer)
	err := controllerutil.SetControllerReference(req.Instance, imsConfig, h.scheme)
	if err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(imsConfig)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for IMS Configmap")
	}

	found := &corev1.ConfigMap{}

	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating IMS Configmap")
			err = h.client.Create(req.Ctx, imsConfig)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(imsConfig), key.Name)
				return nil
			}
		}
		return err
	}

	req.Logger.Info("IMS Configmap already exists", "imsConfigMap.Namespace", found.Namespace, "imsConfigMap.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	return nil
}

func NewIMSConfigForCR(cr *hcov1beta1.HyperConverged, namespace, conversionContainer, vmwareContainer string) *corev1.ConfigMap {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "v2v-vmware",
			Labels:    labels,
			Namespace: namespace,
		},
		Data: map[string]string{
			"v2v-conversion-image":              conversionContainer,
			"kubevirt-vmware-image":             vmwareContainer,
			"kubevirt-vmware-image-pull-policy": "IfNotPresent",
		},
	}
}
