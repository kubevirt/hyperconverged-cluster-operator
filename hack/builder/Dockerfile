FROM quay.io/fedora/fedora:42
RUN dnf install -y make jq git curl rsync rsync-daemon && \
    dnf clean all && \
    curl -L -O https://go.dev/dl/go1.24.7.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.24.7.linux-amd64.tar.gz && \
    rm -f go1.24.7.linux-amd64.tar.gz && \
    mkdir -p /go
COPY rsyncd.conf /etc/rsyncd.conf
ADD entrypoint.sh /usr/bin/entrypoint.sh
ENTRYPOINT ["/usr/bin/entrypoint.sh"]
ENV PATH=$PATH:/usr/local/go/bin:/go/bin \
    GOPATH=/go \
    PACKAGE=github.com/kubevirt/hyperconverged-cluster-operator \
    API_FOLDER=api \
    API_VERSION=v1beta1 \
    PROJECT_ROOT=/root/hco
# Build args for code-generator versions
ARG K8S_VER
ARG KUBEOPENAPI_VER
# Install code generation tools as root
RUN go install \
    k8s.io/code-generator/cmd/deepcopy-gen@${K8S_VER} \
    k8s.io/code-generator/cmd/defaulter-gen@${K8S_VER} && \
    go install \
    k8s.io/kube-openapi/cmd/openapi-gen@${KUBEOPENAPI_VER}
# Verify installation
RUN go version && \
    ls -l /go/bin
# Define volume for the build directory
VOLUME ["/root/hco"]
# Default directory when container starts
WORKDIR /root/hco