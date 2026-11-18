# This expects to be run with the root of the repo as the context directory

# Redis version under test; must be a bookworm tag (the mongo install is bookworm-only)
ARG REDIS_TAG=6.0-bookworm
FROM redis:${REDIS_TAG}

# Install mongo, and musl (for oplogtoredis bin)
COPY scripts/install-debian-mongo.sh ./install-debian-mongo.sh
RUN apt-get update && \
  ./install-debian-mongo.sh && \
  apt-get install -y \
  jq \
  netcat-openbsd \
  postgresql \
  musl && \
  rm -rf /var/lib/apt/lists/*

# If you need to move /bin/oplogtoredis around, you will have to set OTR_BIN to the path to oplogtoredis
COPY --from=local-oplogtoredis:latest   /bin/oplogtoredis                       /bin/oplogtoredis
COPY --from=oltr-integration:latest     /integration/bin/fault-injection.test   /integration/bin/fault-injection.test

CMD /integration/bin/fault-injection.test -test.timeout 5m -test.v
