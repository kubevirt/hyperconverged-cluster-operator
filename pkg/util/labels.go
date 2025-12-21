package util

import (
	"encoding/json"
	"maps"

	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MergeLabels merges src labels into tgt ones.
func MergeLabels(src, tgt *metav1.ObjectMeta) {
	if src.Labels == nil {
		return
	}

	if tgt.Labels == nil {
		tgt.Labels = make(map[string]string, len(src.Labels))
	}

	maps.Copy(tgt.Labels, src.Labels)
}

// MergeLabelsJSONPatch merges src labels into tgt ones.
func MergeLabelsJSONPatch(src, tgt *metav1.ObjectMeta) ([]byte, error) {
	if CompareLabels(src, tgt) {
		return nil, nil
	}

	op := "replace"
	if tgt.Labels == nil {
		op = "add"
	}

	tgt = tgt.DeepCopy()
	MergeLabels(src, tgt)

	patch := []jsonpatch.Operation{jsonpatch.NewOperation(op, "/metadata/labels", tgt.Labels)}

	return json.Marshal(patch)
}

// CompareLabels reports whether src labels are contained into tgt ones; extra labels on tgt are ignored.
// It returns true if the src labels map is a subset of the tgt one.
func CompareLabels(src, tgt metav1.Object) bool {
	return compareLabels(src.GetLabels(), tgt.GetLabels())
}

func compareLabels(src, tgt map[string]string) bool {
	for srcKey, srcVal := range src {
		tgtVal, ok := tgt[srcKey]
		if !ok || tgtVal != srcVal {
			return false
		}
	}
	return true
}
