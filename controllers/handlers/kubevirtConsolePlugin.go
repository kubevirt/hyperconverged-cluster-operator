package handlers

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/template"

	log "github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	kvUIPluginName            = "kubevirt-plugin"
	kvUIPluginDeploymentName  = string(hcoutil.AppComponentUIPlugin)
	kvUIProxyDeploymentName   = string(hcoutil.AppComponentUIProxy)
	kvUIPluginSvcName         = kvUIPluginDeploymentName + "-service"
	kvUIProxySvcName          = kvUIProxyDeploymentName + "-service"
	kvUIPluginServingCertName = "plugin-serving-cert"
	kvUIProxyServingCertName  = "console-proxy-serving-cert"
	kvUIPluginServingCertPath = "/var/serving-cert"
	kvUIProxyServingCertPath  = "/app/cert"
	nginxConfigMapName        = "nginx-conf"
	kvUIUserSettingsCMName    = "kubevirt-user-settings"
	kvUIFeaturesCMName        = "kubevirt-ui-features"
	kvUIConfigReaderRoleName  = "kubevirt-ui-config-reader"
	kvUIConfigReaderRBName    = "kubevirt-ui-config-reader-rolebinding"
)

const ( // for network policies
	k8sDNSNamespaceSelector = "kube-system"
	k8sDNSPodSelectorLabel  = "k8s-app"
	k8sDNSPodSelectorVal    = "kube-dns"
	k8sDNSPort              = int32(53)

	openshiftDNSNamespaceSelector = "openshift-dns"
	openshiftDNSPodSelectorLabel  = "dns.operator.openshift.io/daemonset-dns"
	openshiftDNSPodSelectorVal    = "default"
	openshiftDNSPort              = int32(5353)

	apiServerPort int32 = 6443
)

// **** Kubevirt UI Plugin Deployment Handler ****
func NewKvUIPluginDeploymentHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewDeploymentHandler(Client, Scheme, NewKvUIPluginDeployment, hc), nil
}

// **** Kubevirt UI apiserver proxy Deployment Handler ****
func NewKvUIProxyDeploymentHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewDeploymentHandler(Client, Scheme, NewKvUIProxyDeployment, hc), nil
}

// **** Kubevirt UI Plugin ServiceAccount Handler ****
func NewKvUIPluginSAHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewServiceAccountHandler(Client, Scheme, NewKvUIPluginSA), nil
}

// **** Kubevirt UI Proxy ServiceAccount Handler ****
func NewKvUIProxySAHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewServiceAccountHandler(Client, Scheme, NewKvUIProxySA), nil
}

func NewKvUIPluginSA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIPluginDeploymentName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
		},
	}
}

func NewKvUIProxySA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIProxyDeploymentName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIProxy),
		},
	}
}

// **** nginx config map Handler ****
func NewKvUINginxCMHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, _ *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewDynamicCmHandler(Client, Scheme, NewKVUINginxCM), nil
}

// **** UI user settings config map Handler ****
func NewKvUIUserSettingsCMHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewCmHandler(Client, Scheme, NewKvUIUserSettingsCM(hc)), nil
}

// **** UI features config map Handler ****
func NewKvUIFeaturesCMHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewCmHandler(Client, Scheme, NewKvUIFeaturesCM(hc)), nil
}

// **** Kubevirt UI Console Plugin Custom Resource Handler ****
func NewKvUIPluginCRHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return newConsolePluginHandler(Client, Scheme, NewKVConsolePlugin(hc)), nil
}

