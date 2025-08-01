#!/usr/bin/env bash
set -ex -o pipefail -o errtrace -o functrace

function catch() {
    echo "error $1 on line $2"
    exit 255
}

trap 'catch $? $LINENO' ERR TERM INT

# build-manifests is designed to populate the deploy directory
# with all of the manifests necessary for use in development
# and for consumption with the operator-lifecycle-manager.
#
# First, we create a temporary directory and filling it with
# all of the component operator's ClusterServiceVersion (CSV for OLM)
# and CustomResourceDefinitions (CRDs); being sure to copy the CRDs
# into the deploy/crds directory.
#
# The CSV manifests contain all of the information we need to 1) generate
# a combined CSV and 2) other development related manifests (like the
# operator deployment + rbac).
#
# Second, we pass all of the component CSVs off to the manifest-templator
# that handles the deployment specs, service account names, permissions, and
# clusterPermissions by converting them into their corresponding Kubernetes
# manifests (ie. permissions + serviceAccountName = role + service account
# + role binding) before writing them to disk.
#
# Lastly, we take give the component CSVs to the csv-merger that combines all
# of the manifests into a single, unified, ClusterServiceVersion.

PROJECT_ROOT="$(readlink -e $(dirname "${BASH_SOURCE[0]}")/../)"
source "${PROJECT_ROOT}"/hack/config
source hack/cri-bin.sh

TOOLS=${PROJECT_ROOT}/_out

# update image digests
"${PROJECT_ROOT}"/automation/digester/update_images.sh
source "${PROJECT_ROOT}"/deploy/images.env

HCO_OPERATOR_IMAGE=${HCO_OPERATOR_IMAGE:-quay.io/kubevirt/hyperconverged-cluster-operator:${CSV_VERSION}-unstable}
HCO_WEBHOOK_IMAGE=${HCO_WEBHOOK_IMAGE:-quay.io/kubevirt/hyperconverged-cluster-webhook:${CSV_VERSION}-unstable}
HCO_DOWNLOADS_IMAGE=${HCO_DOWNLOADS_IMAGE:-quay.io/kubevirt/virt-artifacts-server:${CSV_VERSION}-unstable}
DIGEST_LIST="${DIGEST_LIST},${HCO_OPERATOR_IMAGE}|hyperconverged-cluster-operator,${HCO_WEBHOOK_IMAGE}|hyperconverged-cluster-webhook,${HCO_DOWNLOADS_IMAGE}|virt-artifacts-server"

DEPLOY_DIR="${PROJECT_ROOT}/deploy"
CRD_DIR="${DEPLOY_DIR}/crds"
OLM_DIR="${DEPLOY_DIR}/olm-catalog"
CSV_VERSION=${CSV_VERSION}
CSV_TIMESTAMP=$(date +%Y%m%d%H%M -u)
PACKAGE_NAME="community-kubevirt-hyperconverged"
CSV_DIR="${OLM_DIR}/${PACKAGE_NAME}/${CSV_VERSION}"
DEFAULT_CSV_GENERATOR="/usr/bin/csv-generator"
SSP_CSV_GENERATOR="/csv-generator"
UNIQUE="${UNIQUE:-false}"

INDEX_IMAGE_DIR=${DEPLOY_DIR}/index-image
CSV_INDEX_IMAGE_DIR="${INDEX_IMAGE_DIR}/${PACKAGE_NAME}/${CSV_VERSION}"

OPERATOR_NAME="${OPERATOR_NAME:-kubevirt-hyperconverged-operator}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-kubevirt-hyperconverged}"
IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-IfNotPresent}"

# Important extensions
CSV_EXT="clusterserviceversion.yaml"
CSV_CRD_EXT="csv_crds.yaml"
CRD_EXT="crd.yaml"

readonly amd64_machinetype=q35
readonly arm64_machinetype=virt

function gen_csv() {
  # Handle arguments
  local csvGeneratorPath="$1" && shift
  local operatorName="$1" && shift
  local imagePullUrl="$1" && shift
  local dumpCRDsArg="$1" && shift
  local operatorArgs="$@"

  # Handle important vars
  local csv="${operatorName}.${CSV_EXT}"
  local crds="${operatorName}.crds.yaml"

  # TODO: Use oc to run if cluster is available
  local dockerArgs="$CRI_BIN run --rm --entrypoint=${csvGeneratorPath} ${imagePullUrl} ${operatorArgs}"

  eval $dockerArgs $dumpCRDsArg | ${TOOLS}/manifest-splitter --operator-name="${operatorName}"
}

