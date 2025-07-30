FROM docker.io/golang:alpine AS builder
ARG GARM_REF

LABEL stage=builder

RUN apk add --no-cache musl-dev gcc libtool m4 autoconf g++ make libblkid util-linux-dev git linux-headers upx curl jq
RUN git config --global --add safe.directory /build && git config --global --add advice.detachedHead false

ADD . /build/garm

RUN git -C /build/garm checkout ${GARM_REF}
RUN cd /build/garm \
    && go build -o /bin/garm \
      -tags osusergo,netgo,sqlite_omit_load_extension \
      -ldflags "-linkmode external -extldflags '-static' -s -w -X github.com/cloudbase/garm/util/appdefaults.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" \
      /build/garm/cmd/garm && upx /bin/garm
RUN cd /build/garm/cmd/garm-cli \
    && go build -o /bin/garm-cli \
      -tags osusergo,netgo,sqlite_omit_load_extension \
      -ldflags "-linkmode external -extldflags '-static' -s -w -X github.com/cloudbase/garm/util/appdefaults.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" \
      . && upx /bin/garm-cli
RUN set -e; \
    mkdir -p /opt/garm/providers.d; \
    for repo in \
      cloudbase/garm-provider-azure \
      cloudbase/garm-provider-openstack \
      cloudbase/garm-provider-lxd \
      cloudbase/garm-provider-incus \
      cloudbase/garm-provider-aws \
      cloudbase/garm-provider-gcp \
      cloudbase/garm-provider-equinix \
      flatcar/garm-provider-linode \
      mercedes-benz/garm-provider-k8s; \
    do \
        export PROVIDER_NAME="$(basename $repo)"; \
        export PROVIDER_SUBDIR=""; \
        if [ "$GARM_REF" == "main" ]; then \
          export PROVIDER_TAG="main"; \
        else \
          export PROVIDER_TAG="$(curl -s -L https://api.github.com/repos/$repo/releases/latest | jq -r '.tag_name')"; \
        fi; \
        git clone --branch "$PROVIDER_TAG" "https://github.com/$repo" "/build/$PROVIDER_NAME"; \
        case $PROVIDER_NAME in \
        "garm-provider-k8s") \
            export PROVIDER_SUBDIR="cmd/garm-provider-k8s"; \
            export PROVIDER_LDFLAGS="-linkmode external -extldflags \"-static\" -s -w"; \
            git -C /build/garm-provider-k8s checkout v0.3.1; \
            ;; \
        "garm-provider-linode") \
            export PROVIDER_LDFLAGS="-linkmode external -extldflags \"-static\" -s -w"; \
            ;; \
        *) \
            export PROVIDER_LDFLAGS="-linkmode external -extldflags \"-static\" -s -w -X main.Version=$(git -C /build/$PROVIDER_NAME describe --tags --match='v[0-9]*' --dirty --always)"; \
            ;; \
        esac; \
        && cd "/build/$PROVIDER_NAME/$PROVIDER_SUBDIR" \
        && go build -ldflags="$PROVIDER_LDFLAGS" -o /opt/garm/providers.d/$PROVIDER_NAME . \
        && upx /opt/garm/providers.d/$PROVIDER_NAME; \
    done

FROM busybox

COPY --from=builder /bin/garm /bin/garm
COPY --from=builder /bin/garm-cli /bin/garm-cli
COPY --from=builder /opt/garm/providers.d/garm-provider-openstack /opt/garm/providers.d/garm-provider-openstack
COPY --from=builder /opt/garm/providers.d/garm-provider-lxd /opt/garm/providers.d/garm-provider-lxd
COPY --from=builder /opt/garm/providers.d/garm-provider-incus /opt/garm/providers.d/garm-provider-incus
COPY --from=builder /opt/garm/providers.d/garm-provider-azure /opt/garm/providers.d/garm-provider-azure
COPY --from=builder /opt/garm/providers.d/garm-provider-aws /opt/garm/providers.d/garm-provider-aws
COPY --from=builder /opt/garm/providers.d/garm-provider-gcp /opt/garm/providers.d/garm-provider-gcp
COPY --from=builder /opt/garm/providers.d/garm-provider-equinix /opt/garm/providers.d/garm-provider-equinix

COPY --from=builder /opt/garm/providers.d/garm-provider-k8s /opt/garm/providers.d/garm-provider-k8s
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/bin/garm", "-config", "/etc/garm/config.toml"]
