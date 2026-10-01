# This expects to be run with the root of the repo as the context directory

FROM redis:6.0-bullseye
# Debian no longer serves all of bullseye-security, so it comes from Debian's snapshot archive.
RUN sed -i -E 's#http://(deb|security)\.debian\.org/debian-security#http://snapshot.debian.org/archive/debian-security/20260824T000000Z#' /etc/apt/sources.list && \
  echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99snapshot

# Install mongo, and musl (for oplogtoredis bin)
COPY scripts/install-debian-mongo.sh ./install-debian-mongo.sh
RUN apt-get update && \
  ./install-debian-mongo.sh && \
  apt-get install -y \
  jq \
  netcat \
  postgresql \
  musl && \
  rm -rf /var/lib/apt/lists/*

# If you need to move /bin/oplogtoredis around, you will have to set OTR_BIN to the path to oplogtoredis
COPY --from=local-oplogtoredis:latest   /bin/oplogtoredis                       /bin/oplogtoredis
COPY --from=oltr-integration:latest     /integration/bin/fault-injection.test   /integration/bin/fault-injection.test

CMD /integration/bin/fault-injection.test -test.timeout 5m -test.v
