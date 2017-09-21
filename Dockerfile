FROM golang:alpine as goofys

ENV GOPATH=/tmp/go Goofys_docker="github.com/haibin-fx/goofys-docker"

COPY . $GOPATH/src/$Goofys_docker
WORKDIR $GOPATH/src/$Goofys_docker
RUN apk add --update gcc libc-dev git
RUN go get $Goofys_docker

#    $$ go get $Goofys_docker

FROM alpine:latest as catfs
RUN apk --update add cargo make fuse-dev git
RUN cd $HOME \
    && git clone https://github.com/kahing/catfs.git
RUN cd $HOME/catfs \
    && cargo install catfs


FROM alpine:latest
RUN apk add --update ca-certificates fuse syslog-ng llvm-libunwind
COPY --from=goofys /tmp/go/bin/goofys-docker /usr/local/bin
COPY --from=catfs root/.cargo/bin/catfs /usr/local/bin

RUN mkdir -p /run/docker/plugins /mnt/volumes
CMD ["/usr/local/bin/goofys-docker"]

