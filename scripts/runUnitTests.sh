#!/bin/bash

# Runs unit tests via go test

set -e
cd `dirname "$0"`'/..'

export OTR_REDIS_URL=redis://yyy
export OTR_MONGO_URL=redis://xxx
go test -race -cover -timeout 5s . ./lib/...