function create_virt_csv() {
  local operatorName="kubevirt"
  local dumpCRDsArg="--dumpCRDs"
  local operatorArgs
  operatorArgs=" \
    --namespace=${OPERATOR_NAMESPACE} \
    --csvVersion=${CSV_VERSION} \
    --operatorImageVersion=${KUBEVIRT_OPERATOR_IMAGE/*@/} \
    --kubeVirtVersion=${KUBEVIRT_VERSION} \
    --virt-api-image="${KUBEVIRT_API_IMAGE}" \
    --virt-controller-image="${KUBEVIRT_CONTROLLER_IMAGE}" \
    --virt-handler-image="${KUBEVIRT_HANDLER_IMAGE}" \
    --virt-launcher-image="${KUBEVIRT_LAUNCHER_IMAGE}" \
    --virt-export-proxy-image="${KUBEVIRT_EXPORTPROXY_IMAGE}" \
    --virt-export-server-image="${KUBEVIRT_EXPORSERVER_IMAGE}" \
    --virt-synchronization-controller-image="${KUBEVIRT_SYNC_CONTROLLER_IMAGE}" \
    --gs-image="${KUBEVIRT_LIBGUESTFS_TOOLS_IMAGE}" \
    --sidecar-shim-image="${KUBEVIRT_SIDECAR_SHIM}" \
    --pr-helper-image="${KUBEVIRT_PR_HELPER}" \
    --virt-operator-image="${KUBEVIRT_OPERATOR_IMAGE}"
  "

  gen_csv "${DEFAULT_CSV_GENERATOR}" "${operatorName}" "${KUBEVIRT_OPERATOR_IMAGE}" "${dumpCRDsArg}" "${operatorArgs}"
  echo "${operatorName}"
}

function create_cna_csv() {
  local operatorName="cluster-network-addons"
  local dumpCRDsArg="--dump-crds"
  local containerPrefix="${CNA_OPERATOR_IMAGE%/*}"
  local imageName="${CNA_OPERATOR_IMAGE#${containerPrefix}/}"
  local tag="${CNA_OPERATOR_IMAGE/*:/}"
  local operatorArgs=" \
    --namespace=${OPERATOR_NAMESPACE} \
    --version=${CSV_VERSION} \
    --version-replaces=${REPLACES_VERSION} \
    --image-pull-policy=IfNotPresent \
    --operator-version=${NETWORK_ADDONS_VERSION} \
    --container-tag=${CNA_OPERATOR_IMAGE/*:/} \
    --container-prefix=${containerPrefix} \
    --image-name=${imageName/:*/} \
    --kube-rbac-proxy-image=${KUBE_RBAC_PROXY_IMAGE}
  "

  gen_csv ${DEFAULT_CSV_GENERATOR} ${operatorName} "${CNA_OPERATOR_IMAGE}" ${dumpCRDsArg} ${operatorArgs}
  echo "${operatorName}"
}

function create_ssp_csv() {
  local operatorName="scheduling-scale-performance"
  local dumpCRDsArg="--dump-crds"
  local operatorArgs=" \
    --namespace=${OPERATOR_NAMESPACE} \
    --csv-version=${CSV_VERSION} \
    --operator-image=${SSP_OPERATOR_IMAGE} \
    --operator-version=${SSP_VERSION} \
    --validator-image=${SSP_VALIDATOR_IMAGE} \
  "

  gen_csv ${SSP_CSV_GENERATOR} ${operatorName} "${SSP_OPERATOR_IMAGE}" ${dumpCRDsArg} ${operatorArgs}
  echo "${operatorName}"
}

function create_cdi_csv() {
  local operatorName="containerized-data-importer"

  local dumpCRDsArg="--dump-crds"
  local operatorArgs=" \
    --namespace=${OPERATOR_NAMESPACE} \
    --csv-version=${CSV_VERSION} \
    --pull-policy=IfNotPresent \
    --operator-image=${CDI_OPERATOR_IMAGE} \
    --controller-image=${CDI_CONTROLLER_IMAGE} \
    --apiserver-image=${CDI_APISERVER_IMAGE} \
    --cloner-image=${CDI_CLONER_IMAGE} \
    --importer-image=${CDI_IMPORTER_IMAGE} \
    --uploadproxy-image=${CDI_UPLOADPROXY_IMAGE} \
    --uploadserver-image=${CDI_UPLOADSERVER_IMAGE} \
    --operator-version=${CDI_VERSION} \
    --ovirt-populator-image="${CDI_IMPORTER_IMAGE}" \
  "
  gen_csv ${DEFAULT_CSV_GENERATOR} ${operatorName} "${CDI_OPERATOR_IMAGE}" ${dumpCRDsArg} ${operatorArgs}
  echo "${operatorName}"
}

