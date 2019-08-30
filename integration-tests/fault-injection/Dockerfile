# This expects to be run with the root of the repo as the context directory

FROM ubuntu:xenial-20190222

# Install add-apt-repository
# Add mongo, redis, and go repos
# Install mongo, redis, and go
RUN apt-get update && \
    apt-get install -y software-properties-common apt-transport-https && \
    apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 2930ADAE8CAF5059EE73BB4B58712A2291FA4AD5 && \
    echo "deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu xenial/mongodb-org/3.6 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-3.6.list && \
    apt-get update && \
    apt-get install -y \
        mongodb-org=3.6.11 \
        mongodb-org-server=3.6.11 \
        mongodb-org-shell=3.6.11 \
        mongodb-org-mongos=3.6.11 \
        mongodb-org-tools=3.6.11 \
        redis-server=2:3.0.6-1ubuntu0.4 \
        musl
        # did you know that if you build a binary with one libc and try to run it on a machine with a different one,
        # your shell will tell you `[executable]: No such file or directory.`? pretty fun thing to debug. this is why we
        # need musl--oltr and oltr-integration are built on alpine with musl, but ubuntu doesn't have it installed by
        # default.

COPY --from=local-oplogtoredis:latest   /bin/oplogtoredis                       /bin/oplogtoredis
COPY --from=oltr-integration:latest     /integration/bin/fault-injection.test   /integration/bin/fault-injection.test

CMD /integration/bin/fault-injection.test -test.timeout 5m -test.v
