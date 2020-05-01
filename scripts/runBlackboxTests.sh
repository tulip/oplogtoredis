#!/bin/bash
set -eu
# Currently the only test is whether oplogtoredis can connect
# to a redis instance through TLS

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd $(dirname $SCRIPT_DIR)

CERTS_DIR=$(pwd)"/blackbox-tests/certificates"
mkdir -p $CERTS_DIR

function cleanup() {
  rm -rf blackbox-tests/certificates
  docker-compose -f blackbox-tests/docker-compose.yml down --volumes
}

# # GENERATE CERTS FOR ENABLING TLS
docker build -f blackbox-tests/Dockerfile.cert-generator blackbox-tests -t cert-generator
docker run \
  --mount type=bind,source=$CERTS_DIR,target=/shared-volume \
  cert-generator

# BRING UP OPLOGTOREDIS
docker-compose -f blackbox-tests/docker-compose.yml up -d --build

# WAIT FOR OPLOGTOREDIS TO START
# network=host only works on linux, so if running from a mac/windows
# please install curl
if [[ -x "$(command -v curl)" ]]; then
  ./scripts/wait-for-server-healthz.sh
else
  docker build -f blackbox-tests/Dockerfile.curl . -t wait-for-server-healthz
  docker run --network="host" wait-for-server-healthz
fi

# INSERT DATA INTO MONGO
docker-compose -f blackbox-tests/docker-compose.yml exec -T \
  mongo sh -c \
  'mongo --eval "db.products.insert( { item: \"card\", qty: 15 } )"'

# CHECK REDIS HAS DATA
# xargs is a hack to get rid of whitespace
# if oplogtoredis transmitted oplog to redis, we should have 2 entries
RESULT=$(
  docker-compose -f blackbox-tests/docker-compose.yml exec -T \
    redis sh -c \
    'redis-cli --scan --pattern "*" | wc -l | xargs')

echo "Num keys received: $RESULT"
if [[  $RESULT != 2 ]] ; then
  echo "Test failed, number of inserted keys does not match expected value"
  echo "cleaning up: running docker-compose down"
  cleanup
  exit 3
else
  echo "Test succeeded"
fi

cleanup
