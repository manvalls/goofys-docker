FROM alpine:latest as goofys

ENV GOPATH=/tmp/go Goofys_docker="github.com/manvalls/goofys-docker"

RUN apk --update add ca-certificates fuse syslog-ng \
    && apk --update add musl-dev go git \
    && set -ex \
    && go get $Goofys_docker

WORKDIR /
RUN apk add ca-certificates
RUN update-ca-certificates
RUN apk add wget
RUN wget http://github.com/kahing/catfs/releases/download/v0.6.0/catfs
RUN chmod a+rwx catfs

FROM debian
RUN apt-get update
RUN apt-get install fuse musl ca-certificates -y
COPY --from=goofys /tmp/go/bin/goofys-docker /usr/local/bin
COPY --from=goofys /catfs /usr/local/bin

RUN mkdir -p /run/docker/plugins /mnt/volumes
CMD ["/usr/local/bin/goofys-docker"]
