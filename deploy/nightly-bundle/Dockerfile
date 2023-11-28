FROM quay.io/centos/centos:stream9@sha256:8845d412fc1bfcd06a0f8615dcd53acf8f8895af653e40fd95625be6b24c370b

COPY deploy/nightly-bundle/deploy.sh /deploy.sh
COPY deploy/nightly-bundle/create_docker_config.sh /create_docker_config.sh
COPY deploy/nightly-bundle/kubevirt-testing-infra.yaml /kubevirt-testing-infra.yaml
COPY deploy/nightly-bundle/kubevirt-tests-pod-spec-override.json.in /kubevirt-tests-pod-spec-override.json.in

RUN yum install -y jq sed skopeo
RUN chmod +x /deploy.sh /create_docker_config.sh
