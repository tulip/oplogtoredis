# This expects to be run with the root of the repo as the context directory

FROM ubuntu:xenial-20190222

# Install add-apt-repository
# Add mongo, redis, and go repos
# Install mongo, redis, and go
RUN apt-get update && \
    apt-get install -y software-properties-common apt-transport-https && \
    add-apt-repository -y ppa:gophers/archive && \
    apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 2930ADAE8CAF5059EE73BB4B58712A2291FA4AD5 && \
    echo "deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu xenial/mongodb-org/3.6 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-3.6.list && \
    apt-get update && \
    apt-get install -y \
        golang-1.10-go \
        mongodb-org=3.6.11 \
        mongodb-org-server=3.6.11 \
        mongodb-org-shell=3.6.11 \
        mongodb-org-mongos=3.6.11 \
        mongodb-org-tools=3.6.11 \
        redis-server=2:3.0.6-1ubuntu0.3

ENV PATH="/usr/lib/go-1.10/bin:${PATH}"
ENV GOPATH="/go"

RUN mkdir -p /go/src/github.com/tulip/oplogtoredis

ADD ./vendor /go/src/github.com/tulip/oplogtoredis/vendor
ADD ./lib /go/src/github.com/tulip/oplogtoredis/lib
ADD ./*.go /go/src/github.com/tulip/oplogtoredis

WORKDIR /go/src/github.com/tulip/oplogtoredis

# Build the app
RUN go build -o app && mv ./app /bin/oplogtoredis

# Set up CMD
ADD ./integration-tests/fault-injection /go/src/github.com/tulip/oplogtoredis/integration-tests/fault-injection
ADD ./integration-tests/helpers /go/src/github.com/tulip/oplogtoredis/integration-tests/helpers

WORKDIR /go/src/github.com/tulip/oplogtoredis/integration-tests/fault-injection
CMD go test . -timeout 5m -v


