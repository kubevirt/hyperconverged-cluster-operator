#!/usr/bin/env bash

set -euo pipefail

TEMPDIR=$(mktemp -d) || (echo "Failed to create temp directory" && exit 1)
BUNDLEDIR="$TEMPDIR/bundle"
mkdir -p "$BUNDLEDIR"

cp -R ./deploy/index-image/kubevirt-hyperconverged/1.4.0/metadata "$BUNDLEDIR"

mkdir -p "$BUNDLEDIR/manifests"
cp ./deploy/index-image/kubevirt-hyperconverged/1.4.0/*.yaml "$BUNDLEDIR/manifests/"

go build -o scorecard/image/custom-scorecard-tests  scorecard/image/main.go

docker build -t  quay.io/erkanerol/custom-scorecard-tests:dev scorecard/image
docker push quay.io/erkanerol/custom-scorecard-tests:dev

operator-sdk scorecard "$BUNDLEDIR"  -c ./scorecard/config.yaml  --verbose -x