func NewKvUIPluginDeployment(hc *hcov1beta1.HyperConverged) *appsv1.Deployment {
	// The env var was validated prior to handler creation
	kvUIPluginImage, _ := os.LookupEnv(hcoutil.KVUIPluginImageEnvV)
	deployment := getKvUIDeployment(hc, kvUIPluginDeploymentName, kvUIPluginImage,
		kvUIPluginServingCertName, kvUIPluginServingCertPath, hcoutil.UIPluginServerPort, hcoutil.AppComponentUIPlugin)

	nginxVolumeMount := corev1.VolumeMount{
		Name:      nginxConfigMapName,
		MountPath: "/etc/nginx/nginx.conf",
		SubPath:   "nginx.conf",
		ReadOnly:  true,
	}

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		nginxVolumeMount,
	)

	nginxVolume := corev1.Volume{
		Name: nginxConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: nginxConfigMapName,
				},
			},
		},
	}

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, nginxVolume)

	return deployment
}

func NewKvUIProxyDeployment(hc *hcov1beta1.HyperConverged) *appsv1.Deployment {
	// The env var was validated prior to handler creation
	kvUIProxyImage, _ := os.LookupEnv(hcoutil.KVUIProxyImageEnvV)
	deployment := getKvUIDeployment(hc, kvUIProxyDeploymentName, kvUIProxyImage, kvUIProxyServingCertName,
		kvUIProxyServingCertPath, hcoutil.UIProxyServerPort, hcoutil.AppComponentUIProxy)

	ciphers, minTLSVersion := tlssecprofile.GetCipherSuitesAndMinTLSVersionInGolangFormat(hc.Spec.TLSSecurityProfile)

	var args []string
	if len(ciphers) > 0 {
		cipherStrs := make([]string, len(ciphers))
		for i := range ciphers {
			cipherStrs[i] = strconv.Itoa(int(ciphers[i]))
		}

		cipherSuiteStr := strings.Join(cipherStrs, ",")
		arg := fmt.Sprintf("--tls-cipher-suites=%s", cipherSuiteStr)
		args = append(args, arg)
	}

	arg := fmt.Sprintf("--tls-min-version=%d", minTLSVersion)
	args = append(args, arg)

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, args...)

	return deployment
}

func getKvUIDeployment(hc *hcov1beta1.HyperConverged, deploymentName string, image string,
	servingCertName string, servingCertPath string, port int32, componentName hcoutil.AppComponent) *appsv1.Deployment {
	labels := operands.GetLabels(hc, componentName)
	infrastructureHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable()
	var replicas int32
	if infrastructureHighlyAvailable {
		replicas = int32(2)
	} else {
		replicas = int32(1)
	}

	affinity := operands.GetPodAntiAffinity(labels[hcoutil.AppLabelComponent], infrastructureHighlyAvailable)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Labels:    labels,
			Namespace: hc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"openshift.io/required-scc": "restricted-v2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: deploymentName,
					SecurityContext:    components.GetStdPodSecurityContext(),
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: port,
								Protocol:      corev1.ProtocolTCP,
							}},
							SecurityContext:          components.GetStdContainerSecurityContext(),
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      servingCertName,
									MountPath: servingCertPath,
									ReadOnly:  true,
								},
							},
						},
					},
					PriorityClassName: kvPriorityClass,
					Volumes: []corev1.Volume{
						{
							Name: servingCertName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  servingCertName,
									DefaultMode: ptr.To(int32(420)),
								},
							},
						},
					},
				},
			},
		},
	}

	if hc.Spec.Infra.NodePlacement != nil {
		if hc.Spec.Infra.NodePlacement.NodeSelector != nil {
			deployment.Spec.Template.Spec.NodeSelector = maps.Clone(hc.Spec.Infra.NodePlacement.NodeSelector)
		} else {
			deployment.Spec.Template.Spec.NodeSelector = nil
		}

		if hc.Spec.Infra.NodePlacement.Affinity != nil {
			deployment.Spec.Template.Spec.Affinity = hc.Spec.Infra.NodePlacement.Affinity.DeepCopy()
		} else {
			deployment.Spec.Template.Spec.Affinity = affinity
		}

		if hc.Spec.Infra.NodePlacement.Tolerations != nil {
			deployment.Spec.Template.Spec.Tolerations = make([]corev1.Toleration, len(hc.Spec.Infra.NodePlacement.Tolerations))
			copy(deployment.Spec.Template.Spec.Tolerations, hc.Spec.Infra.NodePlacement.Tolerations)
		} else {
			deployment.Spec.Template.Spec.Tolerations = nil
		}
	} else {
		deployment.Spec.Template.Spec.NodeSelector = nil
		deployment.Spec.Template.Spec.Affinity = affinity
		deployment.Spec.Template.Spec.Tolerations = nil
	}
	return deployment
}

