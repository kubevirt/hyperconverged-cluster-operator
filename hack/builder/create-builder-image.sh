#!/usr/bin/env bash
# Load build arguments directly from go.mod

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
IMAGE_NAME=${IMAGE_NAME:-hco-builder}

# Extract K8S_VER from code-generator replace directive
K8S_VER=$(grep "k8s.io/code-generator => k8s.io/code-generator" go.mod | xargs | cut -d" " -f4)
    
# Extract KUBEOPENAPI_VER from kube-openapi replace directive
KUBEOPENAPI_VER=$(grep "k8s.io/kube-openapi => k8s.io/kube-openapi" go.mod | xargs | cut -d" " -f4)

echo "Loaded from go.mod: K8S_VER=${K8S_VER}, KUBEOPENAPI_VER=${KUBEOPENAPI_VER}"

podman build -t ${IMAGE_NAME} -f hack/builder/Dockerfile --build-arg K8S_VER=${K8S_VER} --build-arg KUBEOPENAPI_VER=${KUBEOPENAPI_VER} ${script_dir}


