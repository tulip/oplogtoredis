#!/usr/bin/env bash

set -euo pipefail
cd `dirname "$0"`

mongo "$MONGO_URL" --eval 'rs.initiate({ _id: "myapp", members: [{ _id: 0, host: "mongo:27017"}] })'
mkdir /benchresult

samples=10

for i in $(seq $samples); do
  /integration/bin/performance.test -test.bench=. -test.benchtime=5s -test.timeout=100s | tee /benchresult/"$i".out
done

RESULT_DIR=/benchresult PASS_THRESHOLD=0.35 /integration/bin/analyze
