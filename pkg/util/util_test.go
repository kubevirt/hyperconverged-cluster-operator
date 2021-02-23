package util

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestControllerCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HCO Util Suite")
}
