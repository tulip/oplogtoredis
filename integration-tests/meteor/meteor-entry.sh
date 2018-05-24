#!/bin/sh

set -e

mongo "$MONGO_URL_NO_RS" --eval 'rs.initiate({ _id: "myapp", members: [{ _id: 0, host: "mongo:27017"}] })'
export METEOR_SETTINGS="`cat /meteor-settings.json`"

sleep 5
PORT=8080 node main.js
