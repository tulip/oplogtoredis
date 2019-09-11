package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"github.com/stretchr/testify/require"
	. "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// we're using the official mongo go driver because mgo was behaving weirdly re: transactions.

func TestTransaction(t *testing.T) {
	h := startHarness()
	defer h.stop()

	client, err := mongo.NewClient(options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))

	var serverStatus M
	err = client.Database("test").RunCommand(ctx, D{{"serverStatus", 1}}).Decode(&serverStatus)
	require.NoError(t, err)

	ver := strings.SplitN(serverStatus["version"].(string), ".", 2)[0]
	major, err := strconv.Atoi(ver)
	require.NoError(t, err)

	if major < 4 {
		t.Log("mongo server version out of range")
		t.SkipNow()
	}

	// not easy to explicitly create a collection with this library:
	// https://jira.mongodb.org/browse/GODRIVER-1147
	// and you can't create collections (even implicitly) in transactions, so we need to do this ahead of time
	_, err = client.Database("test").Collection("Tx").InsertOne(ctx, M{"test": "abc", "_id": "dummy"})
	require.NoError(t, err)
	h.resetMessages()

	session, err := client.StartSession()
	require.NoError(t, err)

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		tx := sc.Client().Database("test").Collection("Tx")

		_, err := tx.InsertOne(sc, M{
			"_id": "foo",
			"bar": "baz",
		})
		require.NoError(t, err)

		_, err = tx.UpdateOne(sc, M{
			"_id": "foo",
		}, M{
			"$set": M{"bar": "quux"},
		})
		require.NoError(t, err)

		return nil, nil
	})
	require.NoError(t, err)

	session.EndSession(ctx)

	expectedMsgs := []helpers.OTRMessage{
		{
			Event: "i",
			Document: map[string]interface{}{
				"_id": "foo",
			},
			Fields: []string{
				"_id",
				"bar",
			},
		},
		{
			Event: "u",
			Document: map[string]interface{}{
				"_id": "foo",
			},
			Fields: []string{
				"bar",
			},
		},
	}

	h.verify(t, map[string][]helpers.OTRMessage{
		"test.Tx":      expectedMsgs,
		"test.Tx::foo": expectedMsgs,
	})
}
