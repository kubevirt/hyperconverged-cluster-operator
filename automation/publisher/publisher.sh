set -ex

function main() {
  TARGET_REPO_UPSTREAM=${TARGET_REPO_UPSTREAM:-"k8s-operatorhub/community-operators"}
  TARGET_REPO_COMMUNITY=${TARGET_REPO_COMMUNITY:-"redhat-openshift-ecosystem/community-operators-prod"}
  PACKAGE_DIR=${PACKAGE_DIR:-"_out/olm-catalog/community-kubevirt-hyperconverged"}
  PR_TEMPLATE="https://raw.githubusercontent.com/operator-framework/community-operators/master/docs/pull_request_template.md"
  BOT_USERNAME="hco-bot"

  if [ -z "${TAGGED_VERSION}" ]
  then
    echo "ERROR: Tagged version was not provided."
    exit 1
  fi

  echo "The tagged HCO version is: ${TAGGED_VERSION}"

  cp -r "${PACKAGE_DIR}/${TAGGED_VERSION}" ../hco_bundle

  cd ..
  tree hco_bundle

  PR_BODY=$(get_pr_body "${TAGGED_VERSION}")

  echo "Add annotation for community-operators index image"
  IFS='.' read -r -a SPLITTED_BASE_VERSION <<< "${TAGGED_VERSION%.*}"
  SPLITTED_BASE_VERSION[0]=$((SPLITTED_BASE_VERSION[0]+3))
  SPLITTED_BASE_VERSION[1]=$((SPLITTED_BASE_VERSION[1]+4))
  INDEX_IMAGE_VERSION=$(IFS=. ; echo "${SPLITTED_BASE_VERSION[*]}")
  echo "  com.redhat.openshift.versions: \"v${INDEX_IMAGE_VERSION}\"" >> hco_bundle/metadata/annotations.yaml

  echo "Login to GH account for GH CLI"
  echo "${HCO_BOT_TOKEN}" > token.txt
  gh auth login --with-token < token.txt
  rm -f token.txt

  create_pr "${TARGET_REPO_UPSTREAM}"
  create_pr "${TARGET_REPO_COMMUNITY}"
}

function create_pr() {
  TARGET_REPO=$1

  echo "Clone the community operators repo"
  gh repo clone "${TARGET_REPO}"

  echo "Update HCO manifests"
  BUNDLE_DIR=${TARGET_REPO##*/}/operators/community-kubevirt-hyperconverged/${TAGGED_VERSION}
  mkdir -p "${BUNDLE_DIR}"
  cp -r hco_bundle/* "${BUNDLE_DIR}"

  echo "Open a pull request to ${TARGET_REPO}"
  cd "${TARGET_REPO##*/}"
  git config user.name "hco-bot"
  git config user.email "hco-bot@redhat.com"
  git add .
  BRANCH_NAME=${TARGET_REPO%%/*}-release_hco_v${TAGGED_VERSION}
  git checkout -b "${BRANCH_NAME}"
  git status
  git commit -asm "Release Kubevirt HCO v${TAGGED_VERSION}"
  git push "https://${HCO_BOT_TOKEN}@github.com/${BOT_USERNAME}/${TARGET_REPO##*/}.git"
  echo "Create a pull request to ${TARGET_REPO} with tagged HCO version"
  gh pr create --title "[${TARGET_REPO%%/*}]: Release Kubevirt HCO v${TAGGED_VERSION}" --body "${PR_BODY}" \
    --repo "${TARGET_REPO}" --head "${BOT_USERNAME}:${BRANCH_NAME}"
  cd ..
  rm -rf "${TARGET_REPO##*/}"
}

function get_pr_body() {
   wget ${PR_TEMPLATE}
   sed -ir "s/\[ \]/\[x\]/g; 0,/Is operator/d" pull_request_template.md
   sed -r "1s/^/Release Kubevirt HCO v$1\n/" pull_request_template.md
   rm -f pull_request_template.md
}

main
