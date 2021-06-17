FROM registry.access.redhat.com/ubi8/ubi:8.3 as builder
RUN dnf update -y && dnf install -y cmake diffutils gcc gcc-c++ libpq-devel m4 make openssl-devel systemd-devel tar unzip && dnf clean all

ARG FLUENT_BIT_VER=1.7.8
ADD https://github.com/fluent/fluent-bit/archive/refs/tags/v${FLUENT_BIT_VER}.zip /source.zip
RUN mkdir -p /fluent-bit/bin /fluent-bit/etc /fluent-bit/log /tmp/src \
      && unzip /source.zip -d /tmp/src && mv /tmp/src/fluent-bit-*/* /tmp/src/ \
      && rm -rf /tmp/src/build/*

ARG BISON_VER=3.7
ARG BUSON_URL=http://ftp.gnu.org/gnu/bison
ARG FLEX_VER=2.6.4
ARG FLEX_URL=https://github.com/westes/flex/files/981163
ADD ${BUSON_URL}/bison-${BISON_VER}.tar.gz /bison/
ADD ${FLEX_URL}/flex-${FLEX_VER}.tar.gz /flex/
RUN tar -xzvf /bison/bison-${BISON_VER}.tar.gz -C /bison/ && tar -xzvf /flex/flex-${FLEX_VER}.tar.gz -C /flex/
WORKDIR /bison/bison-${BISON_VER}/
RUN ./configure && make && make install
WORKDIR /flex/flex-${FLEX_VER}/
RUN ./configure && make && make install

WORKDIR /tmp/src/build/

RUN cmake \
      -DFLB_TESTS_RUNTIME=On \
      -DFLB_TESTS_INTERNAL=On \
      -DFLB_RELEASE=On \
      -DFLB_TLS=On \
      -DFLB_OUT_TD=Off \
      -DFLB_IN_PROC=Off \
      ../ && \
      make && \
      install bin/fluent-bit /fluent-bit/bin/

RUN cp /tmp/src/conf/*.conf /fluent-bit/etc/

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.3 as production

RUN microdnf update && microdnf install -y openssl libpq systemd && microdnf clean all

COPY --from=builder /fluent-bit /fluent-bit

EXPOSE 2020
CMD ["/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.conf"]