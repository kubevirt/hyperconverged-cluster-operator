#!/bin/bash

set -euxo pipefail

go tool ginkgo ./api/... ./pkg/... ./controllers/... ./cmd/... ./tools/manifests
