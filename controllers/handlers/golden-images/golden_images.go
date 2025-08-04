package golden_images

import (
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	dataImportCronTemplatesFileLocation = "./dataImportCronTemplates"

	CDIImmediateBindAnnotation = "cdi.kubevirt.io/storage.bind.immediate.requested"

	MultiArchDICTAnnotation = "ssp.kubevirt.io/dict.architectures"

	DictConditionDeployedType    = "Deployed"
	dictConditionDeployedReason  = "UnsupportedArchitectures"
	dictConditionDeployedMessage = "DataImportCronTemplate has no supported architectures for the current cluster"
)

var (
	// dataImportCronTemplateHardCodedMap are set of data import cron template configurations. The handler reads a list
	// of data import cron templates from a local file and updates SSP with the up-to-date list
	dataImportCronTemplateHardCodedMap map[string]hcov1beta1.DataImportCronTemplate

	getDataImportCronTemplatesFileLocation = func() string {
		return dataImportCronTemplatesFileLocation
	}

	logger = logf.Log.WithName("dataImportCronTemplateInit")
)

var GetDataImportCronTemplates = func(hc *hcov1beta1.HyperConverged) ([]hcov1beta1.DataImportCronTemplateStatus, error) {
	crDicts, err := getDicMapFromCr(hc)
	if err != nil {
		return nil, err
	}

	var dictList []hcov1beta1.DataImportCronTemplateStatus
	if ptr.Deref(hc.Spec.EnableCommonBootImageImport, true) {
		dictList = getCommonDicts(dictList, crDicts, hc)
	}
	dictList = getCustomDicts(dictList, crDicts)

	if hc.Spec.FeatureGates.EnableMultiArchBootImageImport != nil && *hc.Spec.FeatureGates.EnableMultiArchBootImageImport {
		for i := range dictList {
			setDataImportCronTemplateStatusMultiArch(&dictList[i], nodeinfo.GetWorkloadsArchitectures())
		}
	}

	sort.Sort(dataImportTemplateSlice(dictList))

	return dictList, nil
}

func CheckDataImportCronTemplates(hc *hcov1beta1.HyperConverged) {
	multiArchEnabled := ptr.Deref(hc.Spec.FeatureGates.EnableMultiArchBootImageImport, false)

	for _, dict := range hc.Status.DataImportCronTemplates {
		if multiArchEnabled && meta.IsStatusConditionFalse(dict.Status.Conditions, DictConditionDeployedType) {
			metrics.SetDICTWithNoSupportedArchitectures(dict.Name, dict.Spec.ManagedDataSource)
			continue
		}

		metrics.SetDICTWithSupportedArchitectures(dict.Name, dict.Spec.ManagedDataSource)
	}
}

func HCODictSliceToSSP(hc *hcov1beta1.HyperConverged, hcoDictStatuses []hcov1beta1.DataImportCronTemplateStatus) []sspv1beta3.DataImportCronTemplate {
	return slices.Collect(hcoDictToSSPSeq(hc, slices.Values(hcoDictStatuses)))
}

func ApplyDataImportSchedule(hc *hcov1beta1.HyperConverged) {
	if hc.Status.DataImportSchedule != "" {
		overrideDataImportSchedule(hc.Status.DataImportSchedule)
	}
}

func init() {
	if err := readDataImportCronTemplatesFromFile(); err != nil {
		panic(fmt.Errorf("can't process the data import cron template file; %s; %w", err.Error(), err))
	}
}

func readDataImportCronTemplatesFromFile() error {
	dataImportCronTemplateHardCodedMap = make(map[string]hcov1beta1.DataImportCronTemplate)

	fileLocation := getDataImportCronTemplatesFileLocation()

	err := util.ValidateManifestDir(fileLocation)
	if err != nil {
		return errors.Unwrap(err) // if not wrapped, then it's not an error that stops processing, and it returns nil
	}

	return filepath.Walk(fileLocation, func(filePath string, info fs.FileInfo, internalErr error) error {
		if internalErr != nil {
			return internalErr
		}

		if !info.IsDir() && path.Ext(info.Name()) == ".yaml" {
			file, internalErr := os.Open(filePath)
			if internalErr != nil {
				logger.Error(internalErr, "Can't open the dataImportCronTemplate yaml file", "file name", filePath)
				return internalErr
			}

			dataImportCronTemplateFromFile := make([]hcov1beta1.DataImportCronTemplate, 0)
			internalErr = util.UnmarshalYamlFileToObject(file, &dataImportCronTemplateFromFile)
			if internalErr != nil {
				return internalErr
			}

			for _, dict := range dataImportCronTemplateFromFile {
				dataImportCronTemplateHardCodedMap[dict.Name] = dict
			}
		}

		return nil
	})
}

