package cmdcommon

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdcommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cmdcommon Suite")
}
