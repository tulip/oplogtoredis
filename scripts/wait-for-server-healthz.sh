#!/bin/sh

ATTEMPT_COUNTER=0
MAX_ATTEMPTS=15
echo "Waiting for server to come up"
until $(curl --output /dev/null --silent --head --fail localhost:9000/healthz); do
  if [ ${ATTEMPT_COUNTER} -eq ${MAX_ATTEMPTS} ];then
    echo "Max attempts reached"
    grep 'docker\|lxc' /proc/1/cgroup &> /dev/null
    if [ "$?" != "0" ]; then
      echo "Not running in docker, check that something like minio isn't bound to port 9000 already"
    fi
    exit 3
  fi
  curl -I localhost:9000/healthz --head 2>&1 | grep "403 Forbidden" >/dev/null
  if [ "$?" = "0" ]; then
    echo "Some other web server (probably minio) is listening at port 9000, try shutting it down"
    exit 3
  fi
  printf '.'
  sleep 1
  ATTEMPT_COUNTER=$(($ATTEMPT_COUNTER+1))
done
echo "Successfully hit healthz endpoint"
