# Base image
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV HOME=/opt/custom-scorecard-tests \
    USER_NAME=custom-scorecard-tests \
    USER_UID=1001

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd

WORKDIR ${HOME}

COPY func-tests.test /usr/local/bin/func-tests.test
COPY main.sh /usr/local/bin/main.sh

ENTRYPOINT ["/usr/local/bin/main.sh"]

USER ${USER_UID}
