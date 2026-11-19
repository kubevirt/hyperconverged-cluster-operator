#!/usr/bin/env bash

# Refresh testdata from the libvirt cpu_map shipped with this repo's KubeVirt.
# KUBEVIRT_VERSION is hack/config. Libvirt is the newest LIBVIRT_VERSION in that
# release's hack/rpm-deps.sh.

set -euo pipefail

cd "$(dirname "$0")/.."
kv=$(sed -n 's/^KUBEVIRT_VERSION="\(.*\)"$/\1/p' hack/config)
ref=v$(curl -fsSL "https://raw.githubusercontent.com/kubevirt/kubevirt/${kv}/hack/rpm-deps.sh" \
	| sed -n 's/.*LIBVIRT_VERSION=.*:\([0-9.]*\)-.*/\1/p' \
	| LC_ALL=C sort -V | tail -n 1)
dest=pkg/internal/nodeinfo/testdata/libvirt_x86_cpu_models.txt

echo "# libvirt ${ref} from KubeVirt ${kv}" > "${dest}"
curl -fsSL "https://gitlab.com/libvirt/libvirt/-/raw/${ref}/src/cpu_map/index.xml" \
	| sed -n "s/.*filename='x86_\(.*\)\.xml'.*/\1/p" \
	| grep -Ev '^(features|vendors)$' \
	| LC_ALL=C sort -u >> "${dest}"
