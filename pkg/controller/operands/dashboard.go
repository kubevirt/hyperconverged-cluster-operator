package operands

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"os"
	filepath "path/filepath"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
	log "github.com/go-logr/logr"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Dashboard ConfigMaps contain json definitions of OCP UI
const (
	dashboardManifestLocationVarName = "DASHBOARD_FILES_LOCATION"
	dashboardManifestLocationDefault = "./dashboard"
)

func newDashboardHandler(Client client.Client, Scheme *runtime.Scheme, required *v1.ConfigMap) Operand {
	h := &genericOperand{
		Client: Client,
		Scheme: Scheme,
		crType: "ConfigMap",
		isCr:   false,
		// Previous versions used to have HCO-operator (scope namespace)
		// as the owner of NetworkAddons (scope cluster).
		// It's not legal, so remove that.
		removeExistingOwner: false,
		hooks:               &dashboardHooks{required: required},
	}

	return h
}

type dashboardHooks struct {
	required *v1.ConfigMap
}

func (h dashboardHooks) getFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h dashboardHooks) getEmptyCr() client.Object {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.required.Name,
		},
	}
}

func (h dashboardHooks) validate() error                                        { return nil }
func (h dashboardHooks) postFound(_ *common.HcoRequest, _ runtime.Object) error { return nil }
func (h dashboardHooks) getConditions(_ runtime.Object) []conditionsv1.Condition {
	return nil
}

func (h dashboardHooks) checkComponentVersion(_ runtime.Object) bool {
	return true
}

func (h dashboardHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*v1.ConfigMap).ObjectMeta
}

func (h dashboardHooks) reset() { /* no implementation */ }

func (h dashboardHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*v1.ConfigMap)

	if !ok {
		return false, false, errors.New("can't convert to Configmap")
	}

	if !reflect.DeepEqual(found.Data, h.required.Data) ||
		!reflect.DeepEqual(found.Labels, h.required.Labels) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Configmap to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated Configmap to its opinionated values", "name", h.required.Name)
		}
		util.DeepCopyLabels(&h.required.ObjectMeta, &found.ObjectMeta)
		h.required.DeepCopyInto(found)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func getDashboardHandlers(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) ([]Operand, error) {
	filesLocation := getDashboardDirPath()

	err := validateDashboardDir(filesLocation)
	if err != nil {
		return nil, errors.Unwrap(err) // if not wrapped, then it's not an error that stops processing, and it return nil
	}

	return createDashboardHandlersFromFiles(logger, Client, Scheme, hc, filesLocation)
}

func getDashboardDirPath() string {
	filesLocation := os.Getenv(dashboardManifestLocationVarName)
	if filesLocation == "" {
		return dashboardManifestLocationDefault
	}

	return filesLocation
}

// This function returns 3-state error:
//   err := validateDashboardDir(...)
//   err == nil - OK: dashboards directory exists
//   err != nil && errors.Unwrap(err) == nil - dashboards directory does not exist, but that ok
//   err != nil && errors.Unwrap(err) != nil - actual error
func validateDashboardDir(filesLocation string) error {
	info, err := os.Stat(filesLocation)
	if err != nil {
		if os.IsNotExist(err) { // don't return error if there is no dashboards dir, just ignore it
			return newProcessingError(nil) // return error, but don't stop processing
		}
		return newProcessingError(err)
	}

	if !info.IsDir() {
		err := fmt.Errorf("%s is not a directory", filesLocation)
		return newProcessingError(err) // return error
	}
	return nil
}

func createDashboardHandlersFromFiles(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, filesLocation string) ([]Operand, error) {
	var handlers []Operand
	err := filepath.Walk(filesLocation, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		qs, err := processDashboardConfigMapFile(path, info, logger, hc, Client, Scheme)
		if err != nil {
			return err
		}

		if qs != nil {
			handlers = append(handlers, qs)
		}

		return nil
	})

	return handlers, err
}

func processDashboardConfigMapFile(path string, info os.FileInfo, logger log.Logger, hc *hcov1beta1.HyperConverged, Client client.Client, Scheme *runtime.Scheme) (Operand, error) {
	if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
		file, err := os.Open(path)
		if err != nil {
			logger.Error(err, "Can't open the dashboard yaml file", "file name", path)
			return nil, err
		}

		cm, err := yamlToConfigMap(file)
		if err != nil {
			logger.Error(err, "Can't generate a Configmap object from yaml file", "file name", path)
		} else {
			cm.Labels = getLabels(hc, util.AppComponentCompute)
			return newDashboardHandler(Client, Scheme, cm), nil
		}
	}
	return nil, nil
}

func yamlToConfigMap(file io.Reader) (*v1.ConfigMap, error) {
	qs := &v1.ConfigMap{}

	yamlBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(yamlBytes, qs); err != nil {
		return nil, err
	}

	return qs, nil
}
