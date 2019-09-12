#!/bin/bash

set -eu
cd "$(dirname "$0")/../integration-tests/acceptance"

# Use docker-compose to spin up the test environment
mongo_tags=( "3.6.11" "3.4.14" "3.2.19" "4.0.12" "4.2.0" )
redis_tags=( "3.2.4" "4.0.9" )
otr_dockerfiles=( "Dockerfile" "Dockerfile.racedetector" )

for mongo_tag in "${mongo_tags[@]}"; do
    for redis_tag in "${redis_tags[@]}"; do
        for otr_dockerfile in "${otr_dockerfiles[@]}"; do
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

            MONGO_ARGS=""

            read -r -d . MAJOR <<<"$MONGO_TAG"
            if [[ $MAJOR -lt 4 ]]; then
                MONGO_ARGS="--smallfiles"
            fi

            export MONGO_ARGS

            docker-compose rm -vf
            docker-compose down -v
            docker-compose up \
                --build \
                --exit-code-from test \
                --abort-on-container-exit
        done
    done
done
