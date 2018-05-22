#!/bin/bash

set -eu
cd `dirname "$0"`

./runLint.sh
./runUnitTests.sh
./runIntegrationAcceptance.sh
./runIntegrationFaultInjection.sh
./runIntegrationPerformance.sh
