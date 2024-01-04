ARG PARENT_IMAGE
FROM $PARENT_IMAGE AS base

FROM registry.fedoraproject.org/fedora-minimal:40@sha256:7b6b81051e9755b5869b1335859d71e624d6c69be5df4b9987cb1dd0236d5ef7 AS builder

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

