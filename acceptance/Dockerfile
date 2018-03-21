FROM golang:1.10.0-alpine3.7

RUN apk add --update mongodb

RUN mkdir -p /go/src/github.com/tulip/oplogtoredis/acceptance
ENV GOPATH=/go
WORKDIR /go/src/github.com/tulip/oplogtoredis/acceptance

ADD wait-for.sh /go/src/github.com/tulip/oplogtoredis/acceptance/
ADD entry.sh /go/src/github.com/tulip/oplogtoredis/acceptance/

ADD helpers /go/src/github.com/tulip/oplogtoredis/acceptance/helpers
ADD *.go /go/src/github.com/tulip/oplogtoredis/acceptance/
