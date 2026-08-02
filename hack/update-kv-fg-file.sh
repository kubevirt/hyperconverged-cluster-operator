#!/usr/bin/env bash
set -ex

source "hack/validate-kv-fg-file.sh"

trap 'rm -f kv-beta-feature-gates.json' EXIT

KV_VERSION=$(grep "KUBEVIRT_VERSION=" hack/config | sed -E 's|^KUBEVIRT_VERSION="(.*)"$|\1|')
gh release download "${KV_VERSION}" \
   -R kubevirt/kubevirt \
   -O kv-beta-feature-gates.json \
   --pattern=feature-gates.json \
   --clobber

validate-kv-fg-json-file kv-beta-feature-gates.json

if ! diff -q pkg/internal/kvfeaturegates/kv-beta-feature-gates.json kv-beta-feature-gates.json > /dev/null; then
  mv kv-beta-feature-gates.json pkg/internal/kvfeaturegates/kv-beta-feature-gates.json
fi
