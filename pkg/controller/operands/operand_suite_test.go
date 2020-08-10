package operands

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOperands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operands Suite")
}
