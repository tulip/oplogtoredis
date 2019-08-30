#!/bin/bash

set -eu
cd `dirname "$0"`'/..'

docker build . -f Dockerfile -t local-oplogtoredis
docker build . -f Dockerfile.integration -t oltr-integration
docker build . -f integration-tests/fault-injection/Dockerfile -t oplogtoredis-fault-injection
docker run --rm -e TERM=xterm oplogtoredis-fault-injection
