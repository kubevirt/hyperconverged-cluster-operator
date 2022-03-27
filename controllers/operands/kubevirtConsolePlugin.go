package operands

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/cmdcommon"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"

	log "github.com/go-logr/logr"
	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	kvUIPluginName    = "kubevirt-plugin"
	kvUIPluginSvcName = kvUIPluginName + "-service"
	kvUIPluginNameEnv = "UI_PLUGIN_NAME"
)

// **** Kubevirt UI Plugin Deployment Handler ****
func newKvUiPluginDplymntHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]Operand, error) {
	kvUiPluginDeplymnt, err := NewKvUiPluginDeplymnt(hc)
	if err != nil {
		return nil, err
	}
	return []Operand{newDeploymentHandler(Client, Scheme, kvUiPluginDeplymnt)}, nil
}

// **** Kubevirt UI Plugin Service Handler ****
func newKvUiPluginSvcHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]Operand, error) {
	kvUiPluginSvc := NewKvUiPluginSvc(hc)

	return []Operand{newServiceHandler(Client, Scheme, kvUiPluginSvc)}, nil
}

// **** Kubevirt UI Console Plugin Custom Resource Handler ****
func newKvUiPluginCRHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]Operand, error) {
	kvUiConsolePluginCR := NewKvConsolePlugin(hc)

	return []Operand{newConsolePluginHandler(Client, Scheme, kvUiConsolePluginCR)}, nil
}

func NewKvUiPluginDeplymnt(hc *hcov1beta1.HyperConverged) (*appsv1.Deployment, error) {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIPluginName,
			Labels:    getLabels(hc, hcoutil.AppComponentDeployment),
			Namespace: hc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": kvUIPluginName,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": kvUIPluginName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					Containers: []corev1.Container{
						{
							Name:            kvUIPluginName,
							Image:           "quay.io/kubevirt-ui/kubevirt-plugin:latest",
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{{
								ContainerPort: hcoutil.UiPluginServerPort,
								Protocol:      corev1.ProtocolTCP,
							}},
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					PriorityClassName: "system-cluster-critical",
				},
			},
		},
	}, nil
}

func NewKvUiPluginSvc(hc *hcov1beta1.HyperConverged) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{Port: hcoutil.UiPluginServerPort, Name: kvUIPluginName + "-port", Protocol: corev1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: hcoutil.UiPluginServerPort}},
	}
	pluginName := kvUIPluginName
	val, ok := os.LookupEnv(kvUIPluginNameEnv)
	if ok && val != "" {
		pluginName = val
	}
	labelSelect := map[string]string{"app": pluginName}

	spec := corev1.ServiceSpec{
		Ports:    servicePorts,
		Selector: labelSelect,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIPluginSvcName,
			Labels:    getLabels(hc, hcoutil.AppComponentDeployment),
			Namespace: hc.Namespace,
		},
		Spec: spec,
	}
}

func NewKvConsolePlugin(hc *hcov1beta1.HyperConverged) *consolev1alpha1.ConsolePlugin {
	return &consolev1alpha1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name:   kvUIPluginName,
			Labels: getLabels(hc, hcoutil.AppComponentDeployment),
		},
		Spec: consolev1alpha1.ConsolePluginSpec{
			DisplayName: "Kubevirt Console Plugin",
			Service: consolev1alpha1.ConsolePluginService{
				Name:      kvUIPluginSvcName,
				Namespace: hc.Namespace,
				Port:      int32(hcoutil.UiPluginServerPort),
				BasePath:  "/",
			},
		},
	}
}

func newConsolePluginHandler(Client client.Client, Scheme *runtime.Scheme, required *consolev1alpha1.ConsolePlugin) Operand {
	h := &genericOperand{
		Client:              Client,
		Scheme:              Scheme,
		crType:              "ConsolePlugin",
		removeExistingOwner: false,
		hooks:               &consolePluginHooks{required: required},
	}

	return h
}

type consolePluginHooks struct {
	required *consolev1alpha1.ConsolePlugin
}

func (h consolePluginHooks) getFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h consolePluginHooks) getEmptyCr() client.Object {
	return &consolev1alpha1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.required.Name,
		},
	}
}

func (h consolePluginHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*consolev1alpha1.ConsolePlugin).ObjectMeta
}

func (h consolePluginHooks) reset() { /* no implementation */ }

func (h consolePluginHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (h consolePluginHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*consolev1alpha1.ConsolePlugin)

	if !ok {
		return false, false, errors.New("can't convert to ConsolePlugin")
	}

	if !reflect.DeepEqual(found.Spec, h.required.Spec) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing ConsolePlugin to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated ConsolePlugin to its opinionated values", "name", h.required.Name)
		}
		hcoutil.DeepCopyLabels(&h.required.ObjectMeta, &found.ObjectMeta)
		h.required.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

type consoleHandler struct {
	// K8s client
	Client client.Client
	Scheme *runtime.Scheme
}

func (h consoleHandler) ensure(req *common.HcoRequest) *EnsureResult {
	// Enable console plugin for kubevirt if not already enabled
	consoleKey := client.ObjectKey{Namespace: hcoutil.UndefinedNamespace, Name: "cluster"}
	consoleObj := &operatorv1.Console{}
	err := h.Client.Get(req.Ctx, consoleKey, consoleObj)
	if err != nil {
		req.Logger.Error(err, fmt.Sprintf("Could not find resource - APIVersion: %s, Kind: %s, Name: %s",
			consoleObj.APIVersion, consoleObj.Kind, consoleObj.Name))
		return &EnsureResult{
			Err: nil,
		}
	}

	plugins := consoleObj.Spec.Plugins
	if !cmdcommon.StringInSlice(kvUIPluginName, plugins) {
		req.Logger.Info("Enabling kubevirt plugin in Console")
		plugins = append(plugins, kvUIPluginName)
		consoleObj.Spec.Plugins = plugins
		err := h.Client.Update(req.Ctx, consoleObj)
		if err != nil {
			req.Logger.Error(err, fmt.Sprintf("Could not update resource - APIVersion: %s, Kind: %s, Name: %s",
				consoleObj.APIVersion, consoleObj.Kind, consoleObj.Name))
			return &EnsureResult{
				Err: err,
			}
		} else {
			return &EnsureResult{
				Err:         nil,
				Updated:     true,
				UpgradeDone: true,
			}
		}
	}
	return &EnsureResult{
		Err:         nil,
		Updated:     false,
		UpgradeDone: true,
	}
}

func (h consoleHandler) reset() { /* no implementation */ }

func newConsoleHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]Operand, error) {
	h := &consoleHandler{
		Client: Client,
		Scheme: Scheme,
	}
	return []Operand{h}, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}
