module github.com/kubevirt/hyperconverged-cluster-operator/tools/release-notes

go 1.20

require (
	github.com/golang/glog v1.2.0
	github.com/joho/godotenv v1.5.1
	github.com/kubevirt/hyperconverged-cluster-operator/tools/release-notes/git v1.9.0
)

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto afb1ddc0824c // indirect
	github.com/acomagu/bufpipe v1.0.4 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-git/go-git/v5 v5.10.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-github/v57 v57.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/imdario/mergo v1.0.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	golang.org/x/crypto v0.16.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/oauth2 v0.15.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	google.golang.org/appengine/v2 v2.0.5 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace github.com/kubevirt/hyperconverged-cluster-operator/tools/release-notes/git => ./git

// FIX: Denial of service in golang.org/x/text/language
replace golang.org/x/text => golang.org/x/text v0.14.0

// FIX: Uncontrolled Resource Consumption
replace golang.org/x/net => golang.org/x/net v0.19.0

// FIX: Use of a Broken or Risky Cryptographic Algorithm in golang.org/x/crypto/ssh
replace golang.org/x/crypto => golang.org/x/crypto v0.16.0
