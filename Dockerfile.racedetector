FROM golang:1.10.0

RUN mkdir -p /go/src/github.com/tulip/oplogtoredis
WORKDIR /go/src/github.com/tulip/oplogtoredis

ADD . ./
RUN go build -o app -race && mv ./app /bin/oplogtoredis

ENV PORT 8080
EXPOSE 8080