func getCommonDicts(list []hcov1beta1.DataImportCronTemplateStatus, crDicts map[string]hcov1beta1.DataImportCronTemplate, hc *hcov1beta1.HyperConverged) []hcov1beta1.DataImportCronTemplateStatus {
	enableMultiArchBootImageImport := ptr.Deref(hc.Spec.FeatureGates.EnableMultiArchBootImageImport, false)
	for dictName, commonDict := range dataImportCronTemplateHardCodedMap {
		targetDict := hcov1beta1.DataImportCronTemplateStatus{
			DataImportCronTemplate: *commonDict.DeepCopy(),
			Status: hcov1beta1.DataImportCronStatus{
				CommonTemplate: true,
			},
		}

		if crDict, found := crDicts[dictName]; found {
			if !customizeCommonDICT(&targetDict, crDict, enableMultiArchBootImageImport) {
				continue
			}
		} else if ns := hc.Spec.CommonBootImageNamespace; ns != nil && len(*ns) > 0 {
			targetDict.Namespace = *ns
		}

		list = append(list, targetDict)
	}

	return list
}

func customizeCommonDICT(targetDict *hcov1beta1.DataImportCronTemplateStatus, crDict hcov1beta1.DataImportCronTemplate, enableMultiArchBootImageImport bool) bool {
	if !isDataImportCronTemplateEnabled(crDict) {
		return false
	}

	// if the schedule is missing, copy from the common dict:
	if len(crDict.Spec.Schedule) == 0 {
		crDict.Spec.Schedule = targetDict.Spec.Schedule
	}

	customizeCommonDictAnnotations(targetDict, crDict, enableMultiArchBootImageImport)

	targetDict.Spec = crDict.Spec.DeepCopy()
	targetDict.Namespace = crDict.Namespace
	targetDict.Status.Modified = true

	return true
}

// customizeCommonDictAnnotations updates the annotations of the target DICT, with a special handling of the MultiArch
// DICT Annotation:
//
// if DICT registry was not customized, use the original common DICT annotation in the result DICT,
// or if it's missing from the common DICT, remove it from the result DICT.
//
// if DICT registry was customized, use the customized DICT annotation in the result DICT, or if it's
// missing from the customized DICT, remove it from the result DITC.
func customizeCommonDictAnnotations(targetDict *hcov1beta1.DataImportCronTemplateStatus, crDict hcov1beta1.DataImportCronTemplate, enableMultiArchBootImageImport bool) {
	registryModified := crDict.Spec.Template.Spec.Source.Registry != nil &&
		!reflect.DeepEqual(crDict.Spec.Template.Spec.Source.Registry, targetDict.Spec.Template.Spec.Source.Registry)
	crDictAnnotations := maps.Clone(crDict.Annotations)

	if crDictAnnotations != nil {
		if enableMultiArchBootImageImport && !registryModified {
			adoptOrigCommonDictAnnotation(targetDict, crDictAnnotations)
		}
		copyOrCloneMap(&targetDict.Annotations, crDictAnnotations)
	}
	if enableMultiArchBootImageImport && registryModified {
		adoptCRDictAnnotation(targetDict, crDictAnnotations)
	}
}

func adoptOrigCommonDictAnnotation(targetDict *hcov1beta1.DataImportCronTemplateStatus, crDictAnnotations map[string]string) {
	multiArchDICTAnnotation, exists := targetDict.Annotations[MultiArchDICTAnnotation]
	if !exists {
		delete(crDictAnnotations, MultiArchDICTAnnotation)
	} else {
		// If the MultiArchDICTAnnotation annotation exists in target, keep it
		crDictAnnotations[MultiArchDICTAnnotation] = multiArchDICTAnnotation
	}
}

func copyOrCloneMap(dst *map[string]string, src map[string]string) {
	if *dst == nil {
		*dst = maps.Clone(src)
	} else {
		maps.Copy(*dst, src)
	}
}

func adoptCRDictAnnotation(targetDict *hcov1beta1.DataImportCronTemplateStatus, crDictAnnotations map[string]string) {
	_, ext := crDictAnnotations[MultiArchDICTAnnotation]
	if !ext {
		delete(targetDict.Annotations, MultiArchDICTAnnotation)
	}
}

func isDataImportCronTemplateEnabled(dict hcov1beta1.DataImportCronTemplate) bool {
	annotationVal, found := dict.Annotations[util.DataImportCronEnabledAnnotation]
	return !found || strings.ToLower(annotationVal) == "true"
}

func getCustomDicts(list []hcov1beta1.DataImportCronTemplateStatus, crDicts map[string]hcov1beta1.DataImportCronTemplate) []hcov1beta1.DataImportCronTemplateStatus {
	for dictName, crDict := range crDicts {
		if !isDataImportCronTemplateEnabled(crDict) {
			continue
		}

		if _, isCommon := dataImportCronTemplateHardCodedMap[dictName]; !isCommon {
			list = append(list, hcov1beta1.DataImportCronTemplateStatus{
				DataImportCronTemplate: *crDict.DeepCopy(),
				Status: hcov1beta1.DataImportCronStatus{
					CommonTemplate: false,
				},
			})
		}
	}

	return list
}

