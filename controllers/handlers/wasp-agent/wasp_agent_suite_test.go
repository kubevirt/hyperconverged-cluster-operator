package wasp_agent

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWaspAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wasp Agent Suite")
}
