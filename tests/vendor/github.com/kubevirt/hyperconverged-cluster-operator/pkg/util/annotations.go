package util

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func MergeAnnotations(src, tgt *metav1.ObjectMeta) {
	if src.Annotations == nil {
		return
	}

	tgt.Annotations = make(map[string]string, len(src.Annotations))
	for key, val := range src.Annotations {
		tgt.Annotations[key] = val
	}
}
