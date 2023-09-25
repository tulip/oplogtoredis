#!/bin/bash

set -eu
cd `dirname "$0"`'/..'

docker build --platform=linux/amd64 . -f Dockerfile -t local-oplogtoredis
docker build --platform=linux/amd64 . -f Dockerfile.integration -t oltr-integration

# based image: redis, copies from local-oplogtoredis and oltr-integration
docker build --platform=linux/amd64 . -f integration-tests/fault-injection/Dockerfile -t oplogtoredis-fault-injection
docker run --platform=linux/amd64 -e TERM=xterm oplogtoredis-fault-injection
