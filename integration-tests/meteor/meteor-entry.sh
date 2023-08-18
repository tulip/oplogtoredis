#!/bin/sh

set -e

export METEOR_SETTINGS="`cat /meteor-settings.json`"

sleep 5
PORT=8080 node main.js
