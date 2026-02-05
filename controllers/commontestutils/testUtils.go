package commontestutils

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	deschedulerv1 "github.com/openshift/cluster-kube-descheduler-operator/pkg/apis/descheduler/v1"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	aaqv1alpha1 "kubevirt.io/application-aware-quota/staging/src/kubevirt.io/application-aware-quota-api/pkg/apis/core/v1alpha1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	migrationv1alpha1 "kubevirt.io/kubevirt-migration-operator/api/v1alpha1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	"github.com/kubevirt/hyperconverged-cluster-operator/api"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// Name and Namespace of our primary resource
const (
	Name           = "kubevirt-hyperconverged"
	Namespace      = "kubevirt-hyperconverged"
	VirtioWinImage = "quay.io/kubevirt/virtio-container-disk:v2.0.0"
)

var (
	TestLogger  = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("controller_hyperconverged")
	TestRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      Name,
			Namespace: Namespace,
		},
	}
)

func NewHco() *hcov1beta1.HyperConverged {
	hco := components.GetOperatorCR()
	hco.Namespace = Namespace
	return hco
}

func NewHcoNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: Namespace,
		},
	}
}

func NewReq(inst *hcov1beta1.HyperConverged) *common.HcoRequest {
	return &common.HcoRequest{
		Request:      TestRequest,
		Logger:       TestLogger,
		Conditions:   common.NewHcoConditions(),
		Ctx:          context.TODO(),
		Instance:     inst,
		HCOTriggered: true,
	}
}

func getNodePlacement(num1, num2 int64) *sdkapi.NodePlacement {
	var (
		key1 = fmt.Sprintf("key%d", num1)
		key2 = fmt.Sprintf("key%d", num2)

		val1 = fmt.Sprintf("value%d", num1)
		val2 = fmt.Sprintf("value%d", num2)

		operator1 = corev1.NodeSelectorOperator(fmt.Sprintf("operator%d", num1))
		operator2 = corev1.NodeSelectorOperator(fmt.Sprintf("operator%d", num2))

		effect1 = corev1.TaintEffect(fmt.Sprintf("effect%d", num1))
		effect2 = corev1.TaintEffect(fmt.Sprintf("effect%d", num2))

		firstVal1  = fmt.Sprintf("value%d1", num1)
		secondVal1 = fmt.Sprintf("value%d2", num1)
		firstVal2  = fmt.Sprintf("value%d1", num2)
		secondVal2 = fmt.Sprintf("value%d2", num2)
	)
	return &sdkapi.NodePlacement{
		NodeSelector: map[string]string{
			key1: val1,
			key2: val2,
		},
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: key1, Operator: operator1, Values: []string{firstVal1, secondVal1}},
								{Key: key2, Operator: operator2, Values: []string{firstVal2, secondVal2}},
							},
							MatchFields: []corev1.NodeSelectorRequirement{
								{Key: key1, Operator: operator1, Values: []string{firstVal1, secondVal1}},
								{Key: key2, Operator: operator2, Values: []string{firstVal2, secondVal2}},
							},
						},
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{Key: key1, Operator: corev1.TolerationOperator(operator1), Value: val1, Effect: effect1, TolerationSeconds: &num1},
			{Key: key2, Operator: corev1.TolerationOperator(operator2), Value: val2, Effect: effect2, TolerationSeconds: &num2},
		},
	}
}

func NewNodePlacement() *sdkapi.NodePlacement {
	return getNodePlacement(1, 2)
}

func NewOtherNodePlacement() *sdkapi.NodePlacement {
	return getNodePlacement(3, 4)
}

var testScheme *runtime.Scheme

var schemeOnce = &sync.Once{}

