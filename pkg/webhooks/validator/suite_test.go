package validator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestValidatorWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Webhooks Suite")
}
