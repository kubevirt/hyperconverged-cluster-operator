FROM quay.io/fedora/fedora:40-x86_64@sha256:28318fd0642cb56b13d186115ef024a2e2df9319cffca372c9dde43a6f95798b

COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests
COPY tools/operator-sdk-validate/validate-bundles.sh .

RUN ./validate-bundles.sh

ENTRYPOINT ["operator-sdk"]
CMD ["--help"]
