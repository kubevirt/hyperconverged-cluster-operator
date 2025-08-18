package collectors

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCollectors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Collectors Suite")
}
