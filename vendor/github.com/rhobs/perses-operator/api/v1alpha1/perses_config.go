package v1alpha1

import (
	"fmt"

	"github.com/brunoga/deep"
	"github.com/rhobs/perses/pkg/model/api/config"
)

type PersesConfig struct {
	config.Config `json:",inline"`
}

func (in *PersesConfig) DeepCopyInto(out *PersesConfig) {
	if in == nil {
		return
	}

	copied, err := deep.Copy(in)
	if err != nil {
		panic(fmt.Errorf("failed to deep copy PersesConfig: %w", err))
	}
	*out = *copied
}
