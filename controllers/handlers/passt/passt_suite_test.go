package passt_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPasst(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Passt Suite")
}
