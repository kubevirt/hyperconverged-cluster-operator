package operands

import (
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1 "github.com/openshift/api/security/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newSecurityContextConstraintsFunc func(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints

func NewSecurityContextConstraintsHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newSecurityContextConstraintsFunc) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "SecurityContextConstraints", &securityContextConstraintsHooks{newCrFunc: newCrFunc}, false)
}

type securityContextConstraintsHooks struct {
	newCrFunc newSecurityContextConstraintsFunc
}

func (h securityContextConstraintsHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (securityContextConstraintsHooks) GetEmptyCr() client.Object {
	return &securityv1.SecurityContextConstraints{}
}

func (securityContextConstraintsHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */
}

func (securityContextConstraintsHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateSecurityContextConstraints(req, Client, exists, required)
}

func updateSecurityContextConstraints(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	securityContextConstraints, ok1 := required.(*securityv1.SecurityContextConstraints)
	found, ok2 := exists.(*securityv1.SecurityContextConstraints)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to SecurityContextConstraints")
	}
	if !hasSecurityContextConstraintsRightFields(found, securityContextConstraints) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing SecurityContextConstraints Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated SecurityContextConstraints's Spec to its opinionated values")
		}
		util.MergeLabels(&securityContextConstraints.ObjectMeta, &found.ObjectMeta)
		// Copy only the security fields, not the entire object (to preserve ResourceVersion)
		found.AllowPrivilegedContainer = securityContextConstraints.AllowPrivilegedContainer
		found.AllowHostDirVolumePlugin = securityContextConstraints.AllowHostDirVolumePlugin
		found.AllowHostIPC = securityContextConstraints.AllowHostIPC
		found.AllowHostNetwork = securityContextConstraints.AllowHostNetwork
		found.AllowHostPID = securityContextConstraints.AllowHostPID
		found.AllowHostPorts = securityContextConstraints.AllowHostPorts
		found.ReadOnlyRootFilesystem = securityContextConstraints.ReadOnlyRootFilesystem
		found.RunAsUser = securityContextConstraints.RunAsUser
		found.SELinuxContext = securityContextConstraints.SELinuxContext
		found.Users = securityContextConstraints.Users
		found.Volumes = securityContextConstraints.Volumes
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func hasSecurityContextConstraintsRightFields(found *securityv1.SecurityContextConstraints, required *securityv1.SecurityContextConstraints) bool {
	return util.CompareLabels(required, found) &&
		reflect.DeepEqual(found.AllowPrivilegedContainer, required.AllowPrivilegedContainer) &&
		reflect.DeepEqual(found.AllowHostDirVolumePlugin, required.AllowHostDirVolumePlugin) &&
		reflect.DeepEqual(found.AllowHostIPC, required.AllowHostIPC) &&
		reflect.DeepEqual(found.AllowHostNetwork, required.AllowHostNetwork) &&
		reflect.DeepEqual(found.AllowHostPID, required.AllowHostPID) &&
		reflect.DeepEqual(found.AllowHostPorts, required.AllowHostPorts) &&
		reflect.DeepEqual(found.ReadOnlyRootFilesystem, required.ReadOnlyRootFilesystem) &&
		reflect.DeepEqual(found.RunAsUser, required.RunAsUser) &&
		reflect.DeepEqual(found.SELinuxContext, required.SELinuxContext) &&
		reflect.DeepEqual(found.Users, required.Users) &&
		reflect.DeepEqual(found.Volumes, required.Volumes)
}
