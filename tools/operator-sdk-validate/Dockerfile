FROM quay.io/fedora/fedora:40-x86_64@sha256:e253230648a9fae54a563a90f8e914902a6f170ea5fc9a4a1f4277ae67013a9d

COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests
COPY tools/operator-sdk-validate/validate-bundles.sh .

RUN ./validate-bundles.sh

ENTRYPOINT ["operator-sdk"]
CMD ["--help"]
