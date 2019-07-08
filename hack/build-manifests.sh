#!/usr/bin/env bash
set -e

# TODO: If we create more hack scripts this should go in common
# and be sourced
PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/../)"

source $PROJECT_ROOT/hack/vars

# TODO: Move this to deploy
DEPLOY_DIR="${PROJECT_ROOT}/deploy"
STD_DEPLOY_DIR="${DEPLOY_DIR}/standard"
CONVERGED_DEPLOY_DIR="${DEPLOY_DIR}/converged"

NAMESPACE="${NAMESPACE:-kubevirt-hyperconverged}"
CSV_VERSION="${CSV_VERSION:-0.0.1}"

HCO_CONTAINER_PREFIX="${HCO_CONTAINER_PREFIX:-quay.io/kubevirt}"
CNA_CONTAINER_PREFIX="${CNA_CONTAINER_PREFIX:-quay.io/kubevirt}"
WEBUI_CONTAINER_PREFIX="${WEBUI_CONTAINER_PREFIX:-quay.io/kubevirt}"
SSP_CONTAINER_PREFIX="${SSP_CONTAINER_PREFIX:-quay.io/fromani}"
NMO_CONTAINER_PREFIX="${NMO_CONTAINER_PREFIX:-quay.io/kubevirt}"
KUBEVIRT_CONTAINER_PREFIX="${KUBEVIRT_CONTAINER_PREFIX:-docker.io/kubevirt}"
CDI_CONTAINER_PREFIX="${CDI_CONTAINER_PREFIX:-docker.io/kubevirt}"
IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-IfNotPresent}"

CNA_SRIOV_NETWORK_TYPE=${CNA_SRIOV_NETWORK_TYPE:-}
CNA_MULTUS_IMAGE=${CNA_MULTUS_IMAGE:-}
CNA_LINUX_BRIDGE_CNI_IMAGE=${CNA_LINUX_BRIDGE_CNI_IMAGE:-}
CNA_LINUX_BRIDGE_MARKER_IMAGE=${CNA_LINUX_BRIDGE_MARKER_IMAGE:-}
CNA_SRIOV_DP_IMAGE=${CNA_SRIOV_DP_IMAGE:-}
CNA_SRIOV_CNI_IMAGE=${CNA_SRIOV_CNI_IMAGE:-}
CNA_KUBE_MAC_POOL_IMAGE=${CNA_KUBE_MAC_POOL_IMAGE:-}
CNA_NM_STATE_HANDLER_IMAGE=${CNA_NM_STATE_HANDLER_IMAGE:-}

CDI_OPERATOR_NAME=${CDI_OPERATOR_NAME:-}
CDI_CONTROLLER_IMAGE=${CDI_CONTROLLER_IMAGE:-}
CDI_IMPORTER_IMAGE=${CDI_IMPORTER_IMAGE:-}
CDI_CLONER_IMAGE=${CDI_CLONER_IMAGE:-}
CDI_API_SERVER_IMAGE=${CDI_API_SERVER_IMAGE:-}
CDI_UPLOAD_PROXY_IMAGE=${CDI_UPLOAD_PROXY_IMAGE:-}
CDI_UPLOAD_SERVER_IMAGE=${CDI_UPLOAD_SERVER_IMAGE:-}

# Use 'latest' tag for all the operators images
USE_LATEST_TAG="${USE_LATEST_TAG:-false}"

(cd ${PROJECT_ROOT}/tools/manifest-templator/ && go build)