func GetScheme() *runtime.Scheme {
	schemeOnce.Do(func() {
		testScheme = runtime.NewScheme()
		for _, f := range []func(*runtime.Scheme) error{
			api.AddToScheme,
			kubevirtcorev1.AddToScheme,
			cdiv1beta1.AddToScheme,
			networkaddonsv1.AddToScheme,
			sspv1beta3.AddToScheme,
			monitoringv1.AddToScheme,
			apiextensionsv1.AddToScheme,
			routev1.Install,
			imagev1.Install,
			consolev1.Install,
			operatorv1.Install,
			openshiftconfigv1.Install,
			securityv1.Install,
			csvv1alpha1.AddToScheme,
			aaqv1alpha1.AddToScheme,
			migrationv1alpha1.AddToScheme,
			deschedulerv1.AddToScheme,
			netattdefv1.AddToScheme,
			rbacv1.AddToScheme,
			networkingv1.AddToScheme,
			appsv1.AddToScheme,
			corev1.AddToScheme,
			schedulingv1.AddToScheme,
			admissionregistrationv1.AddToScheme,
		} {
			if err := f(testScheme); err != nil {
				panic(fmt.Sprintf("failed to add scheme: %T, %v", f, err))
			}
		}
	})

	return testScheme
}

// RepresentCondition - returns a GomegaMatcher useful for comparing conditions
func RepresentCondition(expected metav1.Condition) gomegatypes.GomegaMatcher {
	return &RepresentConditionMatcher{
		expected: expected,
	}
}

type RepresentConditionMatcher struct {
	expected metav1.Condition
}

// Match - compares two conditions
// two conditions are the same if they have the same type, status, reason, and message
func (matcher *RepresentConditionMatcher) Match(actual interface{}) (success bool, err error) {
	actualCondition, ok := actual.(metav1.Condition)
	if !ok {
		return false, fmt.Errorf("RepresentConditionMatcher expects a Condition")
	}

	if matcher.expected.Type != actualCondition.Type {
		return false, nil
	}
	if matcher.expected.Status != actualCondition.Status {
		return false, nil
	}
	if matcher.expected.Reason != actualCondition.Reason {
		return false, nil
	}
	if matcher.expected.Message != actualCondition.Message {
		return false, nil
	}
	return true, nil
}

func (matcher *RepresentConditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto match the condition\n\t%#v", actual, matcher.expected)
}

func (matcher *RepresentConditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to match the condition\n\t%#v", actual, matcher.expected)
}

const (
	BaseDomain = "basedomain"
)

// ClusterInfoMock mocks regular Openshift
type ClusterInfoMock struct{}

// make sure ClusterInfoMock implements hcoutil.ClusterInfo
var _ hcoutil.ClusterInfo = &ClusterInfoMock{}

func (ClusterInfoMock) Init(_ context.Context, _ client.Client, _ logr.Logger) error {
	return nil
}
func (ClusterInfoMock) IsOpenshift() bool {
	return true
}
func (ClusterInfoMock) IsRunningLocally() bool {
	return false
}
func (ClusterInfoMock) IsManagedByOLM() bool {
	return true
}
func (ClusterInfoMock) GetBaseDomain() string {
	return BaseDomain
}
func (c ClusterInfoMock) IsConsolePluginImageProvided() bool {
	return true
}
func (c ClusterInfoMock) IsMonitoringAvailable() bool {
	return true
}
func (c ClusterInfoMock) IsDeschedulerAvailable() bool {
	return true
}
func (c ClusterInfoMock) IsNADAvailable() bool {
	return true
}
func (c ClusterInfoMock) IsDeschedulerCRDDeployed(_ context.Context, _ client.Client) bool {
	return true
}
func (c ClusterInfoMock) IsSingleStackIPv6() bool {
	return true
}

func (c ClusterInfoMock) IsHyperShiftManaged() bool {
	return false
}

func KeysFromSSMap(ssmap map[string]string) gstruct.Keys {
	keys := gstruct.Keys{}
	for k, v := range ssmap {
		keys[k] = Equal(v)
	}
	return keys
}
