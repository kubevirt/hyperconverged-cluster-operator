FROM registry.access.redhat.com/ubi9/python-39@sha256:81a000d48bca6b09e09a7b834e0b7c8b6ffb1907b7867e76df9b044e02f9a2ac

RUN pip3 install operator-courier
COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests

RUN operator-courier verify --ui_validate_io /manifests

ENTRYPOINT ["operator-courier"]
CMD ["--help"]
