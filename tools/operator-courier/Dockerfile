FROM registry.access.redhat.com/ubi9/python-39@sha256:28cdbc26c697050370095ff2cffa1b8403f5501720b99f3796efeee896e1727d

RUN pip3 install operator-courier
COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests

RUN operator-courier verify --ui_validate_io /manifests

ENTRYPOINT ["operator-courier"]
CMD ["--help"]
