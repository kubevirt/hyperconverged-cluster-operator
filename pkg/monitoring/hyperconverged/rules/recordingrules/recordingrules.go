package recordingrules

import "github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

func Register(operatorRegistry *operatorrules.Registry) error {
	return operatorRegistry.RegisterRecordingRules(
		operatorRecordingRules,
	)
}
