#!/bin/bash

# Runs unit tests via go test

set -e
cd `dirname "$0"`'/..'

go test -race -cover -timeout 5s . ./lib/...