func NewKvUIPluginSvc(hc *hcov1beta1.HyperConverged) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Port:       hcoutil.UIPluginServerPort,
			Name:       kvUIPluginDeploymentName + "-port",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: hcoutil.UIPluginServerPort},
		},
	}

	spec := corev1.ServiceSpec{
		Ports:    servicePorts,
		Selector: map[string]string{hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIPlugin)},
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   kvUIPluginSvcName,
			Labels: operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": kvUIPluginServingCertName,
			},
			Namespace: hc.Namespace,
		},
		Spec: spec,
	}
}

func NewKvUIProxySvc(hc *hcov1beta1.HyperConverged) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Port:       hcoutil.UIProxyServerPort,
			Name:       kvUIProxyDeploymentName + "-port",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: hcoutil.UIProxyServerPort},
		},
	}

	spec := corev1.ServiceSpec{
		Ports:    servicePorts,
		Selector: map[string]string{hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIProxy)},
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   kvUIProxySvcName,
			Labels: operands.GetLabels(hc, hcoutil.AppComponentUIProxy),
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": kvUIProxyServingCertName,
			},
			Namespace: hc.Namespace,
		},
		Spec: spec,
	}
}

//go:embed templates/nginx.conf.tmpl
var nginxConfTemplate string

var nginxConfTmpl = template.Must(template.New("nginx.conf").Parse(nginxConfTemplate))

func nginxSSLProtocolsFromMinTLS(minTLS openshiftconfigv1.TLSProtocolVersion) string {
	switch minTLS {
	case openshiftconfigv1.VersionTLS10:
		return "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3"
	case openshiftconfigv1.VersionTLS11:
		return "TLSv1.1 TLSv1.2 TLSv1.3"
	case openshiftconfigv1.VersionTLS12:
		return "TLSv1.2 TLSv1.3"
	case openshiftconfigv1.VersionTLS13:
		return "TLSv1.3"
	default:
		return "TLSv1.2 TLSv1.3"
	}
}

type nginxConfTemplateData struct {
	Port         int32
	SSLProtocols string
	SSLCiphers   string
}

func getNginxConfig(hc *hcov1beta1.HyperConverged) (string, error) {
	ciphers, minTLS := tlssecprofile.GetCipherSuitesAndMinTLSVersion(hc.Spec.TLSSecurityProfile)
	data := nginxConfTemplateData{
		Port:         hcoutil.UIPluginServerPort,
		SSLProtocols: nginxSSLProtocolsFromMinTLS(minTLS),
		SSLCiphers:   strings.Join(ciphers, ":"),
	}

	var out bytes.Buffer
	if err := nginxConfTmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("failed rendering embedded nginx.conf template: %w", err)
	}

	return out.String(), nil
}

func NewKVUINginxCM(hc *hcov1beta1.HyperConverged) (*corev1.ConfigMap, error) {
	nginxConf, err := getNginxConfig(hc)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginxConfigMapName,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
			Namespace: hc.Namespace,
		},
		Data: map[string]string{
			"nginx.conf": nginxConf,
		},
	}, nil
}

func NewKvUIUserSettingsCM(hc *hcov1beta1.HyperConverged) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIUserSettingsCMName,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIConfig),
			Namespace: hc.Namespace,
		},
		Data: map[string]string{},
	}
}

var UIFeaturesConfig = map[string]string{
	"automaticSubscriptionActivationKey":  "",
	"automaticSubscriptionOrganizationId": "",
	"disabledGuestSystemLogsAccess":       "false",
	"kubevirtApiserverProxy":              "true",
	"loadBalancerEnabled":                 "true",
	"nodePortAddress":                     "",
	"nodePortEnabled":                     "false",
}

