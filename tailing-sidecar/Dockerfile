FROM registry.access.redhat.com/ubi8/ubi as go-builder

### Requiered Labels#####
LABEL name="Tailing Sidecar" \
      vendor="SumoLogic" \
      version="1.0" \
      release="1" \
      summary="SumoLogic tailing sidecar" \
      description="This will ...."

### Required End User Agreements### 
COPY licenses /licenses


ENV NAME=golang \
    GO_MAJOR_VERSION=1 \
    GO_MINOR_VERSION=16 \
    GO_PATCH_VERSION=5 \
    CONTAINER_NAME="rhel8/go-toolset"

# Define the VERSION environment variable in a separate step, so we can
# re-use the GO_MAJOR_VERSION, GO_MINOR_VERSION and GO_PATCH_VERSION variables. 
ENV VERSION=$GO_MAJOR_VERSION.$GO_MINOR_VERSION.$GO_PATCH_VERSION \
    SUMMARY="Platform for building and running Go Applications" \
    DESCRIPTION="Go Toolset available as a container is a base platform for \
building and running various Go applications and frameworks. \
Go is an easy to learn, powerful, statically typed language in the C/C++ \
tradition with garbage collection, concurrent programming support, and memory safety features."

LABEL summary="$SUMMARY" \
      description="$DESCRIPTION" \
      io.k8s.description="$DESCRIPTION" \
      io.k8s.display-name="Go $VERSION" \
      io.openshift.tags="builder,golang,golang${GO_MAJOR_VERSION}${GO_MINOR_VERSION},rh-golang${GO_MAJOR_VERSION}${GO_MINOR_VERSION},go" \
      com.redhat.component="go-toolset-container" \
      com.redhat.license_terms="https://www.redhat.com/en/about/red-hat-end-user-license-agreements#UBI" \
      name="$CONTAINER_NAME" \
      version="$VERSION"

RUN INSTALL_PKGS="go-toolset" && \
    yum install -y --setopt=tsflags=nodocs $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all -y \
    && yum install make -y

RUN go version

RUN mkdir /build
ADD ./out_gstdout /build/
WORKDIR /build
RUN make all

FROM  quay.io/yoza/fluent-bit-ubi:latest
ENV LOG_LEVEL=warning

COPY --from=go-builder \
  /build/out_gstdout.so \
  /tailing-sidecar/lib/

COPY conf/fluent-bit.conf \
  conf/plugins.conf \
  /fluent-bit/etc/

CMD ["/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.conf", "--quiet"]