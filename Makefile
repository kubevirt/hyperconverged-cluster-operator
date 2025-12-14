QUAY_USERNAME      ?=
QUAY_PASSWORD      ?=
SOURCE_DIRS        = cmd pkg
SOURCES            := $(shell find . -name '*.go' -not -path "*/vendor/*")
SHA                := $(shell git describe --no-match  --always --abbrev=40 --dirty)
IMAGE_REGISTRY     ?= quay.io
REGISTRY_NAMESPACE ?= kubevirt
IMAGE_TAG          ?= latest
OPERATOR_IMAGE     ?= $(REGISTRY_NAMESPACE)/hyperconverged-cluster-operator
WEBHOOK_IMAGE      ?= $(REGISTRY_NAMESPACE)/hyperconverged-cluster-webhook
FUNC_TEST_IMAGE    ?= $(REGISTRY_NAMESPACE)/hyperconverged-cluster-functest
VIRT_ARTIFACTS_SERVER ?= $(REGISTRY_NAMESPACE)/virt-artifacts-server
BUNDLE_IMAGE       ?= $(REGISTRY_NAMESPACE)/hyperconverged-cluster-bundle
INDEX_IMAGE        ?= $(REGISTRY_NAMESPACE)/hyperconverged-cluster-index
BUILDER_IMAGE      ?= $(REGISTRY_NAMESPACE)/hco-builder
BUILDER_IMAGE_TAG  ?= latest
LDFLAGS            ?= -w -s
GOLANDCI_LINT_VERSION ?= v2.6.0
HCO_BUMP_LEVEL ?= minor
ASSETS_DIR ?= assets
DUMP_NETWORK_POLICIES ?= "false"
UNAME_ARCH     := $(shell uname -m)
ifeq ($(UNAME_ARCH),x86_64)
	TEMP_ARCH = amd64
else ifeq ($(UNAME_ARCH),aarch64)
	TEMP_ARCH = arm64
else
	TEMP_ARCH := $(UNAME_ARCH)
endif

ARCH ?= $(TEMP_ARCH)

# Prow doesn't have docker command
DO=./hack/in-docker.sh
ifeq (, $(shell (which docker 2> /dev/null || which podman 2> /dev/null)))
DO=eval
export JOB_TYPE=prow
endif

sanity: generate gogenerate cp-json-patch gogenerate-crd-creator generate-doc validate-no-offensive-lang goimport lint-metrics lint-monitoring
	go version
	go fmt ./...
	go mod tidy -v
	go mod vendor
	make build-manifests
	git add -N vendor
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

goimport:
	go install golang.org/x/tools/cmd/goimports@latest
	goimports -w -local="kubevirt.io,github.com/kubevirt,github.com/kubevirt/hyperconverged-cluster-operator"  $(shell find . -type f -name '*.go' ! -path "*/vendor/*" ! -path "./_kubevirtci/*" ! -path "*zz_generated*" )


lint:
	GOTOOLCHAIN=go1.24.3 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANDCI_LINT_VERSION}
	golangci-lint run

build: build-operator build-csv-merger build-webhook

build-operator: $(SOURCES) ## Build binary from source
	go build -ldflags="${LDFLAGS}" -o _out/hyperconverged-cluster-operator ./cmd/hyperconverged-cluster-operator

build-csv-merger: ## Build binary from source
	go build -ldflags="${LDFLAGS}" -o _out/csv-merger ./tools/csv-merger

build-manifest-templator: ## Build binary from source
	go build -ldflags="${LDFLAGS}" -o _out/manifest-templator ./tools/manifest-templator

build-crd-creator: generate
	go build -ldflags="${LDFLAGS}" -o _out/crd-creator ./tools/crd-creator

build-manifest-splitter:
	go build -ldflags="${LDFLAGS}" -o _out/manifest-splitter ./tools/manifest-splitter

build-webhook: $(SOURCES) ## Build binary from source
	go build -ldflags="${LDFLAGS}" -o _out/hyperconverged-cluster-webhook ./cmd/hyperconverged-cluster-webhook

