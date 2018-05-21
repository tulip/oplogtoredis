package helpers

import (
	"fmt"
	"os"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// DBData is a map of [collection name] -> [array of bson.M records]
type DBData map[string][]bson.M

// SeedTestDB clears the testing DB and then re-seeds it with the given records.
// Returns an mgo.Database for doing any additional setup or assertions.
//
// The givens records is a map of [collection name] -> [array of records]
func SeedTestDB(records DBData) *mgo.Database {
	dialInfo, err := mgo.ParseURL(os.Getenv("MONGO_URL"))
	if err != nil {
		panic("Could not parse MONGO_URL")
	}

	dbName := dialInfo.Database
	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		panic("Could not connect to Mongo")
	}

	db := session.DB(dbName)

	err = db.DropDatabase()
	if err != nil {
		panic("Failed to drop the database")
	}

	for collectionName, collectionRecords := range records {
		if len(collectionRecords) == 0 {
			continue
		}

		bulk := db.C(collectionName).Bulk()

		// We have to convert []bson.M to []interface{} (see https://golang.org/doc/faq#convert_slice_of_interface)
		ifaceRecords := make([]interface{}, len(collectionRecords))
		for i, r := range collectionRecords {
			ifaceRecords[i] = r
		}
		bulk.Insert(ifaceRecords...)

		_, err := bulk.Run()
		if err != nil {
			panic(fmt.Sprintf("Failed to insert documents into collection %s", collectionName))
		}
	}

	return db
}