function create_hpp_csv() {
  local operatorName="hostpath-provisioner"
  local dumpCRDsArg="--dump-crds"
  local operatorArgs=" \
    --csv-version=${CSV_VERSION} \
    --operator-image-name=${HPPO_IMAGE} \
    --provisioner-image-name=${HPP_IMAGE} \
    --csi-driver-image-name=${HPP_CSI_IMAGE} \
    --csi-node-driver-image-name=${NODE_DRIVER_REG_IMAGE} \
    --csi-liveness-probe-image-name=${LIVENESS_PROBE_IMAGE} \
    --csi-external-provisioner-image-name=${CSI_SIG_STORAGE_PROVISIONER_IMAGE} \
    --csi-snapshotter-image-name=${CSI_SNAPSHOT_IMAGE} \
    --namespace=${OPERATOR_NAMESPACE} \
    --pull-policy=IfNotPresent \
  "

  gen_csv ${DEFAULT_CSV_GENERATOR} ${operatorName} "${HPPO_IMAGE}" ${dumpCRDsArg} ${operatorArgs}
  echo "${operatorName}"
}

function create_aaq_csv() {
  local operatorName="application-aware-quota"
  local dumpCRDsArg="--dump-crds"
  local operatorArgs=" \
    --csv-version=${CSV_VERSION} \
    --operator-image=${AAQ_OPERATOR_IMAGE} \
    --controller-image=${AAQ_CONTROLLER_IMAGE} \
    --aaq-server-image=${AAQ_SERVER_IMAGE} \
    --operator-version=${AAQ_VERSION} \
    --namespace=${OPERATOR_NAMESPACE} \
    --pull-policy=IfNotPresent \
  "

  gen_csv ${DEFAULT_CSV_GENERATOR} ${operatorName} "${AAQ_OPERATOR_IMAGE}" ${dumpCRDsArg} ${operatorArgs}
  echo "${operatorName}"
}

# Write HCO CRDs
hco_crds=${PROJECT_ROOT}/config/crd/bases/hco.kubevirt.io_hyperconvergeds.yaml
${TOOLS}/crd-creator --output-file=${hco_crds}

(cd ${PROJECT_ROOT}/tools/manifest-splitter/ && go build)

TEMPDIR=$(mktemp -d) || (echo "Failed to create temp directory" && exit 1)
pushd $TEMPDIR
virtFile=$(create_virt_csv)
virtCsv="${TEMPDIR}/${virtFile}.${CSV_EXT}"
cnaFile=$(create_cna_csv)
cnaCsv="${TEMPDIR}/${cnaFile}.${CSV_EXT}"
sspFile=$(create_ssp_csv)
sspCsv="${TEMPDIR}/${sspFile}.${CSV_EXT}"
cdiFile=$(create_cdi_csv)
cdiCsv="${TEMPDIR}/${cdiFile}.${CSV_EXT}"
hppFile=$(create_hpp_csv)
hppCsv="${TEMPDIR}/${hppFile}.${CSV_EXT}"
aaqFile=$(create_aaq_csv)
aaqCsv="${TEMPDIR}/${aaqFile}.${CSV_EXT}"
csvOverrides="${TEMPDIR}/csv_overrides.${CSV_EXT}"
keywords="  keywords:
  - KubeVirt
  - Virtualization
  - VM"
cat > ${csvOverrides} <<- EOM
---
spec:
$keywords
EOM

cat ${hco_crds} | ${TOOLS}/manifest-splitter --operator-name="hco"

popd


rm -fr "${CSV_DIR}"
mkdir -p "${CSV_DIR}/metadata" "${CSV_DIR}/manifests"


cat << EOF > "${CSV_DIR}/metadata/annotations.yaml"
annotations:
  operators.operatorframework.io.bundle.channel.default.v1: ${CSV_VERSION}
  operators.operatorframework.io.bundle.channels.v1: ${CSV_VERSION}
  operators.operatorframework.io.bundle.manifests.v1: manifests/
  operators.operatorframework.io.bundle.mediatype.v1: registry+v1
  operators.operatorframework.io.bundle.metadata.v1: metadata/
  operators.operatorframework.io.bundle.package.v1: ${PACKAGE_NAME}
EOF


SMBIOS=$(cat <<- EOM
Family: KubeVirt
Manufacturer: KubeVirt
Product: None
EOM
)

