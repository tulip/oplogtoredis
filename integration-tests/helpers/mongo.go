package helpers

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

// DBData is a map of [collection name] -> [array of bson.M records]
type DBData map[string][]bson.M

// SeedTestDB clears the testing DB and then re-seeds it with the given records.
// Returns a mongo.Database for doing any additional setup or assertions.
//
// The givens records is a map of [collection name] -> [array of records]
func SeedTestDB(records DBData) *mongo.Database {
	cs, err := connstring.ParseAndValidate(os.Getenv("MONGO_URL"))
	if err != nil {
		panic("Could not parse MONGO_URL " + err.Error())
	}

	clientOptions := options.Client()
	clientOptions.ApplyURI(os.Getenv("MONGO_URL"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		panic("Error connecting to Mongo")
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic("Error pinging Mongo")
	}

	db := client.Database(cs.Database)

	err = db.Drop(context.Background())
	if err != nil {
		panic("Failed to drop the database")
	}

	for collectionName, collectionRecords := range records {
		if len(collectionRecords) == 0 {
			continue
		}

		// We have to convert []bson.M to []interface{} (see https://golang.org/doc/faq#convert_slice_of_interface)
		ifaceRecords := make([]interface{}, len(collectionRecords))
		for i, r := range collectionRecords {
			ifaceRecords[i] = r
		}

		_, err := db.Collection(collectionName).InsertMany(context.Background(), ifaceRecords)
		if err != nil {
			panic(fmt.Sprintf("Failed to insert documents into collection %s", collectionName))
		}
	}

	return db
}
