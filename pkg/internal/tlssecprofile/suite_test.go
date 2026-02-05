package tlssecprofile_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTLSSecurityProfile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS Security Profile Suite")
}
