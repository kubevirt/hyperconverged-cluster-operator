package bearer_token_controller

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWebhookBearerToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Bearer Token Controller Suite")
}
