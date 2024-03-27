FROM quay.io/fedora/fedora:41-x86_64@sha256:50490d8d94f3b5876e9625f660055339f6d322c208d72c88466c504abd3de3ea

COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests
COPY tools/operator-sdk-validate/validate-bundles.sh .

RUN ./validate-bundles.sh

ENTRYPOINT ["operator-sdk"]
CMD ["--help"]
