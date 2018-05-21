#!/bin/sh

set -euo pipefail
cd `dirname "$0"`

mongo "$MONGO_URL" --eval 'rs.initiate({ _id: "myapp", members: [{ _id: 0, host: "mongo:27017"}] })'
mkdir /benchresult

samples=10
i=0

while [ $i -lt $samples ]; do
  go test . -bench=. -benchtime 5s -timeout 100s | tee /benchresult/$i.out
  i=`expr $i + 1`
done

RESULT_DIR=/benchresult PASS_THRESHOLD=0.35 go run analyzeBench.go