# validate CSVs. Make sure each one of them contain an image (and so, also not empty):
csvs=("${cnaCsv}" "${virtCsv}" "${sspCsv}" "${cdiCsv}" "${hppCsv}" "${aaqCsv}")
for csv in "${csvs[@]}"; do
  grep -E "^ *image: [_a-zA-Z0-9/\.:@\-]+$" ${csv}
done

# Build and write deploy dir
${TOOLS}/manifest-templator \
  --api-sources=${PROJECT_ROOT}/api/... \
  --cna-csv="$(<${cnaCsv})" \
  --virt-csv="$(<${virtCsv})" \
  --ssp-csv="$(<${sspCsv})" \
  --cdi-csv="$(<${cdiCsv})" \
  --hpp-csv="$(<${hppCsv})" \
  --aaq-csv="$(<${aaqCsv})" \
  --kv-virtiowin-image-name="${KUBEVIRT_VIRTIO_IMAGE}" \
  --operator-namespace="${OPERATOR_NAMESPACE}" \
  --smbios="${SMBIOS}" \
  --amd64-machinetype="${amd64_machinetype}" \
  --arm64-machinetype="${arm64_machinetype}" \
  --hco-kv-io-version="${CSV_VERSION}" \
  --kubevirt-version="${KUBEVIRT_VERSION}" \
  --cdi-version="${CDI_VERSION}" \
  --cnao-version="${NETWORK_ADDONS_VERSION}" \
  --ssp-version="${SSP_VERSION}" \
  --hppo-version="${HPPO_VERSION}" \
  --aaq-version="${AAQ_VERSION}" \
  --operator-image="${HCO_OPERATOR_IMAGE}" \
  --webhook-image="${HCO_WEBHOOK_IMAGE}" \
  --network-passt-binding-image-name="${NETWORK_PASST_BINDING_IMAGE}" \
  --network-passt-binding-cni-image-name="${NETWORK_PASST_BINDING_CNI_IMAGE}" \
  --cli-downloads-image="${HCO_DOWNLOADS_IMAGE}"

if [[ "${UNIQUE}" == "true"  ]]; then
  CSV_VERSION_PARAM=${CSV_VERSION}-${CSV_TIMESTAMP}
  ENABLE_UNIQUE="true"
else
  CSV_VERSION_PARAM=${CSV_VERSION}
  ENABLE_UNIQUE="false"
fi

NETWORK_POLICIES_PARAMS=""
if [[ ${DUMP_NETWORK_POLICIES} == "true" ]]; then
  # Dump network policies to the deploy directory
  NETWORK_POLICIES_PARAMS="--dump-network-policies=true --deploy-k8s-dns-networkpolicy=true"
fi

# Build and merge CSVs
CSV_DIR=${CSV_DIR}/manifests
${TOOLS}/csv-merger \
  --cna-csv="$(<${cnaCsv})" \
  --virt-csv="$(<${virtCsv})" \
  --ssp-csv="$(<${sspCsv})" \
  --cdi-csv="$(<${cdiCsv})" \
  --hpp-csv="$(<${hppCsv})" \
  --aaq-csv="$(<${aaqCsv})" \
  --kv-virtiowin-image-name="${KUBEVIRT_VIRTIO_IMAGE}" \
  --csv-version=${CSV_VERSION_PARAM} \
  --replaces-csv-version=${REPLACES_CSV_VERSION} \
  --hco-kv-io-version="${CSV_VERSION}" \
  --spec-displayname="KubeVirt HyperConverged Cluster Operator" \
  --spec-description="$(<${PROJECT_ROOT}/docs/operator_description.md)" \
  --metadata-description="A unified operator deploying and controlling KubeVirt and its supporting operators with opinionated defaults" \
  --crd-display="HyperConverged Cluster Operator" \
  --smbios="${SMBIOS}" \
  --amd64-machinetype="${amd64_machinetype}" \
  --arm64-machinetype="${arm64_machinetype}" \
  --csv-overrides="$(<${csvOverrides})" \
  --enable-unique-version=${ENABLE_UNIQUE} \
  --kubevirt-version="${KUBEVIRT_VERSION}" \
  --cdi-version="${CDI_VERSION}" \
  --cnao-version="${NETWORK_ADDONS_VERSION}" \
  --ssp-version="${SSP_VERSION}" \
  --hppo-version="${HPPO_VERSION}" \
  --aaq-version="${AAQ_VERSION}" \
  --related-images-list="${DIGEST_LIST}" \
  --operator-image-name="${HCO_OPERATOR_IMAGE}" \
  --webhook-image-name="${HCO_WEBHOOK_IMAGE}" \
  --kubevirt-consoleplugin-image-name="${KUBEVIRT_CONSOLE_PLUGIN_IMAGE}" \
  --kubevirt-consoleproxy-image-name="${KUBEVIRT_CONSOLE_PROXY_IMAGE}" \
  --cli-downloads-image-name="${HCO_DOWNLOADS_IMAGE}" \
  --network-passt-binding-image-name="${NETWORK_PASST_BINDING_IMAGE}" \
  --network-passt-binding-cni-image-name="${NETWORK_PASST_BINDING_CNI_IMAGE}" \
  ${NETWORK_POLICIES_PARAMS} \
  > temp_manifests.yaml

  ${TOOLS}/manifest-splitter \
  --manifests-file=temp_manifests.yaml \
  --operator-name="${OPERATOR_NAME}" \
  --output-dir="${CSV_DIR}" \
  --csv-extension=".v${CSV_VERSION}.${CSV_EXT}"

