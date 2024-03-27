FROM registry.access.redhat.com/ubi9/python-39@sha256:195c51368e83a798b6f79c6a5d877685fdf5297a81e5211cfca747a7fca725aa

RUN pip3 install operator-courier
COPY deploy/olm-catalog/community-kubevirt-hyperconverged /manifests

RUN operator-courier verify --ui_validate_io /manifests

ENTRYPOINT ["operator-courier"]
CMD ["--help"]