build-manifests: gogenerate-crd-creator build-crd-creator build-csv-merger build-manifest-splitter build-manifest-templator
	DUMP_NETWORK_POLICIES=$(DUMP_NETWORK_POLICIES) ./hack/build-manifests.sh

build-manifests-prev:
	RELEASE_DELTA=1 make build-manifests

build-prom-spec-dumper: ## Build binary from source
	go build -ldflags="${LDFLAGS}" -o _out/rule-spec-dumper ./hack/prom-rule-ci/rule-spec-dumper.go

current-dir := $(realpath .)

prom-rules-verify: build-prom-spec-dumper
	./hack/prom-rule-ci/verify-rules.sh \
		"${current-dir}/_out/rule-spec-dumper" \
		"${current-dir}/hack/prom-rule-ci"

install:
	go install ./cmd/...

clean: ## Clean up the working environment
	@rm -rf _out/

start:
	./hack/deploy.sh

quay-token:
	@./tools/token.sh $(QUAY_USERNAME) $(QUAY_PASSWORD)

bundle-push: container-build-operator-courier
	@QUAY_USERNAME=$(QUAY_USERNAME) QUAY_PASSWORD=$(QUAY_PASSWORD) ./tools/operator-courier/push.sh

hack-clean: ## Run ./hack/clean.sh
	./hack/clean.sh

container-build: container-build-operator container-build-webhook container-build-operator-courier container-build-functest container-build-artifacts-server

build-push-multi-arch-images: build-push-multi-arch-operator-image build-push-multi-arch-webhook-image build-push-multi-arch-functest-image build-push-multi-arch-artifacts-server

container-build-operator: gogenerate gogenerate-crd-creator
	. "hack/cri-bin.sh" && $$CRI_BIN build --platform=linux/$(ARCH) -f build/Dockerfile -t $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG) --build-arg git_sha=$(SHA) .

build-push-multi-arch-operator-image: gogenerate gogenerate-crd-creator
	IMAGE_NAME=$(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG) SHA=SHA DOCKER_FILE=build/Dockerfile ./hack/build-push-multi-arch-images.sh

container-build-webhook:
	. "hack/cri-bin.sh" && $$CRI_BIN build --platform=linux/$(ARCH) -f build/Dockerfile.webhook -t $(IMAGE_REGISTRY)/$(WEBHOOK_IMAGE):$(IMAGE_TAG) --build-arg git_sha=$(SHA) .

build-push-multi-arch-webhook-image:
	IMAGE_NAME=$(IMAGE_REGISTRY)/$(WEBHOOK_IMAGE):$(IMAGE_TAG) SHA=$(SHA) DOCKER_FILE="build/Dockerfile.webhook" ./hack/build-push-multi-arch-images.sh

container-build-operator-courier:
	podman build -f tools/operator-courier/Dockerfile -t hco-courier .

container-build-validate-bundles:
	podman build -f tools/operator-sdk-validate/Dockerfile -t operator-sdk-validate-hco .

container-build-functest:
	. "hack/cri-bin.sh" && $$CRI_BIN build  --platform=linux/$(ARCH) -f build/Dockerfile.functest -t $(IMAGE_REGISTRY)/$(FUNC_TEST_IMAGE):$(IMAGE_TAG) --build-arg git_sha=$(SHA) .

build-push-multi-arch-functest-image:
	IMAGE_NAME=$(IMAGE_REGISTRY)/$(FUNC_TEST_IMAGE):$(IMAGE_TAG) SHA=$(SHA) DOCKER_FILE="build/Dockerfile.functest" ./hack/build-push-multi-arch-images.sh

container-build-artifacts-server:
	podman build -f build/Dockerfile.artifacts -t $(IMAGE_REGISTRY)/$(VIRT_ARTIFACTS_SERVER):$(IMAGE_TAG) --build-arg git_sha=$(SHA) .

build-push-multi-arch-artifacts-server:
	IMAGE_NAME=$(IMAGE_REGISTRY)/$(VIRT_ARTIFACTS_SERVER):$(IMAGE_TAG) SHA=$(SHA) DOCKER_FILE="build/Dockerfile.artifacts" ./hack/build-push-multi-arch-images.sh

