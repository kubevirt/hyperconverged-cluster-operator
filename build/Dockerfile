FROM docker.io/golang:1.21.4@sha256:9baee0edab4139ae9b108fffabb8e2e98a67f0b259fd25283c2a084bd74fea0d AS builder

WORKDIR /go/src/github.com/kubevirt/hyperconverged-cluster-operator/
COPY . .
RUN make build-operator build-csv-merger

FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:c77792b8084ce5946c68f39024fa460ef7769c0eef3fce995e70299e21a7e166
ENV OPERATOR=/usr/local/bin/hyperconverged-cluster-operator \
    CSV_MERGER=/usr/local/bin/csv-merger \
    USER_UID=1001 \
    USER_NAME=hyperconverged-cluster-operator \
    KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1

# ensure $HOME exists and is accessible by group 0 (we don't know what the runtime UID will be)
RUN mkdir -p ${HOME} && \
    chown ${USER_UID}:0 ${HOME} && \
    chmod ug+rwx ${HOME} && \
    # runtime user will need to be able to self-insert in /etc/passwd
    chmod g+rw /etc/passwd

COPY --from=builder /go/src/github.com/kubevirt/hyperconverged-cluster-operator/_out/hyperconverged-cluster-operator $OPERATOR
COPY --from=builder /go/src/github.com/kubevirt/hyperconverged-cluster-operator/_out/csv-merger $CSV_MERGER
COPY --from=builder /go/src/github.com/kubevirt/hyperconverged-cluster-operator/assets/ ./
ENTRYPOINT $OPERATOR
USER ${USER_UID}

ARG git_url=https://github.com/kubevirt/hyperconverged-cluster-operator.git
ARG git_sha=NONE

LABEL multi.GIT_URL=${git_url} \
      multi.GIT_SHA=${git_sha}
