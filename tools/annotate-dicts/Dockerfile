FROM docker.io/golang:1.24.3 AS builder

WORKDIR /go
COPY . ./

RUN go build -o _out/annotate-dicts ./tools/annotate-dicts

FROM registry.access.redhat.com/ubi9/ubi-minimal

ENV XDG_RUNTIME_DIR=/run/user/1001
VOLUME /assets
VOLUME ${XDG_RUNTIME_DIR}/containers

COPY --from=builder /go/_out/annotate-dicts /usr/bin/annotate-dicts
COPY tools/annotate-dicts/entrypoint.sh /usr/bin/entrypoint.sh

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