container-push: container-push-operator container-push-webhook container-push-functest container-push-artifacts-server

quay-login:
	podman login $(IMAGE_REGISTRY) -u $(QUAY_USERNAME) -p "$(QUAY_PASSWORD)"

container-push-operator:
	. "hack/cri-bin.sh" && $$CRI_BIN push $$CRI_INSECURE $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG)

container-push-webhook:
	. "hack/cri-bin.sh" && $$CRI_BIN push $$CRI_INSECURE $(IMAGE_REGISTRY)/$(WEBHOOK_IMAGE):$(IMAGE_TAG)

container-push-functest:
	. "hack/cri-bin.sh" && $$CRI_BIN push $$CRI_INSECURE $(IMAGE_REGISTRY)/$(FUNC_TEST_IMAGE):$(IMAGE_TAG)

container-push-artifacts-server:
	. "hack/cri-bin.sh" && $$CRI_BIN push $(IMAGE_REGISTRY)/$(VIRT_ARTIFACTS_SERVER):$(IMAGE_TAG)

retag-push-all-images:
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(OPERATOR_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(WEBHOOK_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(FUNC_TEST_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(VIRT_ARTIFACTS_SERVER) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(BUNDLE_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(INDEX_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(NEW_TAG) ./hack/retag-multi-arch-images.sh

cluster-up:
	./cluster/up.sh

cluster-down:
	./cluster/down.sh

cluster-sync:
	IMAGE_REGISTRY=$(IMAGE_REGISTRY) REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) ./cluster/sync.sh

cluster-redeploy-operator:
	IMAGE_REGISTRY=$(IMAGE_REGISTRY) REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) ./cluster/sync-pod.sh operator

cluster-redeploy-webhook:
	IMAGE_REGISTRY=$(IMAGE_REGISTRY) REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) ./cluster/sync-pod.sh webhook

cluster-clean:
	CMD="./cluster/kubectl.sh" ./hack/clean.sh

ci-functest: build-functest test-functional

functest: test-functional-in-container

build-functest:
	${DO} ./hack/build-tests.sh

test-functional:
	JOB_TYPE="stdci" ./hack/run-tests.sh

test-functional-prow:
	./hack/run-tests.sh

test-functional-in-container:
	./hack/run-tests-in-container.sh

test-kv-smoke-prow:
	./hack/kv-smoke-tests.sh

stageRegistry:
	@APP_REGISTRY_NAMESPACE=redhat-operators-stage PACKAGE=kubevirt-hyperconverged ./tools/quay-registry.sh $(QUAY_USERNAME) $(QUAY_PASSWORD)

bundleRegistry:
	REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) IMAGE_REGISTRY=$(IMAGE_REGISTRY) ./hack/build-registry-bundle.sh

container-clusterserviceversion:
	REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) IMAGE_REGISTRY=$(IMAGE_REGISTRY) ./hack/upgrade-test-clusterserviceversion.sh

build-push-all: container-build-operator container-push-operator container-build-webhook container-push-webhook container-build-operator-courier bundle-push

upgrade-test:
	./hack/upgrade-test.sh

upgrade-test-index-image:
	./hack/upgrade-test-index-image.sh

upgrade-test-operator-sdk:
	./hack/upgrade-test-operator-sdk.sh

consecutive-upgrades-test:
	./hack/consecutive-upgrades-test.sh

kubevirt-nightly-test:
	./hack/kubevirt-nightly-test.sh

dump-state:
	./hack/dump-state.sh 

bump-kubevirtci:
	./hack/bump-kubevirtci.sh

gogenerate: generate
	go generate ./pkg/upgradepatch

gogenerate-crd-creator: generate
	go generate ./tools/csv-merger
	go generate ./tools/manifest-templator

generate:
	./hack/generate.sh

generate-doc: build-docgen
	_out/docgen ./api/v1beta1/hyperconverged_types.go > docs/api.md
	_out/metricsdocs > docs/metrics.md

