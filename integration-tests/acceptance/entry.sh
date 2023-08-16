#!/bin/sh

set -e
cd `dirname "$0"`

/integration/bin/acceptance.test -test.timeout 60s
