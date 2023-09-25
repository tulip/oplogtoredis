package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"go.mongodb.org/mongo-driver/bson"
)

func BenchmarkInsertNoWait(b *testing.B) {
	time.Sleep(2 * time.Second)

	db := helpers.SeedTestDB(helpers.DBData{})
	defer func() { _ = db.Client().Disconnect(context.Background()) }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Collection("Foo1").InsertOne(context.Background(), bson.M{
			"_id":   fmt.Sprintf("testrecord%d", i),
			"hello": "world",
		})
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkInsertWaitForRedis(b *testing.B) {
	// Connect to redis and start listening for the publication we expect
	client := helpers.RedisClient()
	defer client.Close()

	subscr := client.Subscribe(context.Background(), "tests.Foo2")
	defer subscr.Close()

	db := helpers.SeedTestDB(helpers.DBData{})
	defer func() { _ = db.Client().Disconnect(context.Background()) }()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := subscr.ReceiveMessage(context.Background())
			if err != nil {
				panic(err)
			}
		}
		wg.Done()
	}()

	time.Sleep(1 * time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Collection("Foo2").InsertOne(context.Background(), bson.M{
			"_id":   fmt.Sprintf("testrecord%d", i),
			"hello": "world",
		})
		if err != nil {
			panic(err)
		}
	}

	wg.Wait()
}
