#!/bin/sh

# Runs unit tests via go test

set -e
cd `dirname "$0"`'/..'

go test . ./lib/...
