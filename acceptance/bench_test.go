package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/acceptance/helpers"

	"github.com/globalsign/mgo/bson"
)

func BenchmarkInsertNoWait(b *testing.B) {
	time.Sleep(2 * time.Second)

	db := helpers.SeedTestDB(helpers.DBData{})
	defer db.Session.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.C("Foo1").Insert(bson.M{
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
	client := redisClient()
	defer client.Close()

	subscr := client.Subscribe("tests.Foo2")
	defer subscr.Close()

	db := helpers.SeedTestDB(helpers.DBData{})
	defer db.Session.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := subscr.ReceiveMessage()
			if err != nil {
				panic(err)
			}
		}
		wg.Done()
	}()

	time.Sleep(1 * time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.C("Foo2").Insert(bson.M{
			"_id":   fmt.Sprintf("testrecord%d", i),
			"hello": "world",
		})
		if err != nil {
			panic(err)
		}
	}

	wg.Wait()
}
