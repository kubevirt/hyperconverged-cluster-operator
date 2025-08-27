package handlers

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"strings"

	log "github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// Dashboard ConfigMaps contain json definitions of OCP UI
const (
	DashboardManifestLocation = "dashboard"
)

func GetDashboardHandlers(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, dir fs.FS) ([]operands.Operand, error) {
	err := util.ValidateManifestDir(DashboardManifestLocation, dir)
	if err != nil {
		return nil, errors.Unwrap(err) // if not wrapped, then it's not an error that stops processing, and it return nil
	}

	return createDashboardHandlersFromFiles(logger, Client, Scheme, hc, DashboardManifestLocation, dir)
}

func createDashboardHandlersFromFiles(logger log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, filesLocation string, dir fs.FS) ([]operands.Operand, error) {
	var handlers []operands.Operand
	err := fs.WalkDir(dir, filesLocation, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			return nil
		}

		qs, err := processDashboardConfigMapFile(path, dir, logger, hc, Client, Scheme)
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

func processDashboardConfigMapFile(path string, dir fs.FS, logger log.Logger, hc *hcov1beta1.HyperConverged, Client client.Client, Scheme *runtime.Scheme) (operands.Operand, error) {
	file, err := dir.Open(path)
	if err != nil {
		logger.Error(err, "Can't open the dashboard yaml file", "file name", path)
		return nil, err
	}

	cm, err := cmFromFile(file)
	if err != nil {
		logger.Error(err, "Can't generate a Configmap object from yaml file", "file name", path)
	} else {
		maps.Copy(cm.Labels, operands.GetLabels(hc, util.AppComponentCompute))
		return operands.NewCmHandler(Client, Scheme, cm), nil
	}

	return nil, nil
}

func cmFromFile(reader io.Reader) (*v1.ConfigMap, error) {
	cm := &v1.ConfigMap{}
	err := util.UnmarshalYamlFileToObject(reader, cm)

	if err != nil {
		return nil, err
	}

	return cm, nil
}
