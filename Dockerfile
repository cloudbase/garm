FROM docker.io/golang:alpine

WORKDIR /root
USER root

RUN apk add musl-dev gcc libtool m4 autoconf g++ make libblkid util-linux-dev git linux-headers mingw-w64-gcc

ADD ./scripts/build-static.sh /build-static.sh
RUN chmod +x /build-static.sh

CMD ["/bin/sh"]
