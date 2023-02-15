package validator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks/mutator"

	"github.com/go-logr/logr"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sadmission "k8s.io/apiserver/pkg/admission"
	quotaplugin "k8s.io/apiserver/pkg/admission/plugin/resourcequota"
	"k8s.io/apiserver/pkg/admission/plugin/resourcequota/apis/resourcequota"
	v12 "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/evaluator/core"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "kubevirt.io/api/core/v1"
	corev1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	allowMassage                = "pod won't block migration creation is allowed"
	capacityValidatorAnnotation = "kubevirt.io/capacity-validator"

	CapacityValidator = "CapacityValidator"
)

type MigrationCapacityValidator struct {
	cli          client.Client
	hcoNamespace string
	decoder      *admission.Decoder
	logger       logr.Logger
}

func NewMigrationCapacityValidator(logger logr.Logger, cli client.Client, hcoNamespace string) *MigrationCapacityValidator {
	return &MigrationCapacityValidator{
		cli:          cli,
		hcoNamespace: hcoNamespace,
		logger:       logger,
	}
}

func (mvc *MigrationCapacityValidator) Handle(ctx context.Context, req admission.Request) admission.Response {

	hco, err := mutator.GetHcoObject(ctx, mvc.cli, mvc.hcoNamespace)
	if err != nil {
		mvc.logErr(err, fmt.Sprintf("%v: cannot get the HyperConverged object", CapacityValidator))
		return admission.Errored(http.StatusBadRequest, err)
	}
	if _, capacityValidatorAnnotationExist := hco.Annotations[capacityValidatorAnnotation]; !capacityValidatorAnnotationExist {
		mvc.logger.Info(fmt.Sprintf("%v: doesn't exist", CapacityValidator))
		return admission.Allowed(allowMassage)
	}

	podToCreate := &k8sv1.Pod{}
	err = mvc.decoder.Decode(req, podToCreate)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	virtLauncherList, err := getlauncherPodListInNamespace(mvc.logger, ctx, mvc.cli, podToCreate.Namespace)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	} else if isLauncherPod(podToCreate) {
		migrations, err := getMigrationListInNamespace(mvc.logger, ctx, mvc.cli, podToCreate.Namespace)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if launcherIsMigrating(podToCreate, migrations) {
			return admission.Allowed(allowMassage) //migrating vmi should not be blocked
		}
		virtLauncherList.Items = append(virtLauncherList.Items, *podToCreate)
	}

	// TODO: make sure that virt-launcher pod has VirtualMachineInstanceIsMigratable condition as well so we could avoid messing with VMIs in this web-hook
	vmiList, err := getVMIListInNamespace(mvc.logger, ctx, mvc.cli, podToCreate.Namespace)
	if err != nil {
		mvc.logger.Error(err, fmt.Sprintf("%v: failed getting VMIList in namespace:%v", CapacityValidator, podToCreate.Namespace))
		return admission.Errored(http.StatusBadRequest, err)
	}

	launcherPodsToConsider := filterLauncherPodsFromList(&virtLauncherList.Items,
		launcherDidntFailFilter(),
		launcherDidntSucceededFilter(),
		//IMPORTANT: if the created launcher pod belongs to a new vmi we can't possibly know if
		//the vmi is migratable because it is determined AFTER the launcher pod is created
		//this filter will assume that new vmi is migratable anyway for now.
		launcherVMIIsMigratableFilter(mvc.logger, vmiList),
	)
	if len(launcherPodsToConsider) == 0 {
		return admission.Allowed(allowMassage) //there aren't any VMIs in this namespace
	}

	resourceQuotaList, err := getResourceQuotaListInNamespace(mvc.logger, ctx, mvc.cli, podToCreate.Namespace)
	if err != nil {
		mvc.logger.Error(err, fmt.Sprintf("%v: failed getting resourceQuotaList in namespace:%v", CapacityValidator, podToCreate.Namespace))
		return admission.Errored(http.StatusBadRequest, err)
	}

	podEvaluator := core.NewPodEvaluator(nil, clock.RealClock{})
	limitedResources := []resourcequota.LimitedResource{
		{
			Resource:      "pods",
			MatchContains: []string{"cpu", "memory"},
		},
	}
	podToCreateAttr := k8sadmission.NewAttributesRecord(podToCreate, nil, apiextensions.Kind("Pod").WithVersion("version"), podToCreate.Namespace, podToCreate.Name, corev1beta1.Resource("pods").WithVersion("version"), "", k8sadmission.Create, &metav1.CreateOptions{}, false, nil)

	resourceQuotaListAfterPodAdmission, err := admitPodToQuotas(resourceQuotaList.Items, podToCreateAttr, podEvaluator, limitedResources)
	if err != nil { //if the pod violate quota rules we will let the built-in ResourceQuota admission controller reject it
		return admission.Allowed(allowMassage)
	}

	for _, virtLauncherPod := range launcherPodsToConsider {
		virtLauncherAttr := k8sadmission.NewAttributesRecord(&virtLauncherPod, nil, apiextensions.Kind("Pod").WithVersion("version"), virtLauncherPod.Namespace, virtLauncherPod.Name, corev1beta1.Resource("pods").WithVersion("version"), "", k8sadmission.Create, &metav1.CreateOptions{}, false, nil)
		_, err := quotaplugin.CheckRequest(resourceQuotaListAfterPodAdmission, virtLauncherAttr, podEvaluator, limitedResources)
		if err != nil {
			errMsg := fmt.Sprintf("%v: Please be advised that creation of Pod:%v in the %v namespace,"+
				" may prevent migration of VMI:%v due to ResourceQuota constraints."+
				" In order to avoid encountering the following error message when attempting to migrate the VMI:\n\""+err.Error()+
				"\",you will need to increase the ResourceQuotas for the namespace. ", CapacityValidator, podToCreate.Name,
				podToCreate.Namespace, virtLauncherPod.Labels[v1.VirtualMachineNameLabel])
			mvc.logger.Info(errMsg)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf(errMsg))
		}
	}

	return admission.Allowed(allowMassage)

}

