set -e

mongo "$MONGO_URL" --eval 'rs.initiate({ _id: "myapp", members: [{ _id: 0, host: "mongo:27017"}] })'
go test . -bench=. -benchtime 10s -timeout 20s
