package observabilitycontroller

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	log "github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	guestDeviceDriversConfigMapName    = "kubevirt-guest-device-drivers-config"
	guestDeviceDriversConfigMapKey     = "drivers.yaml"
	GuestDeviceDriversManifestLocation = "guestDeviceDrivers"
)

func GetGuestDeviceDriversHandlers(logger log.Logger, cli client.Client, scheme *runtime.Scheme, _ *hcov1.HyperConverged, dir fs.FS) ([]operands.Operand, error) {
	content, err := readGuestDeviceDriversConfig(dir)
	if err != nil {
		logger.Error(err, "can't read the guest device drivers config file",
			"file", path.Join(GuestDeviceDriversManifestLocation, guestDeviceDriversConfigMapKey))
		return nil, err
	}

	if strings.TrimSpace(content) == "" {
		logger.Info("guest device drivers config file not found; the reference ConfigMap will not be deployed")
		return nil, nil
	}

	logger.Info("guest device drivers config found; deploying the reference ConfigMap")
	return []operands.Operand{newGuestDeviceDriversConfigMapHandler(cli, scheme, content)}, nil
}

func readGuestDeviceDriversConfig(dir fs.FS) (string, error) {
	content, err := fs.ReadFile(dir, path.Join(GuestDeviceDriversManifestLocation, guestDeviceDriversConfigMapKey))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	return string(content), nil
}

func newGuestDeviceDriversConfigMapHandler(cli client.Client, scheme *runtime.Scheme, content string) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewCmHandler(cli, scheme, newGuestDeviceDriversConfigMap(content)),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return newGuestDeviceDriversConfigMapWithNameOnly()
		},
	)
}

func newGuestDeviceDriversConfigMapWithNameOnly() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      guestDeviceDriversConfigMapName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}

func newGuestDeviceDriversConfigMap(content string) *corev1.ConfigMap {
	cm := newGuestDeviceDriversConfigMapWithNameOnly()
	cm.Data = map[string]string{
		guestDeviceDriversConfigMapKey: content,
	}
	return cm
}
