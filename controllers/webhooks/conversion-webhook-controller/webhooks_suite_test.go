package conversion_webhook_controller

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConversionWebhookController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conversion Webhook Controller Suite")
}
