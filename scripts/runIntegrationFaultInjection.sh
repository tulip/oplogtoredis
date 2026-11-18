#!/bin/bash

set -eu
cd `dirname "$0"`'/..'

# Redis versions to test; must be bookworm tags (see the fault-injection Dockerfile)
redis_tags=( "6.0-bookworm" "7.4-bookworm" )

docker build --platform=linux/amd64 . -f Dockerfile -t local-oplogtoredis
docker build --platform=linux/amd64 . -f Dockerfile.integration -t oltr-integration

for redis_tag in "${redis_tags[@]}"; do
  echo "=================================="
  echo "|   FAULT INJECTION TEST RUN     |"
  echo "=================================="
  echo
  echo "> Redis Tag: $redis_tag           "
  echo
  echo "=================================="

  # based image: redis, copies from local-oplogtoredis and oltr-integration
  docker build --platform=linux/amd64 . -f integration-tests/fault-injection/Dockerfile \
    --build-arg REDIS_TAG="$redis_tag" \
    -t oplogtoredis-fault-injection
  docker run --platform=linux/amd64 -e TERM=xterm oplogtoredis-fault-injection
done
