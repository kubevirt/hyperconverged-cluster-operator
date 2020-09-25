QUAY_USERNAME      ?=
QUAY_PASSWORD      ?=
SOURCE_DIRS        = cmd pkg
SOURCES            := $(shell find . -name '*.go' -not -path "*/vendor/*")
SHA                := $(shell git describe --no-match  --always --abbrev=40 --dirty)
IMAGE_REGISTRY     ?= quay.io
IMAGE_TAG          ?= latest
OPERATOR_IMAGE     ?= kubevirt/hyperconverged-cluster-operator
REGISTRY_NAMESPACE ?=


# Prow doesn't have docker command
DO=./hack/in-docker.sh
ifeq (, $(shell which docker))
DO=eval
export JOB_TYPE=prow
endif

sanity:
	go version
	go fmt ./...
	go mod tidy -v
	go mod vendor
	./hack/build-manifests.sh
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

build: $(SOURCES) ## Build binary from source
	go build -i -ldflags="-s -w" -o _out/hyperconverged-cluster-operator ./cmd/hyperconverged-cluster-operator
	go build -i -ldflags="-s -w" -o _out/csv-merger tools/csv-merger/csv-merger.go

build-manifests:
	./hack/build-manifests.sh

build-manifests-prev:
	RELEASE_DELTA=1 ./hack/build-manifests.sh

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

container-build: container-build-operator container-build-operator-courier

container-build-operator:
	docker build -f build/Dockerfile -t $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG) --build-arg git_sha=$(SHA) .

container-build-operator-courier:
	docker build -f tools/operator-courier/Dockerfile -t hco-courier .

container-push: container-push-operator

container-push-operator:
	docker push $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG)

cluster-up:
	./cluster/up.sh

cluster-down:
	./cluster/down.sh

cluster-sync:
	./cluster/sync.sh

cluster-clean:
	CMD="./cluster/kubectl.sh" ./hack/clean.sh

ci-functest: build-functest test-functional

functest: build-functest test-functional-prow

build-functest:
	${DO} ./hack/build-tests.sh

test-functional:
	JOB_TYPE="stdci" ./hack/run-tests.sh

test-functional-prow:
	./hack/run-tests.sh

stageRegistry:
	@APP_REGISTRY_NAMESPACE=redhat-operators-stage PACKAGE=kubevirt-hyperconverged ./tools/quay-registry.sh $(QUAY_USERNAME) $(QUAY_PASSWORD)

bundleRegistry:
	REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) IMAGE_REGISTRY=$(IMAGE_REGISTRY) ./hack/build-registry-bundle.sh

container-clusterserviceversion:
	REGISTRY_NAMESPACE=$(REGISTRY_NAMESPACE) IMAGE_REGISTRY=$(IMAGE_REGISTRY) ./hack/upgrade-test-clusterserviceversion.sh

build-push-all: container-build-operator container-push-operator container-build-operator-courier bundle-push

upgrade-test:
	./hack/upgrade-test.sh

kubevirt-nightly-test:
	./hack/kubevirt-nightly-test.sh

dump-state:
	./hack/dump-state.sh 

bump-kubevirtci:
	rm -rf _kubevirtci
	./hack/bump-kubevirtci.sh

help: ## Show this help screen
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

test-unit:
	JOB_TYPE="travis" ./hack/build-tests.sh

test: test-unit

charts:
	./hack/build-charts.sh

local:
	./hack/make_local.sh

deploy_cr:
	./hack/deploy_only_cr.sh

.PHONY: start \
		clean \
		build \
		build-manifests \
		build-manifests-prev \
		help \
		hack-clean \
		container-build \
		container-build-operator \
		container-push \
		container-push-operator \
		container-build-operator-courier \
		cluster-up \
		cluster-down \
		cluster-sync \
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
		charts \
		kubevirt-nightly-test \
		local \
		deploy_cr \
