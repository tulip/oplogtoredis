#!/bin/bash

set -e
cd `dirname "$0"`

./runIntegrationAcceptance.sh
./runIntegrationPerformance.sh
./runIntegrationFaultInjection.sh
./runIntegrationMeteor.sh
