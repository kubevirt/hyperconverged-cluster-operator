#!/bin/bash

set -euxo pipefail

mkdir -p coverprofiles
go test -v -outputdir=./coverprofiles \
   -coverpkg=./api/v1beta1,./pkg/...,./controllers/... \
   -coverprofile=cover.coverprofile.temp \
   ./api/v1beta1 ./pkg/... ./controllers/...
# don't compute coverage of auto generated code
grep -v zz_generated ./coverprofiles/cover.coverprofile.temp > ./coverprofiles/cover.coverprofile
rm ./coverprofiles/cover.coverprofile.temp
