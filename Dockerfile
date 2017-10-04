FROM alpine:latest as goofys

ENV GOPATH=/tmp/go Goofys_docker="github.com/haibin-fx/goofys-docker"

RUN apk --update add ca-certificates fuse syslog-ng \
    && apk --update add musl-dev go git \
    && set -ex \
    && go get $Goofys_docker

#    $$ go get $Goofys_docker

RUN apk --update add cargo make fuse-dev
RUN cd $HOME \
    && git clone https://github.com/kahing/catfs.git
RUN cd $HOME/catfs \
    && cargo install catfs


FROM alpine:latest
RUN apk add --update ca-certificates fuse syslog-ng llvm-libunwind
COPY --from=goofys /tmp/go/bin/goofys-docker /usr/local/bin
COPY --from=goofys root/.cargo/bin/catfs /usr/local/bin

RUN mkdir -p /run/docker/plugins /mnt/volumes
CMD ["/usr/local/bin/goofys-docker"]

