FROM docker.io/golang:1.23.6 AS builder

WORKDIR /go
COPY . ./
RUN make build-annotate-dicts

FROM registry.access.redhat.com/ubi9/ubi-minimal

ENV XDG_RUNTIME_DIR=/run/user/1001 \
    ASSETS_DIR=/assets \
    ANNOTATE_DICT_BIN=annotate-dicts

VOLUME /assets
VOLUME ${XDG_RUNTIME_DIR}/containers

COPY --from=builder /go/_out/annotate-dicts /usr/bin/
COPY hack/annotate-dicts.sh /usr/bin/entrypoint.sh

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
