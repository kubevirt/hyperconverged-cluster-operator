# HyperConverged Cluster Operator Tests
This document describes the tests that are part of the HyperConverged Cluster Operator (HCO) repository. Please update
this document as part of test contribution.

## Unit Tests
Any change to the source code must be covered by unit tests. The HCO repository uses the
[ginkgo testing framework](https://onsi.github.io/ginkgo/) (together with the [gomega matcher/assertion library](https://onsi.github.io/gomega/)) to implement the unit tests.

The unit tests are running as part of the sanity tests and must pass in order to merge a pull request to the source code.
### Unit Test Coverage
The HCO repository uses the goverall tool to report the coverage to the [coveralls](https://coveralls.io/github/kubevirt/hyperconverged-cluster-operator) site.

The coverage of the unit test is not perfect, because the HCO repository contains a meaningful amount of auto generated
code that is hard to test. However, the coverage must not be decreased. 

### Running Unit Tests Manually
Before running the test, make sure to set the `KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION` environment variable to `v1`.

It is possible to run the unit tests using the `go test` command. To run all the unit test, run this command:
```commandline
KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go test ./pkg/...
```
It's possible to run the unit tests for a specific package, for example, the `operands` package, run:
```commandline
KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go test ./pkg/controller/operands/
```
This is also the way to run unit tests from an IDE. Then it is pretty simple to use the IDE debug tools.

However, it is recommended to use the `ginkgo` tool itself get better output and additional options. It is still required 
to set the `KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION` environment variable. The `./hack/ginkgo.sh` script is a 
ginkgo wrapper that adds this environment variable and pass all the command line parameters to the `ginkgo` tool.

Full documentation of using the `ginkgo` tool may be found in the [ginkgo web site](https://onsi.github.io/ginkgo/#the-ginkgo-cli), But 
here are some useful ways to run the unit tests:

Running all the unit tests:
```commandline
./hack/ginkgo.sh -r pkg/
```
Running unit tests for a specific package; e.g. the `webhook` package:
```commandline
./hack/ginkgo.sh -r pkg/webhooks/
```
Running the unit tests with a verbose output (in this example, running only the `controller` package):
```commandline
./hack/ginkgo.sh -r -v pkg/controller/hyperconverged/
```
## Sanity Checks
### make sanity
The `make sanity` command performs the following:
* auto generates the [API document](./api.md).
* validates that there is no usage of offensive language
* formats the golang source code (`go fmt ./...`)
* handles dependencies (`go mod tidy` and `go mod vendor`)
* build all HCO kubernetes manifest files
* check for changes - if one of the above caused a change in the local git repository, the script will fail. In this
  case review the changes and if needed, commit them and run again the `make sanity` command.

When pushing a PR, the above sanity check is running, and must pass in order to merge the PR. The PR sanity is a github
action that defined [here](../.github/workflows/pr-sanity.yaml). In addition to the
`make sanity` the PR sanity action also runs the following:
* the `golangci-lint` linter
* build applications
* run the unit tests
* build and verify the prometheus rules
* update the coveralls with the PR test coverage
* validate the operator manifest files using the operator SDK

If one of the above fails, the PR can't be merged, so as a best practice, run the relevant tools before pushing a pull
request.

## Functional and CI Tests
### kubevirt-ci Tests
### openshift-ci Tests