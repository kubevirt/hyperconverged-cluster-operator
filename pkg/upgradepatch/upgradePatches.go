package upgradepatch

import (
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/go-logr/logr"
	"os"
	"strings"

	"github.com/blang/semver/v4"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

//go:generate go run ../../tools/crwriter/ --format=json --out=./hc.cr.json
//go:embed hc.cr.json
var hcCRBytes []byte

const (
	upgradeChangesFileLocation = "./upgradePatches.json"
)

type HcoCRPatch struct {
	// SemverRange is a set of conditions which specify which versions satisfy the range
	// (see https://github.com/blang/semver#ranges as a reference).
	SemverRange string `json:"semverRange"`
	// JSONPatch contains a sequence of operations to apply to the HCO CR during upgrades
	// (see: https://datatracker.ietf.org/doc/html/rfc6902 as the format reference).
	JSONPatch jsonpatch.Patch `json:"jsonPatch"`

	// jsonPatchApplyOptions specifies options for calls to ApplyWithOptions.
	// jsonpatch.NewApplyOptions defaults are applied if empty.
	JSONPatchApplyOptions *jsonpatch.ApplyOptions `json:"jsonPatchApplyOptions,omitempty"`
}

type ObjectToBeRemoved struct {
	// SemverRange is a set of conditions which specify which versions satisfy the range
	// (see https://github.com/blang/semver#ranges as a reference).
	SemverRange string `json:"semverRange"`
	// GroupVersionKind unambiguously identifies the kind of the object to be removed
	GroupVersionKind schema.GroupVersionKind `json:"groupVersionKind"`
	// objectKey contains name and namespace of the object to be removed.
	ObjectKey types.NamespacedName `json:"objectKey"`
}

type UpgradePatches struct {
	// hcoCRPatchList is a list of upgrade patches.
	// Each hcoCRPatch consists in a semver range of affected source versions and a json patch to be applied during the upgrade if relevant.
	HCOCRPatchList []HcoCRPatch `json:"hcoCRPatchList"`
	// ObjectsToBeRemoved is a list of objects to be removed on upgrades.
	// Each objectToBeRemoved consists in a semver range of affected source versions and schema.GroupVersionKind and types.NamespacedName of the object to be eventually removed during the upgrade.
	ObjectsToBeRemoved []ObjectToBeRemoved `json:"objectsToBeRemoved"`
}

var (
	hcoUpgradeChanges     UpgradePatches
	hcoUpgradeChangesRead = false
)

func GetHCOCRPatchList() []HcoCRPatch {
	return hcoUpgradeChanges.HCOCRPatchList
}

func GetObjectsToBeRemoved() []ObjectToBeRemoved {
	return hcoUpgradeChanges.ObjectsToBeRemoved
}

var getUpgradeChangesFileLocation = func() string {
	return upgradeChangesFileLocation
}

func readUpgradePatchesFromFile(logger logr.Logger) error {
	if hcoUpgradeChangesRead {
		return nil
	}
	hcoUpgradeChanges = UpgradePatches{}
	fileLocation := getUpgradeChangesFileLocation()

	file, err := os.Open(fileLocation)
	if err != nil {
		logger.Error(err, "Can't open the upgradeChanges yaml file", "file name", fileLocation)
		return err
	}

	jDec := json.NewDecoder(file)
	err = jDec.Decode(&hcoUpgradeChanges)
	if err != nil {
		return err
	}

	hcoUpgradeChangesRead = true
	return nil
}

func ValidateUpgradePatches(logger logr.Logger) error {
	err := readUpgradePatchesFromFile(logger)
	if err != nil {
		return err
	}
	for _, p := range hcoUpgradeChanges.HCOCRPatchList {
		if verr := validateUpgradePatch(p); verr != nil {
			return verr
		}
	}
	for _, r := range hcoUpgradeChanges.ObjectsToBeRemoved {
		if verr := validateUpgradeLeftover(r); verr != nil {
			return verr
		}
	}
	return nil
}

func validateUpgradePatch(p HcoCRPatch) error {
	_, err := semver.ParseRange(p.SemverRange)
	if err != nil {
		return err
	}

	for _, patch := range p.JSONPatch {
		path, err := patch.Path()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, "/spec/") {
			return errors.New("can only modify spec fields")
		}
	}

	if p.JSONPatchApplyOptions != nil {
		_, err = p.JSONPatch.ApplyWithOptions(hcCRBytes, p.JSONPatchApplyOptions)
	} else {
		_, err = p.JSONPatch.Apply(hcCRBytes)
	}
	// tolerate jsonpatch test failures
	if err != nil && !errors.Is(errors.Unwrap(err), jsonpatch.ErrTestFailed) {
		return err
	}
	return nil
}

func validateUpgradeLeftover(r ObjectToBeRemoved) error {
	_, err := semver.ParseRange(r.SemverRange)
	if err != nil {
		return err
	}

	if r.GroupVersionKind.Kind == "" {
		return errors.New("missing object kind")
	}
	if r.GroupVersionKind.Version == "" {
		return errors.New("missing object API version")
	}
	if r.ObjectKey.Name == "" {
		return errors.New("missing object name")
	}
	return nil
}
