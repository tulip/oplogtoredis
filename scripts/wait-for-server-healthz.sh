#!/bin/sh

ATTEMPT_COUNTER=0
MAX_ATTEMPTS=15
echo "Waiting for server to come up"
until $(curl --output /dev/null --silent --head --fail localhost:9000/healthz); do
  if [ ${ATTEMPT_COUNTER} -eq ${MAX_ATTEMPTS} ];then
    echo "Max attempts reached"
    exit 3
  fi
  printf '.'
  sleep 1
  ATTEMPT_COUNTER=$(($ATTEMPT_COUNTER+1))
done
echo "Successfully hit healthz endpoint"
