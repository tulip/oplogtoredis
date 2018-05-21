#!/bin/sh

# Run gofmt to ensure that all files are correctly formatted, then runs go vet
# to identify potential issues.

set -e
cd `dirname "$0"`'/..'

gofmt_output=`gofmt -l *.go lib/**/*.go integration-tests/**/*.go`
if [ ! -z "$gofmt_output" ]; then
    echo 'gofmt found issues with some files: '
    echo $gofmt_output
    exit 1
fi

go vet . ./lib/... ./integration-tests/...

golint . ./lib/... ./integration-tests/...

echo 'Lint passed.'
