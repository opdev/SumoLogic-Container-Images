
FROM registry.access.redhat.com/ubi8/ubi-minimal
ENV USER_ID 1000
RUN microdnf install nc shadow-utils
ENV USER_NAME app-user
RUN adduser --uid $USER_ID $USER_NAME
USER $USER_ID