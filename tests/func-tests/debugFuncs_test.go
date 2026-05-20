package tests

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

// printHyperConverged returns a function to print the HyperConverged CR in JSOn format. It is a lazy initialization,
// so it won't be called unless there is a failure and the output is needed.
func marshalHyperConverged(hc *hcov1.HyperConverged) string {
	ginkgo.GinkgoHelper()

	if hc == nil {
		return "<nil>"
	}

	hcCopy := hc.DeepCopy()
	hcCopy.ManagedFields = nil // remove noise

	hcYAML, err := yaml.Marshal(hcCopy)
	Expect(err).NotTo(HaveOccurred())

	return string(hcYAML)
}

// PrintHyperConverged returns a function to print the HyperConverged CR in JSOn format. It is a lazy initialization,
// so it won't be called unless there is a failure and the output is needed.
func PrintHyperConverged(hc *hcov1.HyperConverged) func() string {
	return func() string {
		hcYAML := marshalHyperConverged(hc)
		return fmt.Sprintf("Current HyperConverged CR:\n%s\n", hcYAML)
	}
}

// PrintHyperConvergedBecause returns a function to print the HyperConverged CR in JSOn format. It is a lazy initialization,
// so it won't be called unless there is a failure and the output is needed.
func PrintHyperConvergedBecause(hc *hcov1.HyperConverged, format string, args ...any) func() string {
	return func() string {
		format += "; current HyperConverged CR:\n%s\n"
		hcYAML := marshalHyperConverged(hc)
		args = append(args, hcYAML)

		return fmt.Sprintf(format, args...)
	}
}

// PrintOrigAndCurrentHyperConvergeds returns a function to print two phases of the HyperConverged CR in JSOn
// format: one before the test. and one as it is now. It is a lazy initialization, so it won't be called unless there is
// a failure and the output is needed.
func PrintOrigAndCurrentHyperConvergeds(orig, modified *hcov1.HyperConverged) func() string {
	return func() string {
		origHCStr := marshalHyperConverged(orig)
		modifiedHCStr := marshalHyperConverged(modified)

		return fmt.Sprintf("Original HC CR:\n%s\nModified HC CR:\n%s\n", origHCStr, modifiedHCStr)
	}
}
