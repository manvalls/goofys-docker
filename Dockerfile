# Deps download

FROM alpine:latest AS deps
RUN apk update

WORKDIR /
RUN apk add ca-certificates
RUN update-ca-certificates
RUN apk add wget

RUN wget http://github.com/kahing/catfs/releases/download/v0.6.0/catfs
RUN chmod a+rwx catfs

RUN wget https://github.com/kahing/goofys/releases/download/v0.0.18/goofys
RUN chmod a+rwx goofys

# Driver build

FROM golang:1.9-alpine AS driver
RUN apk update
RUN set -ex
RUN apk add --no-cache --virtual .build-deps gcc libc-dev

ADD . /go/src/github.com/manvalls/goofys-docker
WORKDIR /go/src/github.com/manvalls/goofys-docker
RUN go get
RUN go install --ldflags '-extldflags "-static"'

# Final image

FROM debian
RUN apt-get update && apt-get install -y \
    fuse musl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=deps /catfs /usr/local/bin
COPY --from=deps /goofys /usr/local/bin
COPY --from=driver /go/bin/goofys-docker /usr/local/bin

RUN mkdir -p /run/docker/plugins /mnt/volumes
CMD ["/usr/local/bin/goofys-docker"]