build-docgen:
	go build -ldflags="${LDFLAGS}" -o _out/docgen ./tools/docgen
	go build -ldflags="${LDFLAGS}" -o _out/metricsdocs ./tools/metricsdocs

help: ## Show this help screen
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

cp-json-patch:
	cp ./assets/upgradePatches.json ./controllers/hyperconverged/test-files/upgradePatches/
	cp ./assets/upgradePatches.json ./pkg/upgradepatch/test-files/

test-unit: gogenerate cp-json-patch gogenerate-crd-creator
	JOB_TYPE="travis" ./hack/build-tests.sh

test: test-unit

charts:
	./hack/build-charts.sh

local:
	./hack/make_local.sh

deploy_cr: deploy_hco_cr deploy_hpp deploy_fake_kubedescheduler

deploy_hco_cr:
	./hack/deploy_only_cr.sh

deploy_hpp:
	./hack/hpp/deploy_hpp.sh

deploy_fake_kubedescheduler:
	./hack/kubeDescheduler/deploy_fake_kubedescheduler.sh

validate-no-offensive-lang:
	./hack/validate-no-offensive-lang.sh

lint-metrics:
	./hack/prom_metric_linter.sh --operator-name="kubevirt" --sub-operator-name="hco"

lint-monitoring:
	go install github.com/kubevirt/monitoring/monitoringlinter/cmd/monitoringlinter@a697c0c
	monitoringlinter ./...

bump-hco:
	./hack/bump-hco.sh ${HCO_BUMP_LEVEL}

build-annotate-dicts:
	go build -o ./_out/annotate-dicts ./tools/annotate-dicts/

annotate-dicts: build-annotate-dicts
	ASSETS_DIR=$(ASSETS_DIR) ./hack/annotate-dicts.sh

create-builder-image:
	IMAGE_NAME=$(IMAGE_REGISTRY)/$(BUILDER_IMAGE):$(IMAGE_TAG) ./hack/builder/create-builder-image.sh

retag-builder-image:
	IMAGE_REPO=$(IMAGE_REGISTRY)/$(BUILDER_IMAGE) CURRENT_TAG=$(IMAGE_TAG) NEW_TAG=$(BUILDER_IMAGE_TAG) ./hack/retag-multi-arch-images.sh

# Temporary alias, until changing the usage of this target, from another repo
push-builder-image: retag-builder-image

.PHONY: start \
		clean \
		build \
		build-operator \
		build-csv-merger \
		build-manifest-templator \
		build-crd-creator \
		build-manifest-splitter \
		build-webhook \
		build-manifests \
		build-manifests-prev \
		help \
		hack-clean \
		container-build \
		container-build-operator \
		container-build-webhook \
		container-build-operator-courier \
		container-build-validate-bundles \
		container-build-functest \
		container-build-artifacts-server \
		container-push \
		container-push-operator \
		container-push-webhook \
		container-push-functest \
		container-push-artifacts-server \
		cluster-up \
		cluster-down \
		cluster-sync \
		cluster-redeploy-operator \
		cluster-redeploy-webhook \
		cluster-clean \
		stageRegistry \
		functest \
		quay-token \
		bundle-push \
		build-push-all \
		ci-functest \
		build-functest \
		test-functional \
		test-functional-prow \
		test-functional-in-container \
		test-kv-smoke-prow \
		charts \
		kubevirt-nightly-test \
		local \
		deploy_cr \
		deploy_fake_kubedescheduler \
		build-docgen \
		gogenerate \
		generate \
		generate-doc \
		validate-no-offensive-lang \
		lint-metrics \
		lint-monitoring \
		sanity \
		goimport \
		bump-hco \
		bump-kubevirtci \
		build-push-multi-arch-operator-image \
		build-push-multi-arch-webhook-image \
		build-push-multi-arch-functest-image \
		build-push-multi-arch-artifacts-server \
		build-push-multi-arch-images \
		retag-push-all-images \
		build-annotate-dicts \
		annotate-dicts \
		cp-json-patch \
		create-builder-image \
		push-builder-image \
		retag-builder-image