func NewKvUIFeaturesCM(hc *hcov1beta1.HyperConverged) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIFeaturesCMName,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIConfig),
			Namespace: hc.Namespace,
		},
		Data: UIFeaturesConfig,
	}
}

func NewKVConsolePlugin(hc *hcov1beta1.HyperConverged) *consolev1.ConsolePlugin {
	return &consolev1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name:   kvUIPluginName,
			Labels: operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
		},
		Spec: consolev1.ConsolePluginSpec{
			DisplayName: "Kubevirt Console Plugin",
			Backend: consolev1.ConsolePluginBackend{
				Type: consolev1.Service,
				Service: &consolev1.ConsolePluginService{
					Name:      kvUIPluginSvcName,
					Namespace: hc.Namespace,
					Port:      hcoutil.UIPluginServerPort,
					BasePath:  "/",
				},
			},
			Proxy: []consolev1.ConsolePluginProxy{{
				Alias:         kvUIProxyDeploymentName,
				Authorization: consolev1.UserToken,
				Endpoint: consolev1.ConsolePluginProxyEndpoint{
					Type: consolev1.ProxyTypeService,
					Service: &consolev1.ConsolePluginProxyServiceConfig{
						Name:      kvUIProxySvcName,
						Namespace: hc.Namespace,
						Port:      hcoutil.UIProxyServerPort,
					},
				},
			}},
		},
	}
}

func newConsolePluginHandler(Client client.Client, Scheme *runtime.Scheme, required *consolev1.ConsolePlugin) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "ConsolePlugin", &consolePluginHooks{required: required}, false)
}

// NewKvUIConfigReaderRoleHandler returns UI configuration (user settings and features) ConfigMap Role Handler
func NewKvUIConfigReaderRoleHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewRoleHandler(Client, Scheme, NewKvUIConfigCMReaderRole(hc)), nil
}

// NewKvUIConfigReaderRoleBindingHandler returns UI configuration (user settings and features) ConfigMap RoleBinding Handler
func NewKvUIConfigReaderRoleBindingHandler(_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewRoleBindingHandler(Client, Scheme, NewKvUIConfigCMReaderRoleBinding(hc)), nil
}

func NewKvUIConfigCMReaderRole(hc *hcov1beta1.HyperConverged) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIConfigReaderRoleName,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
			Namespace: hc.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{kvUIUserSettingsCMName},
				Verbs:         []string{"get", "update", "patch"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{kvUIFeaturesCMName},
				Verbs:         []string{"get"},
			},
		},
	}
}

func NewKvUIConfigCMReaderRoleBinding(hc *hcov1beta1.HyperConverged) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvUIConfigReaderRBName,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
			Namespace: hc.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     kvUIConfigReaderRoleName,
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

type consolePluginHooks struct {
	required *consolev1.ConsolePlugin
}

func (h consolePluginHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h consolePluginHooks) GetEmptyCr() client.Object {
	return &consolev1.ConsolePlugin{}
}

func (h consolePluginHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*consolev1.ConsolePlugin)

	if !ok {
		return false, false, errors.New("can't convert to ConsolePlugin")
	}

	if !reflect.DeepEqual(h.required.Spec, found.Spec) ||
		!hcoutil.CompareLabels(h.required, found) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing ConsolePlugin to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated ConsolePlugin to its opinionated values", "name", h.required.Name)
		}
		hcoutil.MergeLabels(&h.required.ObjectMeta, &found.ObjectMeta)
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
}

