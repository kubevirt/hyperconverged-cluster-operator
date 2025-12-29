package v1alpha1

import (
	"fmt"

	"github.com/brunoga/deep"
	persesv1 "github.com/rhobs/perses/pkg/model/api/v1"
)

type Dashboard struct {
	persesv1.DashboardSpec `json:",inline"`
}

func (in *Dashboard) DeepCopyInto(out *Dashboard) {
	if in == nil {
		return
	}

	copied, err := deep.Copy(in)
	if err != nil {
		panic(fmt.Errorf("failed to deep copy Dashboard: %w", err))
	}
	*out = *copied
}
