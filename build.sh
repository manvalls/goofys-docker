#!/bin/sh
docker run -it --rm -v "$PWD":/go/src/github.com/anrim/goofys-docker -w /go/src/github.com/anrim/goofys-docker -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=0 anrim/golang:1.8-alpine bash -c "go get github.com/Masterminds/glide && glide i && go build -a -v -installsuffix cgo"
