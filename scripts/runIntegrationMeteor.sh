#!/bin/bash

set -eu
cd `dirname "$0"`'/../integration-tests/meteor'

docker compose rm -vf
docker compose down -v
docker compose up \
    --build \
    --exit-code-from test \
    --abort-on-container-exit
