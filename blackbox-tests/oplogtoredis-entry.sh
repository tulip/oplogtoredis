#!/bin/sh
set -eu

/wait-for.sh mongo:27017
/wait-for.sh redis:6380
update-ca-certificates
echo "starting server"
oplogtoredis