rm -f temp_manifests.yaml

rendered_csv="$(cat "${CSV_DIR}/${OPERATOR_NAME}.v${CSV_VERSION}.${CSV_EXT}")"
rendered_keywords="$(echo "$rendered_csv" |grep 'keywords' -A 3)"
# assert that --csv-overrides work
[ "$keywords" == "$rendered_keywords" ]

# Copy all CRDs into the CRD and CSV directories
rm -f ${CRD_DIR}/*
cp -f ${TEMPDIR}/*.${CRD_EXT} ${CRD_DIR}
cp -f ${TEMPDIR}/*.${CRD_EXT} ${CSV_DIR}

# Validate the yaml files
(cd ${CRD_DIR} && $CRI_BIN run --rm -v "$(pwd)":/yaml quay.io/pusher/yamllint yamllint -d "{extends: relaxed, rules: {line-length: disable}}" /yaml)
(cd ${CSV_DIR} && $CRI_BIN run --rm -v "$(pwd)":/yaml quay.io/pusher/yamllint yamllint -d "{extends: relaxed, rules: {line-length: disable}}" /yaml)

# Check there are not API Groups overlap between different CNV operators
go run ${PROJECT_ROOT}/tools/crd-overlap-validator --crds-dir=${CRD_DIR}

if [[ "$1" == "UNIQUE"  ]]; then
  # Add the current CSV_TIMESTAMP to the currentCSV in the packages file
  sed -Ei "s/(currentCSV: ${OPERATOR_NAME}.v${CSV_VERSION}).*/\1-${CSV_TIMESTAMP}/" \
   ${PACKAGE_DIR}/kubevirt-hyperconverged.package.yaml
fi

# Intentionally removing last so failure leaves around the templates
rm -rf ${TEMPDIR}

CSV_FILE="${CSV_DIR}/kubevirt-hyperconverged-operator.v${CSV_VERSION}.${CSV_EXT}"
if git ls-files --error-unmatch "${CSV_FILE}"; then
  # If the only change in the CSV file is its "created_at" field, rollback this change as it causes git conflicts for
  # no good reason.
  if git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh ${CSV_FILE}; then
    git checkout ${CSV_FILE}
  fi
fi

# Prepare files for index-image files that will be used for testing in openshift CI
rm -rf "${INDEX_IMAGE_DIR:?}"
mkdir -p "${INDEX_IMAGE_DIR:?}/${PACKAGE_NAME}"
cp -r "${CSV_DIR%/*}" "${INDEX_IMAGE_DIR:?}/${PACKAGE_NAME}/"
cp "${OLM_DIR}/bundle.Dockerfile" "${INDEX_IMAGE_DIR:?}/"
cp "${OLM_DIR}/Dockerfile.bundle.ci-index-image-upgrade" "${INDEX_IMAGE_DIR:?}/"

INDEX_IMAGE_CSV="${INDEX_IMAGE_DIR}/${PACKAGE_NAME}/${CSV_VERSION}/manifests/kubevirt-hyperconverged-operator.v${CSV_VERSION}.${CSV_EXT}"
sed -r -i "s|createdAt: \".*\$|createdAt: \"2020-10-23 08:58:25\"|;" ${INDEX_IMAGE_CSV}
sed -r -i "s|quay.io/kubevirt/hyperconverged-cluster-operator.*$|+IMAGE_TO_REPLACE+|;" ${INDEX_IMAGE_CSV}
sed -r -i "s|quay.io/kubevirt/hyperconverged-cluster-webhook.*$|+WEBHOOK_IMAGE_TO_REPLACE+|" ${INDEX_IMAGE_CSV}
sed -r -i "s|quay.io/kubevirt/virt-artifacts-server.*$|+ARTIFACTS_SERVER_IMAGE_TO_REPLACE+|" ${INDEX_IMAGE_CSV}
