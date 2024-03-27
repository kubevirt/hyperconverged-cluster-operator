ARG PARENT_IMAGE
FROM $PARENT_IMAGE AS base

FROM registry.fedoraproject.org/fedora-minimal:41@sha256:9410cf9619b8681e7cf8a10e112775156e1f82cc9cfa749290cdb5179edf5c52 AS builder

ARG SOURCE
ARG DESTINATION

COPY --from=base /manifests /manifests

RUN microdnf install -y findutils sed

RUN find /manifests -name *clusterserviceversion* \
    | xargs \
    sed -i "s,$SOURCE,$DESTINATION,g"

FROM $PARENT_IMAGE

USER 0

RUN rm -rf /manifests/* \
    && rm -rf /usr/local/registry \
    && mkdir -p /usr/local/registry \
    && touch /usr/local/registry/bundles.db \
    && chown -R 1001:1001 /usr/local/registry

USER 1001

COPY --from=builder /manifests /manifests
RUN initializer --manifests /manifests --output /usr/local/registry/bundles.db

