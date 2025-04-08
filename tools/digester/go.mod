module github.com/kubevirt/hyperconverged-cluster-operator/tools/digester

go 1.19

require (
	github.com/containers/image/v5 v5.29.3
	github.com/opencontainers/go-digest v1.0.0
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.1.9 // indirect
	github.com/containers/storage v1.51.0 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v24.0.7+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.3 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc5 // indirect
	github.com/opencontainers/runc v1.1.10 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/sys v0.14.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// FIX: Improper Input Validation in GoGo Protobuf
replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2

// FIX: Unhandled exception in gopkg.in/yaml.v3
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1

// FIX: golang.org/x/net/http2/h2c vulnerable to request smuggling attack
replace golang.org/x/net => golang.org/x/net v0.9.0

replace (
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.7
	github.com/ulikunitz/xz => github.com/ulikunitz/xz v0.5.11
)