function versions {
    # HCO Tag hardcoded to latest
    HCO_TAG="${HCO_TAG:-latest}"

    KUBEVIRT_TAG="${KUBEVIRT_TAG:-$(dep status -f='{{if eq .ProjectRoot "kubevirt.io/kubevirt"}}{{.Version}} {{end}}')}"
    echo "KubeVirt: ${KUBEVIRT_TAG}"

    CDI_TAG="${CDI_TAG:-$(dep status -f='{{if eq .ProjectRoot "kubevirt.io/containerized-data-importer"}}{{.Version}} {{end}}')}"
    echo "CDI: ${CDI_TAG}"

    SSP_TAG="${SSP_TAG:-$(dep status -f='{{if eq .ProjectRoot "github.com/MarSik/kubevirt-ssp-operator"}}{{.Version}} {{end}}')}"
    echo "SSP: ${SSP_TAG}"

    WEB_UI_OPERATOR_TAG="$(dep status -f='{{if eq .ProjectRoot "github.com/kubevirt/web-ui-operator"}}{{.Version}} {{end}}')"
    echo "Web UI operator: ${WEB_UI_OPERATOR_TAG}"

    WEB_UI_TAG=$(curl --silent "https://api.github.com/repos/kubevirt/web-ui/releases/latest" | grep -Po '"tag_name": "\K.*?(?=")' | sed 's/kubevirt-/v/g' )
    echo "Web UI: ${WEB_UI_TAG}"

    NETWORK_ADDONS_TAG="${NETWORK_ADDONS_TAG:-$(dep status -f='{{if eq .ProjectRoot "github.com/kubevirt/cluster-network-addons-operator"}}{{.Version}} {{end}}')}"
    echo "Network Addons: ${NETWORK_ADDONS_TAG}"

    NMO_TAG="${NMO_TAG:-$(curl --silent 'https://api.github.com/repos/kubevirt/node-maintenance-operator/releases/latest' | grep -Po '"tag_name": "\K.*?(?=")')}"
    echo "NMO: ${NMO_TAG}   WARNING: Not using Gopkg.toml version"
}

function buildFlags {

  BUILD_FLAGS="--hco-tag=latest \
    --namespace=${NAMESPACE} \
    --csv-version=${CSV_VERSION} \
    --container-prefix=${CONTAINER_PREFIX} \
    --image-pull-policy=${IMAGE_PULL_POLICY}"


    if [ "${USE_LATEST_TAG}" == "false" ]; then
      versions

      BUILD_FLAGS="${BUILD_FLAGS} \
        --hco-tag=${HCO_TAG} \
        --kubevirt-tag=${KUBEVIRT_TAG} \
        --cdi-tag=${CDI_TAG} \
        --ssp-tag=${SSP_TAG} \
        --web-ui-tag=${WEB_UI_OPERATOR_TAG} \
        --nmo-tag=${NMO_TAG} \
        --network-addons-tag=${NETWORK_ADDONS_TAG}"

    fi

    if [ ! -z "${CNA_MULTUS_IMAGE}" ]; then
      BUILD_FLAGS="${BUILD_FLAGS} \
        --cna-sriov-network-type=${CNA_SRIOV_NETWORK_TYPE} \
        --cna-multus-image=${CNA_MULTUS_IMAGE} \
        --cna-linux-bridge-cni-image=${CNA_LINUX_BRIDGE_CNI_IMAGE} \
        --cna-linux-bridge-marker-image=${CNA_LINUX_BRIDGE_MARKER_IMAGE} \
        --cna-sriov-dp-image=${CNA_SRIOV_DP_IMAGE} \
        --cna-sriov-cni-image=${CNA_SRIOV_CNI_IMAGE} \
        --cna-kube-mac-pool-image=${CNA_KUBE_MAC_POOL_IMAGE} \
        --cna-nm-state-handler-image=${CNA_NM_STATE_HANDLER_IMAGE}"
    fi

    if [ ! -z "${CDI_CONTROLLER_IMAGE}" ]; then
      BUILD_FLAGS="${BUILD_FLAGS} \
        --cdi-operator-name=${CDI_OPERATOR_NAME} \
        --cdi-controller-image=${CDI_CONTROLLER_IMAGE} \
	      --cdi-importer-image=${CDI_IMPORTER_IMAGE} \
	      --cdi-cloner-image=${CDI_CLONER_IMAGE} \
	      --cdi-api-server-image=${CDI_API_SERVER_IMAGE} \
	      --cdi-upload-proxy-image=${CDI_UPLOAD_PROXY_IMAGE} \
	      --cdi-upload-server-image=${CDI_UPLOAD_SERVER_IMAGE}"
    fi
}

buildFlags

templates=$(cd ${PROJECT_ROOT}/templates && find . -type f -name "*.yaml.in")
for template in $templates; do
	infile="${PROJECT_ROOT}/templates/${template}"

	std_out_dir="$(dirname ${STD_DEPLOY_DIR}/${template})"
	std_out_dir=${std_out_dir/VERSION/$CSV_VERSION}
	mkdir -p ${std_out_dir}

	std_out_file="${std_out_dir}/$(basename -s .in $template)"
	std_out_file=${std_out_file/VERSION/v$CSV_VERSION}
	rendered=$( \
                 ${PROJECT_ROOT}/tools/manifest-templator/manifest-templator \
                 ${BUILD_FLAGS} \
								 --hco-container-prefix=${HCO_CONTAINER_PREFIX} \
                 --input-file=${infile} \
	)
	if [[ ! -z "$rendered" ]]; then
		echo -e "$rendered" > $std_out_file
	fi

	converged_out_dir="$(dirname ${CONVERGED_DEPLOY_DIR}/${template})"
	converged_out_dir=${converged_out_dir/VERSION/$CSV_VERSION}
	mkdir -p ${converged_out_dir}

	converged_out_file="${converged_out_dir}/$(basename -s .in $template)"
	converged_out_file=${converged_out_file/VERSION/v$CSV_VERSION}
	rendered=$( \
                 ${PROJECT_ROOT}/tools/manifest-templator/manifest-templator \
                 ${BUILD_FLAGS} \
                 --converged \
                 --hco-container-prefix=${HCO_CONTAINER_PREFIX} \
                 --kubevirt-container-prefix=${KUBEVIRT_CONTAINER_PREFIX} \
                 --cdi-container-prefix=${CDI_CONTAINER_PREFIX} \
                 --cna-container-prefix=${CNA_CONTAINER_PREFIX} \
                 --webui-container-prefix=${WEBUI_CONTAINER_PREFIX} \
                 --ssp-container-prefix=${SSP_CONTAINER_PREFIX} \
                 --nmo-container-prefix=${NMO_CONTAINER_PREFIX} \
                 --input-file=${infile} \
	)
	if [[ ! -z "$rendered" ]]; then
		echo -e "$rendered" > $converged_out_file
	fi
done

(cd ${PROJECT_ROOT}/tools/manifest-templator/ && go clean)
