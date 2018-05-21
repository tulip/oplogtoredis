#!/bin/sh

set -eu
cd `dirname "$0"`'/..'

docker build . -f integration-tests/fault-injection/Dockerfile -t oplogtoredis-fault-injection
docker run --rm -e TERM=xterm oplogtoredis-fault-injection
