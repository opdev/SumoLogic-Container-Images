FROM registry.access.redhat.com/ubi8/ubi as builder
RUN dnf update && dnf install -y cmake diffutils gcc gcc-c++ libpq-devel m4 make openssl-devel systemd-devel tar unzip && dnf clean all

ADD https://github.com/fluent/fluent-bit/archive/refs/tags/v1.7.2.zip /source.zip
RUN mkdir -p /fluent-bit/bin /fluent-bit/etc /fluent-bit/log /tmp/src && \
    unzip /source.zip -d /tmp/src && mv /tmp/src/fluent-bit-*/* /tmp/src/ && \
    rm -rf /tmp/src/build/*

ADD http://ftp.gnu.org/gnu/bison/bison-3.7.tar.gz /bison/ && \
    https://github.com/westes/flex/files/981163/flex-2.6.4.tar.gz /flex/
RUN tar -xzvf /bison/bison-3.7.tar.gz -C /bison/ && tar -xzvf /flex/flex-2.6.4.tar.gz -C /flex/ && \
    cd /bison/bison*/ && ./configure && make && make install && \
    cd /flex/flex*/ && ./configure && make && make install

WORKDIR /tmp/src/build/
RUN cmake -DFLB_RELEASE=On -DFLB_TLS=On ../ && make && install bin/fluent-bit /fluent-bit/bin/

# Configuration files
RUN cp /tmp/src/conf/*.conf /fluent-bit/etc/

FROM registry.access.redhat.com/ubi8/ubi-minimal

RUN microdnf update && microdnf install -y openssl libpq systemd && microdnf clean all
COPY --from=builder /fluent-bit /fluent-bit
EXPOSE 2020
CMD ["/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.conf"]