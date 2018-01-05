# Catfs build

FROM rust AS catfs
RUN apt-get update
RUN apt-get -y install libfuse-dev

WORKDIR /root
RUN git clone https://github.com/manvalls/catfs.git

WORKDIR /root/catfs
RUN cargo install

# Driver build

FROM golang:1.9-alpine AS driver
RUN apk update
RUN set -ex
RUN apk add --no-cache --virtual .build-deps gcc libc-dev git

RUN go get github.com/aws/aws-sdk-go
RUN go get github.com/docker/go-plugins-helpers/volume
RUN go get github.com/jacobsa/fuse
RUN go get github.com/kahing/goofys/api
ADD . /go/src/github.com/manvalls/goofys-docker
WORKDIR /go/src/github.com/manvalls/goofys-docker
RUN go install --ldflags '-extldflags "-static"'

# Final image

FROM debian
RUN apt-get update && apt-get install -y \
    fuse musl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=catfs /usr/local/cargo/bin/catfs /usr/local/bin
COPY --from=driver /go/bin/goofys-docker /usr/local/bin

RUN mkdir -p /run/docker/plugins /var/lib/driver/catfs /var/lib/driver/goofys /var/lib/driver/cache
CMD ["/usr/local/bin/goofys-docker"]
