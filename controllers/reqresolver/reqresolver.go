package reqresolver

import (
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	secondaryCRPrefix = "hco-controlled-cr-"
	apiServerCRPrefix = "api-server-cr-"
)

var (
	randomConstSuffix = ""

	// hyperConvergedNamespacedName is the name/namespace of the HyperConverged resource
	hyperConvergedNamespacedName types.NamespacedName

	// secondaryCRPlaceholder is a placeholder to be able to discriminate
	// reconciliation requests triggered by secondary watched resources
	// use a random generated suffix for security reasons
	secondaryCRPlaceholder types.NamespacedName

	apiServerCRPlaceholder types.NamespacedName
)

// ResolveReconcileRequest returns a reconcile.Request to be used throughout the reconciliation cycle,
// regardless of which resource has triggered it.
func ResolveReconcileRequest(logger logr.Logger, originalRequest reconcile.Request) (reconcile.Request, bool) {
	if isTriggeredByHyperConvergedOrUnknown(originalRequest) {
		logger.Info("Reconciling HyperConverged operator")
		return originalRequest, true
	}

	resolvedRequest := getHyperConvergedCRRequest()

	if IsTriggeredByAPIServerCR(originalRequest) {
		// consider a change in APIServerCr like a change in HCO
		return resolvedRequest, true
	}

	logger.Info("The reconciliation got triggered by a secondary CR object")
	return resolvedRequest, false
}

func GetHyperConvergedNamespacedName() types.NamespacedName {
	return hyperConvergedNamespacedName
}

func getHyperConvergedCRRequest() reconcile.Request {
	return reconcile.Request{
		NamespacedName: hyperConvergedNamespacedName,
	}
}

func GetSecondaryCRRequest() reconcile.Request {
	return reconcile.Request{
		NamespacedName: secondaryCRPlaceholder,
	}
}

func GetAPIServerCRRequest() reconcile.Request {
	return reconcile.Request{
		NamespacedName: apiServerCRPlaceholder,
	}
}

func IsTriggeredByHyperConverged(nsName types.NamespacedName) bool {
	return nsName == hyperConvergedNamespacedName
}

func isTriggeredByHyperConvergedOrUnknown(request reconcile.Request) bool {
	return request.NamespacedName != secondaryCRPlaceholder && request.NamespacedName != apiServerCRPlaceholder
}

func IsTriggeredByAPIServerCR(request reconcile.Request) bool {
	return request.NamespacedName == apiServerCRPlaceholder
}

func GeneratePlaceHolders() {
	randomConstSuffix = uuid.New().String()

	ns := hcoutil.GetOperatorNamespaceFromEnv()
	hyperConvergedNamespacedName = types.NamespacedName{
		Name:      hcoutil.HyperConvergedName,
		Namespace: ns,
	}

	secondaryCRPlaceholder = types.NamespacedName{
		Name:      secondaryCRPrefix + randomConstSuffix,
		Namespace: ns,
	}

	apiServerCRPlaceholder = types.NamespacedName{
		Name:      apiServerCRPrefix + randomConstSuffix,
		Namespace: ns,
	}
}

func init() {
	GeneratePlaceHolders()
}
