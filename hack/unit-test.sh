#!/bin/bash

set -euxo pipefail

go test ./api/... ./pkg/... ./controllers/...
