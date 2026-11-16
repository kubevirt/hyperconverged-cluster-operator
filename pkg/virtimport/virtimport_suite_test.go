package virtimport_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVirtImport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtImport Bundle Suite")
}
