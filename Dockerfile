FROM golang:1.10.0-alpine3.7

# Install curl and rq. rq is mirrored from
# https://s3-eu-west-1.amazonaws.com/record-query/record-query/x86_64-unknown-linux-musl/rq
#
# Also install gcc, musl-dev, and sasl, which are needed to build the cgo
# parts of mgo
RUN apk add --no-cache curl gcc cyrus-sasl cyrus-sasl-dev musl-dev && \
    curl -Lo /bin/rq  https://s3.amazonaws.com/co.tulip.cdn/deps/rq  && \
    chmod +x /bin/rq

RUN mkdir -p /go/src/github.com/tulip/oplogtoredis
WORKDIR /go/src/github.com/tulip/oplogtoredis

ADD vendor /go/src/
ADD Gopkg.lock /go/src/github.com/tulip/oplogtoredis

# We want to `go install` each of the dependencies. This allows us to leverage
# the docker cache to cache the compiled packages, and not have to re-build
# them every time the sources change, cutting our cycle time down by over 50%.
#
# This isn't something dep is designed to do: https://github.com/golang/dep/issues/1374
#
# So instead we use rq to parse out Gopkg.toml to get our list of dependencies
# and the `go install` each of these.
RUN cd /go/src && \
  cat github.com/tulip/oplogtoredis/Gopkg.lock | rq -tJ 'map "projects" | spread | map (o) => {o.packages.map(p => o.name + "/" + p)} | spread' | cat | tr -d '"' | xargs go install

ADD *.go ./
ADD ./lib ./lib

RUN go build -o app

# We're using a multistage build -- the previous stage has the full go toolchain
# so it can do the build, and this stage is just a minimal Alpine image that we
# copy the statically-linked binary into to keep our image small.
FROM alpine:3.6

# We need CA certificates to be able to validate Twilio's HTTPS cert
RUN apk add --no-cache ca-certificates

COPY --from=0 /go/src/github.com/tulip/oplogtoredis/app /bin/oplogtoredis
CMD /bin/oplogtoredis

ENV PORT 8080
EXPOSE 8080
