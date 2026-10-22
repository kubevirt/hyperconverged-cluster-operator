package observabilitycontroller

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

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
	guestDeviceDriversManifestLocation = "guestDeviceDrivers/" + guestDeviceDriversConfigMapKey
)

var guestDeviceDriversConfig string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic("can't get the working directory; " + err.Error())
	}

	if err := readGuestDeviceDriversConfig(os.DirFS(wd)); err != nil {
		panic(fmt.Errorf("can't read the guest device drivers config file: %w", err))
	}
}

func readGuestDeviceDriversConfig(wdFS fs.FS) error {
	content, err := fs.ReadFile(wdFS, guestDeviceDriversManifestLocation)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			guestDeviceDriversConfig = ""
			return nil
		}
		return err
	}

	guestDeviceDriversConfig = string(content)
	return nil
}

func GuestDeviceDriverExists() bool {
	return strings.TrimSpace(guestDeviceDriversConfig) != ""
}

func NewGuestDeviceDriversConfigMapHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewCmHandler(cli, scheme, newGuestDeviceDriversConfigMap()),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return NewGuestDeviceDriversConfigMapWithNameOnly()
		},
	)
}

func NewGuestDeviceDriversConfigMapWithNameOnly() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      guestDeviceDriversConfigMapName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}

func newGuestDeviceDriversConfigMap() *corev1.ConfigMap {
	cm := NewGuestDeviceDriversConfigMapWithNameOnly()
	cm.Data = map[string]string{
		guestDeviceDriversConfigMapKey: guestDeviceDriversConfig,
	}
	return cm
}
