package oplog

import (
	"os"
	"testing"

	"github.com/tulip/oplogtoredis/lib/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func benchmarkProcessOplogEntry(b *testing.B, op *oplogEntry) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := processOplogEntry(op); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessOplogEntryInsert(b *testing.B) {
	id, _ := primitive.ObjectIDFromHex("deadbeefdeadbeefdeadbeef")
	data, _ := bson.Marshal(bson.M{
		"_id": id, "name": "station-4", "status": "running", "count": 12,
		"updatedAt": "2026-11-08T00:00:00Z", "owner": "line-2", "tags": bson.A{"a", "b"},
		"meta": bson.M{"x": 1}, "notes": "", "active": true,
	})
	benchmarkProcessOplogEntry(b, &oplogEntry{
		DocID: id, Operation: "i", Namespace: "factory.stations", Database: "factory",
		Collection: "stations", Data: data,
	})
}

func BenchmarkProcessOplogEntryUpdate(b *testing.B) {
	os.Setenv("OTR_REDIS_URL", "redis://yyy")
	os.Setenv("OTR_MONGO_URL", "mongodb://xxx")
	if err := config.ParseEnv(); err != nil {
		b.Fatal(err)
	}
	data, _ := bson.Marshal(bson.M{"$v": 2, "diff": bson.M{"u": bson.M{"status": "idle", "count": 13}}})
	benchmarkProcessOplogEntry(b, &oplogEntry{
		DocID: "station-4", Operation: "u", Namespace: "factory.stations", Database: "factory",
		Collection: "stations", Data: data,
	})
}
