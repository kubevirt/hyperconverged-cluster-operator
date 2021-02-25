FROM golang:1.15.2 AS builder

RUN git clone https://github.com/ahmetb/gen-crd-api-reference-docs

WORKDIR gen-crd-api-reference-docs

RUN go build

VOLUME /go/gen-crd-api-reference-docs/apidocs

ENTRYPOINT ["./gen-crd-api-reference-docs"]

CMD ["-config", "./example-config.json",  "-api-dir", "../src/github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1", "-out-file", "./apidocs/api.md"]