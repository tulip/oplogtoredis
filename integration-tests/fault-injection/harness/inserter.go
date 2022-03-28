package harness

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// RunInserts performs a series of inserts into a Test collection.
//
// It returns an array of the _id for every successful insert
// (unsuccessful inserts are ignored)
func RunInserts(client *mongo.Database, numInserts int, frequency time.Duration) []string {
	result := []string{}

	for i := 0; i < numInserts; i++ {
		id := fmt.Sprintf("doc%d", i)

		// We set a 500ms timout for the insert: long enough that the insert will
		// succeed if Mongo is working normally, but too short for it to retry during
		// a failover
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := client.Collection("Test").InsertOne(ctx, bson.M{
			"_id": id,
		})

		if err != nil {
			log.Printf("Warning: mongo insert failed: %s", err)
		} else {
			log.Printf("Inserted doc %s", id)
			result = append(result, id)
		}

		time.Sleep(frequency)
	}

	return result
}

// BackgroundInserter represents a run of RunInserts that's running in a
// background goroutine
type BackgroundInserter struct {
	waitGroup *sync.WaitGroup
	result    []string
}

// Run100InsertsInBackground runs RunInserts in a background goroutine
// It attempts 100 inserts over at least 10 seconds.
func Run100InsertsInBackground(client *mongo.Database) *BackgroundInserter {
	return RunInsertsInBackground(client, 100, 100*time.Millisecond)
}

// RunInsertsInBackground is a more customizable version of Run100InsertsInBackground
// allowing you to set the number of inserts and how fast to perform them.
func RunInsertsInBackground(client *mongo.Database, numInserts int, frequency time.Duration) *BackgroundInserter {
	inserter := BackgroundInserter{
		waitGroup: &sync.WaitGroup{},
	}

	inserter.waitGroup.Add(1)
	go func() {
		inserter.result = RunInserts(client, numInserts, frequency)
		inserter.waitGroup.Done()
	}()

	return &inserter
}

// Result waits for the inserter to finish, and then returns the result
func (inserter *BackgroundInserter) Result() []string {
	inserter.waitGroup.Wait()

	return inserter.result
}
