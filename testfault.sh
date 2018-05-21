#!/bin/bash

set -e

for i in {1..100}; do
  echo "======= START OF TEST RUN $i ======"
  ./scripts/runIntegrationPerformance.sh 2>&1 | grep Overhead
done
