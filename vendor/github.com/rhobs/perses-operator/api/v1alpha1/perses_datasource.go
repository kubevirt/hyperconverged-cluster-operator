package v1alpha1

import (
	"fmt"

	"github.com/brunoga/deep"
	persesv1 "github.com/rhobs/perses/pkg/model/api/v1"
)

type Datasource struct {
	persesv1.DatasourceSpec `json:",inline"`
}

func (in *Datasource) DeepCopyInto(out *Datasource) {
	if in == nil {
		return
	}

	copied, err := deep.Copy(in)
	if err != nil {
		panic(fmt.Errorf("failed to deep copy Datasource: %w", err))
	}
	*out = *copied
}
