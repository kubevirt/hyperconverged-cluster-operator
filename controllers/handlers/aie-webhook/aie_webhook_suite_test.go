package aie_webhook

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAIEWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AIE Webhook Suite")
}
