package handlers

import (
	"errors"
	"io"
	"io/fs"
	"reflect"
	"strings"

	log "github.com/go-logr/logr"
	consolev1 "github.com/openshift/api/console/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// ConsoleQuickStart resources are a short user guids
const (
	QuickStartDefaultManifestLocation = "quickStart"
)

var quickstartNames []string

func GetQuickStartNames() []string {
	return quickstartNames
}

func newQuickStartHandler(Client client.Client, Scheme *runtime.Scheme, required *consolev1.ConsoleQuickStart) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "ConsoleQuickStart", &qsHooks{required: required}, false)
}

type qsHooks struct {
	required *consolev1.ConsoleQuickStart
}

func (h qsHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h qsHooks) GetEmptyCr() client.Object {
	return &consolev1.ConsoleQuickStart{}
}

func (h qsHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*consolev1.ConsoleQuickStart)

	if !ok {
		return false, false, errors.New("can't convert to ConsoleQuickStart")
	}

	if !reflect.DeepEqual(h.required.Spec, found.Spec) ||
		!util.CompareLabels(h.required, found) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing ConsoleQuickStart's Spec to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated ConsoleQuickStart's Spec to its opinionated values", "name", h.required.Name)
		}
		util.MergeLabels(&h.required.ObjectMeta, &found.ObjectMeta)
		h.required.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func GetQuickStartHandlers(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, dir fs.FS) ([]operands.Operand, error) {
	err := util.ValidateManifestDir(QuickStartDefaultManifestLocation, dir)
	if err != nil {
		return nil, errors.Unwrap(err) // if not wrapped, then it's not an error that stops processing, and it return nil
	}

	return createQuickstartHandlersFromFiles(logger, Client, Scheme, hc, QuickStartDefaultManifestLocation, dir)
}

func createQuickstartHandlersFromFiles(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, filesLocation string, dir fs.FS) ([]operands.Operand, error) {
	var handlers []operands.Operand
	quickstartNames = []string{}

	err := fs.WalkDir(dir, filesLocation, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			return nil
		}

		qs, err := processQuickstartFile(path, logger, hc, Client, Scheme, dir)
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

func processQuickstartFile(path string, logger log.Logger, hc *hcov1beta1.HyperConverged, Client client.Client, Scheme *runtime.Scheme, dir fs.FS) (operands.Operand, error) {
	file, err := dir.Open(path)
	if err != nil {
		logger.Error(err, "Can't open the quickStart yaml file", "file name", path)
		return nil, err
	}

	qs, err := quickStartFromFile(file)
	if err != nil {
		logger.Error(err, "Can't generate a ConsoleQuickStart object from yaml file", "file name", path)
	} else {
		qs.Labels = operands.GetLabels(hc, util.AppComponentCompute)
		quickstartNames = append(quickstartNames, qs.Name)
		return newQuickStartHandler(Client, Scheme, qs), nil
	}

	return nil, nil
}

func quickStartFromFile(reader io.Reader) (*consolev1.ConsoleQuickStart, error) {
	qs := &consolev1.ConsoleQuickStart{}
	err := util.UnmarshalYamlFileToObject(reader, qs)

	if err != nil {
		return nil, err
	}

	return qs, nil
}
