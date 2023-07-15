FROM docker.io/golang:alpine AS builder
ARG GARM_REF

LABEL stage=builder

RUN apk add musl-dev gcc libtool m4 autoconf g++ make libblkid util-linux-dev git linux-headers
RUN git config --global --add safe.directory /build

ADD . /build/garm
RUN cd /build/garm && git checkout ${GARM_REF}
RUN git clone https://github.com/cloudbase/garm-provider-azure /build/garm-provider-azure
RUN git clone https://github.com/cloudbase/garm-provider-openstack /build/garm-provider-openstack

RUN cd /build/garm && go build -o /bin/garm \
    -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags "-linkmode external -extldflags '-static' -s -w -X main.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" \
    /build/garm/cmd/garm
RUN mkdir -p /opt/garm/providers.d
RUN cd /build/garm-provider-azure && go build -ldflags="-linkmode external -extldflags '-static' -s -w" -o /opt/garm/providers.d/garm-provider-azure .
RUN cd /build/garm-provider-openstack && go build -ldflags="-linkmode external -extldflags '-static' -s -w" -o /opt/garm/providers.d/garm-provider-openstack .

FROM scratch

COPY --from=builder /bin/garm /bin/garm
COPY --from=builder /opt/garm/providers.d/garm-provider-openstack /opt/garm/providers.d/garm-provider-openstack
COPY --from=builder /opt/garm/providers.d/garm-provider-azure /opt/garm/providers.d/garm-provider-azure

ENTRYPOINT ["/bin/garm", "-config", "/etc/garm/config.toml"]
