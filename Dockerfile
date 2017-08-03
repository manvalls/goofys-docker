FROM alpine:latest

ENV GOPATH=/tmp/go Goofys_docker="github.com/haibin-fx/goofys-docker"

RUN apk add --no-cache ca-certificates fuse syslog-ng \
    && apk add --no-cache --virtual=build-dependencies musl-dev go git \
    && set -ex \
    && go get $Goofys_docker \
    && cp $GOPATH/bin/goofys-docker /usr/local/bin

RUN apk del build-dependencies \
    && rm -rf /tmp/*
    
RUN mkdir -p /run/docker/plugins /mnt/volumes

CMD ["/usr/local/bin/goofys-docker"]

