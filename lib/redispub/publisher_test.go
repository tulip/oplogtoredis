package redispub

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis/v7"
)

// We don't test PublishStream here -- it requires a real Redis server because
// miniredis doesn't support PUBLISH and its lua support is spotty. It gets
// tested in integration tests.

func TestPublishSingleMessageWithRetriesImmediateSuccess(t *testing.T) {
	publication := &Publication{
		CollectionChannel: "a",
		SpecificChannel:   "b",
		Msg:               []byte("asdf"),
		OplogTimestamp:    bson.MongoTimestamp(0),
	}

	callCount := 0
	publishFn := func(p *Publication) error {
		if p != publication {
			t.Errorf("Got incorrect argument to the publish function: %#v", p)
		}

		callCount++

		return nil
	}

	err := publishSingleMessageWithRetries(publication, 30, time.Second, publishFn)

	if err != nil {
		t.Errorf("Got unexpected error: %s", err)
	}

	if callCount != 1 {
		t.Errorf("Expected callCount 1, got %d", callCount)
	}
}

func TestPublishSingleMessageWithRetriesTransientFailure(t *testing.T) {
	publication := &Publication{
		CollectionChannel: "a",
		SpecificChannel:   "b",
		Msg:               []byte("asdf"),
		OplogTimestamp:    bson.MongoTimestamp(0),
	}

	callCount := 0
	publishFn := func(p *Publication) error {
		if p != publication {
			t.Errorf("Got incorrect argument to the publish function: %#v", p)
		}

		callCount++

		if callCount < 30 {
			// Fail the first 29 times
			return errors.New("Some error")
		}

		return nil
	}

	err := publishSingleMessageWithRetries(publication, 30, 0, publishFn)

	if err != nil {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestPublishSingleMessageWithRetriesPermanentFailure(t *testing.T) {
	publication := &Publication{
		CollectionChannel: "a",
		SpecificChannel:   "b",
		Msg:               []byte("asdf"),
		OplogTimestamp:    bson.MongoTimestamp(0),
	}

	publishFn := func(p *Publication) error {
		return errors.New("Some error")
	}

	err := publishSingleMessageWithRetries(publication, 30, 0, publishFn)

	if err == nil {
		t.Errorf("Expected an error, but didn't get one")
	} else if err.Error() != "sending message (retried 30 times)" {
		t.Errorf("Got wrong error: %s", err)
	}
}
func TestPeriodicallyUpdateTimestamp(t *testing.T) {
	// The code under test operates at a configurable speed (for things like
	// periodic flushing). Adjusting this value controls that speed. Making it
	// faster speeds up tests at the expense of increasing the likelihood of
	// flakes. It is set to 100x the experimentally-determined minimum for
	// a successful tet run.
	var testSpeed = 250 * time.Millisecond

	// Start up testing redis server
	redisServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer redisServer.Close()

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{redisServer.Addr()},
	})

	// Start up the periodic updater
	timestampC := make(chan bson.MongoTimestamp)
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(1)

	go func() {
		periodicallyUpdateTimestamp(redisClient, timestampC, &PublishOpts{
			MetadataPrefix: "someprefix.",
			FlushInterval:  testSpeed,
		})
		waitGroup.Done()
	}()

	key := "someprefix.lastProcessedEntry"

	// Key should be unset
	if redisServer.Exists(key) {
		t.Errorf("Key existed before first write")
	}

	// Write something
	timestampC <- bson.MongoTimestamp(1)
	time.Sleep(testSpeed / 4) // t = 0.25

	// Key should be set
	redisServer.CheckGet(t, key, "1")

	// Wait less FlushInterval and write something
	time.Sleep(testSpeed / 2) // t = 0.75
	timestampC <- bson.MongoTimestamp(2)

	// Key should not have updated
	redisServer.CheckGet(t, key, "1")

	// Wait FlushInterval and write something
	time.Sleep(testSpeed / 2) // t = 1.25
	timestampC <- bson.MongoTimestamp(3)
	time.Sleep(testSpeed / 4) // t = 1.5

	// Key should have been updated
	redisServer.CheckGet(t, key, "3")

	// Wait less than FlushInterval and write something
	time.Sleep(testSpeed / 4) // t = 1.75
	timestampC <- bson.MongoTimestamp(4)

	// Key should not have been updated (making sure that when it *was* updated, we reset the timer)
	redisServer.CheckGet(t, key, "3")

	// Wait more than FlushInterval, make sure that the timestamp eventually
	// get flushed
	time.Sleep(testSpeed * 2) // t = 3.75
	redisServer.CheckGet(t, key, "4")

	// Close the channel, make sure the flusher exists (we wait here so the test
	// will time out if it does not)
	close(timestampC)
	waitGroup.Wait()
}

func TestNilPublicationMessage(t *testing.T) {
	err := publishSingleMessageWithRetries(nil, 5, 1*time.Second, func(p *Publication) error {
		t.Error("Should not have been called")
		return nil
	})

	if err == nil {
		t.Error("Exepcted error")
	}
}
