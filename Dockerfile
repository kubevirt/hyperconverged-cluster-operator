# To build this, from project root run:
# docker build -t docker.io/djzager/kubevirt-operators -f Dockerfile .
FROM quay.io/openshift/origin-operator-registry

# Point this at kubevirt/kubevirt when ready
# This doesn't work because go isn't installed
# Need root in order to be able to install git
# Only need this if the manifests are stored elsewhere
#USER root
#RUN yum -y install git
#RUN git clone --branch olm https://github.com/slintes/kubevirt.git && \
#    cd kubevirt && \
#  ./hack/build-manifests.sh

COPY registry /registry

# Initialize the database
RUN initializer --manifests /registry --output bundles.db

# There are multiple binaries in the origin-operator-registry
# We want the registry-server
ENTRYPOINT ["registry-server"]
CMD ["--database", "bundles.db"]
