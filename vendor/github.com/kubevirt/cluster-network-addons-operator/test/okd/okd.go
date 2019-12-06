package okd

import (
	"os"
	"strings"
)

func IsOnOKDCluster() bool {
	return strings.HasPrefix(os.Getenv("KUBEVIRT_PROVIDER"), "okd-")
}
