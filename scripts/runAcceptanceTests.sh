#!/bin/sh

# Runs the acceptance tests by building a docker image out of the source files,
# and then using docker-compose to spin up the built docker image with
# needed resources (e.g. Mongo), and a container that runs the actual
# acceptance tests

set -e
cd `dirname "$0"`'/../acceptance'

# In CI we want to use the Concourse-built artifact rather than re-building, so
# we allow you to skip the docker build
if [ -n "$SKIP_SERVICE_BUILD" ]; then
    echo "Skipping build of oplogtoredis:latest"
else
    docker build .. -t oplogtoredis:latest
fi

# Use docker-compose to spin up the test environment
docker-compose build && docker-compose rm -vf && docker-compose up --exit-code-from test --abort-on-container-exit
