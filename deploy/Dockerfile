FROM quay.io/openshift/origin-operator-registry:latest@sha256:694b47f0278f9696db82295aca8b4f1f43fbbec33d8c97ee4e0004a895abd7a8

WORKDIR /registry

COPY deploy/olm-catalog .

USER root

# Initialize the database
RUN ["initializer", "--manifests", "/registry/community-kubevirt-hyperconverged", "--output", "bundles.db"]
RUN ["chown", "1001", "bundles.db"]
RUN ["chmod", "-R", "g+rwx", "."]

USER 1001

# There are multiple binaries in the origin-operator-registry
# We want the registry-server
ENTRYPOINT ["registry-server"]
CMD ["--database", "bundles.db"]
