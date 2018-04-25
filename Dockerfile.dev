FROM golang:1.10.0-alpine3.7

ADD scripts/wait-for.sh /wait-for.sh

RUN apk --update add git openssh mongodb && \
    mkdir -p /go/src/github.com/tulip/oplogtoredis && \
    go get github.com/pilu/fresh

WORKDIR /go/src/github.com/tulip/oplogtoredis

CMD fresh