func (h consoleHandler) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	// Enable console plugin for kubevirt if not already enabled
	consoleKey := client.ObjectKey{Namespace: hcoutil.UndefinedNamespace, Name: "cluster"}
	consoleObj := &operatorv1.Console{}
	err := h.Client.Get(req.Ctx, consoleKey, consoleObj)
	if err != nil {
		req.Logger.Error(err, fmt.Sprintf("Could not find resource - APIVersion: %s, Kind: %s, Name: %s",
			consoleObj.APIVersion, consoleObj.Kind, consoleObj.Name))
		return &operands.EnsureResult{
			Err: nil,
		}
	}

	if !slices.Contains(consoleObj.Spec.Plugins, kvUIPluginName) {
		req.Logger.Info("Enabling kubevirt plugin in Console")
		consoleObj.Spec.Plugins = append(consoleObj.Spec.Plugins, kvUIPluginName)
		err := h.Client.Update(req.Ctx, consoleObj)
		if err != nil {
			req.Logger.Error(err, fmt.Sprintf("Could not update resource - APIVersion: %s, Kind: %s, Name: %s",
				consoleObj.APIVersion, consoleObj.Kind, consoleObj.Name))
			return &operands.EnsureResult{
				Err: err,
			}
		}

		return &operands.EnsureResult{
			Err:         nil,
			Updated:     true,
			UpgradeDone: true,
		}
	}
	return &operands.EnsureResult{
		Err:         nil,
		Updated:     false,
		UpgradeDone: true,
	}
}

func (consoleHandler) Reset() { /* no implementation */ }

func NewConsoleHandler(Client client.Client) operands.Operand {
	h := &consoleHandler{
		Client: Client,
	}
	return h
}

func newKVConsolePluginNetworkPolicy(hc *hcov1beta1.HyperConverged) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-console-plugin-np",
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIPlugin),
		},

		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					hcoutil.AppLabel:          hc.Name,
					hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIPlugin),
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: hcoutil.UIPluginServerPort},
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									hcoutil.KubernetesMetadataName: "openshift-console",
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":       "console",
									"component": "ui",
								},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}

func NewKVConsolePluginNetworkPolicyHandler(_ log.Logger, cli client.Client, schm *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	np := newKVConsolePluginNetworkPolicy(hc)

	return operands.NewNetworkPolicyHandler(cli, schm, np), nil
}

func getApiServerEgressRule() networkingv1.NetworkPolicyEgressRule {
	var (
		dnsNamespcaeSelector = k8sDNSNamespaceSelector
		dnsPodSelectorLabel  = k8sDNSPodSelectorLabel
		dnsPodSelectorVal    = k8sDNSPodSelectorVal
		dnsPort              = k8sDNSPort
	)

	if hcoutil.GetClusterInfo().IsOpenshift() {
		dnsNamespcaeSelector = openshiftDNSNamespaceSelector
		dnsPodSelectorLabel = openshiftDNSPodSelectorLabel
		dnsPodSelectorVal = openshiftDNSPodSelectorVal
		dnsPort = openshiftDNSPort
	}

	return networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: ptr.To(corev1.ProtocolTCP),
				Port:     ptr.To(intstr.FromInt32(dnsPort)),
			},
			{
				Protocol: ptr.To(corev1.ProtocolUDP),
				Port:     ptr.To(intstr.FromInt32(dnsPort)),
			},
		},
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						hcoutil.KubernetesMetadataName: dnsNamespcaeSelector,
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						dnsPodSelectorLabel: dnsPodSelectorVal,
					},
				},
			},
		},
	}
}

func newKVAPIServerProxyNetworkPolicy(hc *hcov1beta1.HyperConverged) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-apiserver-proxy-np",
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentUIProxy),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					hcoutil.AppLabel:          hc.Name,
					hcoutil.AppLabelComponent: string(hcoutil.AppComponentUIProxy),
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: hcoutil.UIProxyServerPort},
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				getApiServerEgressRule(),
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: apiServerPort},
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}

func NewKVAPIServerProxyNetworkPolicyHandler(_ log.Logger, cli client.Client, schm *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	np := newKVAPIServerProxyNetworkPolicy(hc)

	return operands.NewNetworkPolicyHandler(cli, schm, np), nil
}
