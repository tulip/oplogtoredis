FROM golang:1.10.0-alpine3.7

# Install gcc, musl-dev, and sasl, which are needed to build the cgo
# parts of mgo
RUN apk add --no-cache gcc cyrus-sasl cyrus-sasl-dev musl-dev

RUN mkdir -p /go/src/github.com/tulip/oplogtoredis
WORKDIR /go/src/github.com/tulip/oplogtoredis

ADD . ./
RUN go build -o app

# We're using a multistage build -- the previous stage has the full go toolchain
# so it can do the build, and this stage is just a minimal Alpine image that we
# copy the statically-linked binary into to keep our image small.
FROM alpine:3.6

RUN apk add --no-cache ca-certificates

COPY --from=0 /go/src/github.com/tulip/oplogtoredis/app /bin/oplogtoredis
CMD /bin/oplogtoredis

ENV PORT 8080
EXPOSE 8080
