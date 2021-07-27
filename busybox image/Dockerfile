
FROM registry.access.redhat.com/ubi8/ubi-minimal

LABEL name="Busybox init container" \
      vendor="SumoLogic" \
      version="1.0" \
      release="1" \
      summary="Busybox init container to run falco" \
      description="This will ...."

COPY licenses /licenses

ENV USER_ID 1000
RUN microdnf install nc shadow-utils
ENV USER_NAME app-user
RUN adduser --uid $USER_ID $USER_NAME
USER $USER_ID