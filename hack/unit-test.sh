#!/bin/bash

set -euxo pipefail

go test ./api/v1beta1 ./cmd/... ./pkg/... ./controllers/...
