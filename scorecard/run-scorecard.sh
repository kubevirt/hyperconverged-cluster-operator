#!/usr/bin/env bash

set -euo pipefail

TEMPDIR=$(mktemp -d) || (echo "Failed to create temp directory" && exit 1)
BUNDLEDIR="$TEMPDIR/bundle"
mkdir -p "$BUNDLEDIR"

cp -R deploy/index-image/kubevirt-hyperconverged/1.4.0/metadata "$BUNDLEDIR"

mkdir -p "$BUNDLEDIR/manifests"
cp deploy/index-image/kubevirt-hyperconverged/1.4.0/*.yaml "$BUNDLEDIR/manifests/"

cp tests/func-tests/_out/func-tests.test scorecard/image/func-tests.test

docker build -t  quay.io/erkanerol/custom-scorecard-tests:dev8 scorecard/image
docker push quay.io/erkanerol/custom-scorecard-tests:dev8

# kubectl create ns scorecard
# kubectl create serviceaccount scorecard -n scorecard
# kubectl create clusterrolebinding scorecard-cluster-admin --clusterrole=cluster-admin --serviceaccount=scorecard:scorecard
operator-sdk scorecard "$BUNDLEDIR"  -c ./scorecard/config.yaml -n scorecard -s scorecard  --verbose -x --wait-time=300s --selector=test=functest