#!/bin/bash

# Run gofmt to ensure that all files are correctly formatted, then runs go vet
# to identify potential issues.

set -e
cd `dirname "$0"`'/..'

golangci-lint run --deadline 15m ./lib/... ./integration-tests/...

echo 'Lint passed.'
