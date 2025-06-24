module github.com/kubevirt/hyperconverged-cluster-operator

go 1.24.3

require (
	dario.cat/mergo v1.0.1
	github.com/blang/semver/v4 v4.0.0
	github.com/evanphx/json-patch/v5 v5.9.11
	github.com/gertd/go-pluralize v0.2.1
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v1.4.2
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/kubevirt/cluster-network-addons-operator v0.99.0
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20240505100225-e29dee0bb12b
	github.com/machadovilaca/operator-observability v0.0.26
	github.com/onsi/ginkgo/v2 v2.23.4
	github.com/onsi/gomega v1.37.0
	github.com/openshift/api v3.9.1-0.20190517100836-d5b34b957e91+incompatible
	github.com/openshift/cluster-kube-descheduler-operator v0.0.0-20250410114548-481d56a6c34e
	github.com/openshift/custom-resource-status v1.1.2
	github.com/openshift/library-go v0.0.0-20250402180609-ce2ba53fb2a4
	github.com/operator-framework/api v0.30.0
	github.com/operator-framework/operator-lib v0.17.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.81.0
	github.com/prometheus/client_golang v1.22.0
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.63.0
	github.com/samber/lo v1.49.1
	github.com/spf13/pflag v1.0.6
	golang.org/x/mod v0.24.0
	golang.org/x/sync v0.13.0
	golang.org/x/tools v0.32.0
	gomodules.xyz/jsonpatch/v2 v2.5.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.32.3
	k8s.io/apiextensions-apiserver v0.32.3
	k8s.io/apimachinery v0.32.3
	k8s.io/apiserver v0.32.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.32.3
	k8s.io/utils v0.0.0-20250321185631-1f6e0b77f77e
	kubevirt.io/api v1.5.0
	kubevirt.io/application-aware-quota v1.4.0
	kubevirt.io/containerized-data-importer-api v1.62.0
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4
	kubevirt.io/ssp-operator/api v0.23.1
	sigs.k8s.io/controller-runtime v0.20.4
	sigs.k8s.io/controller-tools v0.17.3
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.8.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gobuffalo/flect v1.0.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.29.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.6.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
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

// Pinned to v0.32.3
replace (
	k8s.io/api => k8s.io/api v0.32.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.32.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.32.3
	k8s.io/apiserver => k8s.io/apiserver v0.32.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.32.3
	k8s.io/client-go => k8s.io/client-go v0.32.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.32.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.32.3
	k8s.io/code-generator => k8s.io/code-generator v0.32.3
	k8s.io/component-base => k8s.io/component-base v0.32.3
	k8s.io/cri-api => k8s.io/cri-api v0.32.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.32.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.32.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.32.3
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.32.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.32.3
	k8s.io/kubectl => k8s.io/kubectl v0.32.3
	k8s.io/kubelet => k8s.io/kubelet v0.32.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.32.3
	k8s.io/metrics => k8s.io/metrics v0.32.3
	k8s.io/node-api => k8s.io/node-api v0.32.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.32.3
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.32.3
	k8s.io/sample-controller => k8s.io/sample-controller v0.32.3
)

replace (
	github.com/appscode/jsonpatch => github.com/appscode/jsonpatch v1.0.1
	github.com/go-kit/kit => github.com/go-kit/kit v0.12.0
	github.com/openshift/machine-api-operator => github.com/openshift/machine-api-operator v0.2.1-0.20230329185430-d3973b45c2b6
)

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787

replace bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20250409155250-8fcc4e71758a
