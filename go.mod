module github.com/kubevirt/hyperconverged-cluster-operator

go 1.25.3

require (
	dario.cat/mergo v1.0.2
	github.com/blang/semver/v4 v4.0.0
	github.com/containers/image/v5 v5.36.2
	github.com/evanphx/json-patch/v5 v5.9.11
	github.com/gertd/go-pluralize v0.2.1
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v1.4.3
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.7.7
	github.com/kubevirt/cluster-network-addons-operator v0.101.2
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20260217101511-9344f1349f88
	github.com/onsi/ginkgo/v2 v2.28.1
	github.com/onsi/gomega v1.39.1
	github.com/openshift/api v0.0.1
	github.com/openshift/cluster-kube-descheduler-operator v0.0.0-20260214173033-e860b7975c2c
	github.com/openshift/custom-resource-status v1.1.2
	github.com/openshift/library-go v0.0.0-20260213153706-03f1709971c5
	github.com/operator-framework/api v0.39.0
	github.com/operator-framework/operator-lib v0.19.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.89.0
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v1.20.99
	github.com/rhobs/operator-observability-toolkit v0.0.30
	github.com/rhobs/perses-operator v0.1.10-0.20250612173146-78eb619430df
	github.com/samber/lo v1.52.0
	github.com/spf13/pflag v1.0.10
	golang.org/x/mod v0.33.0
	golang.org/x/sync v0.19.0
	golang.org/x/tools v0.41.0
	gomodules.xyz/jsonpatch/v2 v2.5.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.35.1
	k8s.io/apiextensions-apiserver v0.35.1
	k8s.io/apimachinery v0.35.1
	k8s.io/apiserver v0.35.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/component-helpers v0.34.3
	k8s.io/kube-openapi v0.35.1
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2
	kubevirt.io/api v1.8.0-beta.0
	kubevirt.io/application-aware-quota v1.8.0-alpha.0
	kubevirt.io/containerized-data-importer-api v1.64.0
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4
	kubevirt.io/kubevirt-migration-operator v0.1.0
	kubevirt.io/ssp-operator/api v0.25.0
	sigs.k8s.io/controller-runtime v0.23.1
	sigs.k8s.io/controller-tools v0.20.1
	sigs.k8s.io/yaml v1.6.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/PaesslerAG/gval v1.2.4 // indirect
	github.com/PaesslerAG/jsonpath v0.1.2-0.20240726212847-3a740cf7976f // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/brunoga/deep v1.2.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/containers/storage v1.59.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.3.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gobuffalo/flect v1.0.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/cel-go v0.27.0 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/labstack/echo/v4 v4.13.4 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/nexucis/lamenv v0.5.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.2.1 // indirect
	github.com/perses/common v0.27.1-0.20250326140707-96e439b14e0e // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/rhobs/perses v0.0.0-20250612171017-5d7686af9ae4 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zitadel/oidc/v3 v3.36.1 // indirect
	github.com/zitadel/schema v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/term v0.39.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260202165425-ce8ad4cf556b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260202165425-ce8ad4cf556b // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
)

exclude k8s.io/cluster-bootstrap v0.0.0

exclude k8s.io/api v0.0.0

exclude k8s.io/apiextensions-apiserver v0.0.0

exclude k8s.io/apimachinery v0.0.0

exclude k8s.io/apiserver v0.0.0

exclude k8s.io/code-generator v0.0.0

exclude k8s.io/component-base v0.0.0

exclude k8s.io/kube-aggregator v0.0.0

exclude k8s.io/cli-runtime v0.0.0

exclude k8s.io/kubectl v0.0.0

exclude k8s.io/client-go v2.0.0-alpha.0.0.20181121191925-a47917edff34+incompatible

exclude k8s.io/client-go v0.0.0

exclude k8s.io/cloud-provider v0.0.0

exclude k8s.io/cri-api v0.0.0

exclude k8s.io/csi-translation-lib v0.0.0

exclude k8s.io/kube-controller-manager v0.0.0

exclude k8s.io/kube-proxy v0.0.0

exclude k8s.io/kube-scheduler v0.0.0

exclude k8s.io/kubelet v0.0.0

exclude k8s.io/legacy-cloud-providers v0.0.0

exclude k8s.io/metrics v0.0.0

exclude k8s.io/sample-apiserver v0.0.0

// Pinned to v0.35.1
replace (
	k8s.io/api => k8s.io/api v0.35.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.35.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.35.1
	k8s.io/apiserver => k8s.io/apiserver v0.35.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.35.1
	k8s.io/client-go => k8s.io/client-go v0.35.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.35.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.35.1
	k8s.io/code-generator => k8s.io/code-generator v0.35.1
	k8s.io/component-base => k8s.io/component-base v0.35.1
	k8s.io/cri-api => k8s.io/cri-api v0.35.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.35.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.35.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.35.1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20260127142750-a19766b6e2d4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.35.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.35.1
	k8s.io/kubectl => k8s.io/kubectl v0.35.1
	k8s.io/kubelet => k8s.io/kubelet v0.35.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.35.1
	k8s.io/metrics => k8s.io/metrics v0.35.1
	k8s.io/node-api => k8s.io/node-api v0.35.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.35.1
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.35.1
	k8s.io/sample-controller => k8s.io/sample-controller v0.35.1
)

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787

replace bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
