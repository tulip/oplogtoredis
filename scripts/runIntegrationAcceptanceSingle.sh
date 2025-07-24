#!/bin/bash

set -eu
cd "$(dirname "$0")/../integration-tests/acceptance"

# Use docker compose to spin up the test environment
mongo_tag="7.0.22"
redis_tag="7.4"
otr_dockerfile="Dockerfile.racedetector"

echo "=================================="
echo "|      ACCEPTANCE TEST RUN       |"
echo "=================================="
echo
echo "> Mongo Tag: $mongo_tag           "
echo "> Redis Tag: $redis_tag           "
echo "> Dockerfile: $otr_dockerfile     "
echo
echo "=================================="

export MONGO_TAG="$mongo_tag"
export REDIS_TAG="$redis_tag"
export OTR_DOCKERFILE="$otr_dockerfile"

export MONGO_ARGS=""

docker compose rm -vf
docker compose down -v
docker compose up \
    --build \
    --exit-code-from test \
    --abort-on-container-exit
