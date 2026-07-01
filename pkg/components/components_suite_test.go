package components

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestComponents(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Components Suite")
}