func getDicMapFromCr(hc *hcov1beta1.HyperConverged) (map[string]hcov1beta1.DataImportCronTemplate, error) {
	dictMap := make(map[string]hcov1beta1.DataImportCronTemplate)
	for _, dict := range hc.Spec.DataImportCronTemplates {
		_, foundCustom := dictMap[dict.Name]
		if foundCustom {
			return nil, fmt.Errorf("%s DataImportCronTable is already defined", dict.Name)
		}
		dictMap[dict.Name] = dict
	}
	return dictMap, nil
}

func overrideDataImportSchedule(schedule string) {
	for dictName := range dataImportCronTemplateHardCodedMap {
		dict := dataImportCronTemplateHardCodedMap[dictName]
		dict.Spec.Schedule = schedule
		dataImportCronTemplateHardCodedMap[dictName] = dict
	}
}

// implement sort.Interface
type dataImportTemplateSlice []hcov1beta1.DataImportCronTemplateStatus

func (d dataImportTemplateSlice) Len() int           { return len(d) }
func (d dataImportTemplateSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d dataImportTemplateSlice) Less(i, j int) bool { return d[i].Name < d[j].Name }

func setDataImportCronTemplateStatusMultiArch(hcoDictStatus *hcov1beta1.DataImportCronTemplateStatus, workloadsArchs []string) {
	hcoArchsAnnotation, hcoArchsAnnotationExists := hcoDictStatus.Annotations[MultiArchDICTAnnotation]
	if !hcoArchsAnnotationExists {
		return
	}

	sspArchsAnnotation := removeUnsupportedArchs(hcoArchsAnnotation, workloadsArchs)
	if sspArchsAnnotation == "" {
		meta.SetStatusCondition(&hcoDictStatus.Status.Conditions, metav1.Condition{
			Type:    DictConditionDeployedType,
			Status:  metav1.ConditionFalse,
			Reason:  dictConditionDeployedReason,
			Message: dictConditionDeployedMessage,
		})
	} else {
		meta.RemoveStatusCondition(&hcoDictStatus.Status.Conditions, DictConditionDeployedType)
		if hcoDictStatus.Annotations == nil {
			hcoDictStatus.Annotations = make(map[string]string)
		}
	}
	hcoDictStatus.Annotations[MultiArchDICTAnnotation] = sspArchsAnnotation
	hcoDictStatus.Status.OriginalSupportedArchitectures = hcoArchsAnnotation
}

func hcoDictToSSP(hcoDictStatus hcov1beta1.DataImportCronTemplateStatus, multiArchEnabled bool) (sspv1beta3.DataImportCronTemplate, bool) {
	hcoDict := hcoDictStatus.DataImportCronTemplate
	if multiArchEnabled && meta.IsStatusConditionFalse(hcoDictStatus.Status.Conditions, DictConditionDeployedType) {
		// if the condition is false, it means that the DataImportCronTemplate has no supported architectures
		// for the current cluster, so we skip it
		return sspv1beta3.DataImportCronTemplate{}, false
	}

	spec := cdiv1beta1.DataImportCronSpec{}
	if hcoDict.Spec != nil {
		hcoDict.Spec.DeepCopyInto(&spec)
	}

	dict := sspv1beta3.DataImportCronTemplate{
		ObjectMeta: *hcoDict.ObjectMeta.DeepCopy(),
		Spec:       spec,
	}

	if dict.Annotations == nil {
		dict.Annotations = make(map[string]string)
	}

	if _, foundAnnotation := dict.Annotations[CDIImmediateBindAnnotation]; !foundAnnotation {
		dict.Annotations[CDIImmediateBindAnnotation] = "true"
	}

	if !multiArchEnabled {
		delete(dict.Annotations, MultiArchDICTAnnotation)
	}

	return dict, true
}

func hcoDictToSSPSeq(hc *hcov1beta1.HyperConverged, hcoDicts iter.Seq[hcov1beta1.DataImportCronTemplateStatus]) iter.Seq[sspv1beta3.DataImportCronTemplate] {
	multiArchEnabled := ptr.Deref(hc.Spec.FeatureGates.EnableMultiArchBootImageImport, false)

	return func(yield func(sspv1beta3.DataImportCronTemplate) bool) {
		for hcoDict := range hcoDicts {
			sspDict, valid := hcoDictToSSP(hcoDict, multiArchEnabled)
			if valid && !yield(sspDict) {
				return
			}
		}
	}
}

func removeUnsupportedArchs(archAnnotation string, workloadsArchs []string) string {
	var newArchList []string

	for _, arch := range strings.Split(archAnnotation, ",") {
		if slices.Contains(workloadsArchs, arch) {
			newArchList = append(newArchList, arch)
		}
	}

	return strings.Join(newArchList, ",")
}
