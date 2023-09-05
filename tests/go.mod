module github.com/kubevirt/hyperconverged-cluster-operator/tests

go 1.19

require (
	github.com/onsi/ginkgo/v2 v2.12.0
	github.com/onsi/gomega v1.27.10
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/client-go v0.0.1 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.67.1
	github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/common v0.44.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.28.1
	k8s.io/apimachinery v0.28.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	kubevirt.io/api v1.0.0
	kubevirt.io/client-go v1.0.0
	kubevirt.io/containerized-data-importer-api v1.57.0
	kubevirt.io/kubevirt v1.0.0
)

replace (
	github.com/operator-framework/operator-lib => github.com/operator-framework/operator-lib v0.0.0-20230717184314-6efbe3a22f6f
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.16.0
)

// Build with hyperconverged-cluster-operator from the repo
replace (
	cloud.google.com/go => cloud.google.com/go v0.100.2
	github.com/googleapis/gnostic => github.com/google/gnostic v0.6.8
	github.com/kubevirt/hyperconverged-cluster-operator => ../
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20230327201221-f5883ff37f0c
)

require (
	github.com/gertd/go-pluralize v0.2.1
	github.com/kubevirt/cluster-network-addons-operator v0.89.0
	github.com/kubevirt/hyperconverged-cluster-operator v0.0.0-00010101000000-000000000000
	github.com/openshift/custom-resource-status v1.1.2
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apiserver v0.28.1
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4
	kubevirt.io/managed-tenant-quota v1.1.1
	kubevirt.io/ssp-operator/api v0.18.3
)

require (
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cilium/ebpf v0.9.1 // indirect
	github.com/coreos/prometheus-operator v0.38.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/goexpect v0.0.0-20191001010744-5b6988669ffa // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/goterm v0.0.0-20190703233501-fc88cf888a3f // indirect
	github.com/google/pprof v0.0.0-20230821062121-407c9e7a662f // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20210817203519-d82598001386 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.3.0 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/opencontainers/runc v1.1.7 // indirect
	github.com/operator-framework/api v0.17.7 // indirect
	github.com/operator-framework/operator-lib v0.11.1-0.20220921174810-791cc547e6c5 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/rivo/uniseg v0.4.3 // indirect
	github.com/sirupsen/logrus v1.9.2 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/term v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.12.1-0.20230815132531-74c255bcf846 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19 // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.1 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-aggregator v0.27.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230816210353-14e408962443 // indirect
	k8s.io/kubectl v0.26.1 // indirect
	sigs.k8s.io/controller-runtime v0.16.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
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

// Pinned to v0.26.3
replace (
	k8s.io/api => k8s.io/api v0.26.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.3
	k8s.io/apiserver => k8s.io/apiserver v0.26.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.3
	k8s.io/client-go => k8s.io/client-go v0.26.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.3
	k8s.io/code-generator => k8s.io/code-generator v0.26.3
	k8s.io/component-base => k8s.io/component-base v0.26.3
	k8s.io/cri-api => k8s.io/cri-api v0.26.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.3
	k8s.io/kubectl => k8s.io/kubectl v0.26.3
	k8s.io/kubelet => k8s.io/kubelet v0.26.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.3
	k8s.io/metrics => k8s.io/metrics v0.26.3
	k8s.io/node-api => k8s.io/node-api v0.26.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.3
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.26.3
	k8s.io/sample-controller => k8s.io/sample-controller v0.26.3
)

replace (
	github.com/appscode/jsonpatch => github.com/appscode/jsonpatch v1.0.1
	github.com/coreos/prometheus-operator/pkg/apis/monitoring => github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.52.1
	github.com/go-kit/kit => github.com/go-kit/kit v0.12.0
	github.com/kubevirt/cluster-network-addons-operator => github.com/kubevirt/cluster-network-addons-operator v0.89.0
	github.com/kubevirt/cluster-network-addons-operator/pkg/apis => github.com/kubevirt/cluster-network-addons-operator/pkg/apis v0.89.0
	github.com/openshift/machine-api-operator => github.com/openshift/machine-api-operator v0.2.1-0.20191025120018-fb3724fc7bdf
	go.mongodb.org/mongo-driver => go.mongodb.org/mongo-driver v1.5.1
	kubevirt.io/containerized-data-importer-api => kubevirt.io/containerized-data-importer-api v1.57.0
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20230825144922-938af62eda38
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20230807132528-be5346fb33cb
	github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20230809121909-d7e7beca5bae
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.2
)

replace github.com/docker/docker => github.com/moby/moby v1.4.2-0.20200203170920-46ec8731fbce // Required by Helm

replace vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787

replace bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d

// Fixes various security issues forcing newer versions of affected dependencies,
// prune the list once not explicitly required
replace (
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.5.0
	github.com/kubernetes-csi/external-snapshotter/v2 => github.com/kubernetes-csi/external-snapshotter/v2 v2.1.3
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e
	golang.org/x/crypto/ssh => golang.org/x/crypto/ssh v0.0.0-20220525230936-793ad666bf5e
	golang.org/x/crypto/ssh/terminal => golang.org/x/crypto/ssh/terminal v0.0.0-20220525230936-793ad666bf5e
)

// FIX: Unhandled exception in gopkg.in/yaml.v3
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1