// InjectDecoder injects the decoder.
// WebhookHandler implements admission.DecoderInjector so a decoder will be automatically injected.
func (mvc *MigrationCapacityValidator) InjectDecoder(d *admission.Decoder) error {
	mvc.decoder = d
	return nil
}

func (mvc *MigrationCapacityValidator) logErr(err error, format string, a ...any) {
	mvc.logger.Error(err, fmt.Sprintf(format, a...))
}

func getResourceQuotaListInNamespace(logger logr.Logger, ctx context.Context, cli client.Client, namespace string) (*k8sv1.ResourceQuotaList, error) {
	resourceQuota := k8sv1.ResourceQuotaList{}
	err := cli.List(ctx, &resourceQuota, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("%v: failed getting resourceQuotaList in namespace:%v ", CapacityValidator, namespace))
		return nil, err
	}

	return &resourceQuota, nil
}

func getlauncherPodListInNamespace(logger logr.Logger, ctx context.Context, cli client.Client, namespace string) (*k8sv1.PodList, error) {
	podList := k8sv1.PodList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", "kubevirt.io", "virt-launcher"))
	if err != nil {
		panic(err)
	}
	err = cli.List(ctx, &podList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("%v: failed getting podList in namespace:%v", CapacityValidator, namespace))
		return nil, err
	}

	return &podList, nil
}

func getMigrationListInNamespace(logger logr.Logger, ctx context.Context, cli client.Client, namespace string) (*v1.VirtualMachineInstanceMigrationList, error) {
	migrationList := v1.VirtualMachineInstanceMigrationList{}
	err := cli.List(ctx, &migrationList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("%v: failed getting migrationList in namespace:%v ", CapacityValidator, namespace))
		return nil, err
	}

	return &migrationList, nil
}

func isLauncherPod(p *k8sv1.Pod) bool {
	value := p.Labels["kubevirt.io"]
	return value == "virt-launcher"
}
func launcherIsMigrating(p *k8sv1.Pod, migrations *v1.VirtualMachineInstanceMigrationList) bool {
	value := p.ObjectMeta.Labels[v1.MigrationJobLabel]
	for _, migration := range migrations.Items {
		if value == string(migration.ObjectMeta.UID) {
			return true
		}
	}
	return false
}

func getVMIListInNamespace(logger logr.Logger, ctx context.Context, cli client.Client, namespace string) (*v1.VirtualMachineInstanceList, error) {
	vmiList := v1.VirtualMachineInstanceList{}
	err := cli.List(ctx, &vmiList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("%v: failed getting VMList in namespace:%v", CapacityValidator, namespace))
		return nil, err
	}

	return &vmiList, nil
}

func admitPodToQuotas(originalQuotaList []k8sv1.ResourceQuota, attributes k8sadmission.Attributes, evaluator v12.Evaluator, limitedResources []resourcequota.LimitedResource) ([]k8sv1.ResourceQuota, error) {
	return quotaplugin.CheckRequest(originalQuotaList, attributes, evaluator, limitedResources)
}

func vmiIsMigratable(vmi *v1.VirtualMachineInstance) bool {
	for _, c := range vmi.Status.Conditions {
		if c.Type == v1.VirtualMachineInstanceIsMigratable &&
			c.Status == k8sv1.ConditionFalse {
			return false
		}
	}
	return true
}

type filterPredicateFunc func(vmi *k8sv1.Pod) bool

func launcherDidntFailFilter() filterPredicateFunc {
	return func(launcherPod *k8sv1.Pod) bool {
		return launcherPod.Status.Phase != k8sv1.PodFailed
	}
}
func launcherDidntSucceededFilter() filterPredicateFunc {
	return func(launcherPod *k8sv1.Pod) bool {
		return launcherPod.Status.Phase != k8sv1.PodSucceeded
	}
}
func launcherVMIIsMigratableFilter(logger logr.Logger, vmiList *v1.VirtualMachineInstanceList) filterPredicateFunc {
	return func(launcherPod *k8sv1.Pod) bool {
		if len(vmiList.Items) == 0 {
			return false
		}
		for _, vmi := range vmiList.Items {
			if vmi.Name == launcherPod.Labels[v1.VirtualMachineNameLabel] {
				return vmiIsMigratable(&vmi)
			}
		}
		logger.Info(fmt.Sprintf("%v: Warning :Didn't find vm for launcher:%v", CapacityValidator, launcherPod.Name))
		return false
	}
}

func filterLauncherPodsFromList(objs *[]k8sv1.Pod, predicates ...filterPredicateFunc) []k8sv1.Pod {
	var match []k8sv1.Pod
	for _, launcherPod := range *objs {
		passes := true
		for _, p := range predicates {
			if !p(&launcherPod) {
				passes = false
				break
			}
		}
		if passes {
			match = append(match, launcherPod)
		}
	}
	return match
}
