package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	. "github.com/globalsign/mgo/bson"
	"github.com/globalsign/mgo/txn"
	"github.com/stretchr/testify/require"
)

func TestTransaction(t *testing.T) {
	h := startHarness()
	defer h.stop()

	txRunner := txn.NewRunner(h.mongoClient.C("Tx"))

	err := txRunner.Run([]txn.Op{
		{
			C:      "Tx",
			Id:     "foo",
			Insert: M{"bar": "baz"},
		},
		{
			C:      "Tx",
			Id:     "foo",
			Assert: M{"bar": "baz"},
			Update: M{"bar": "quux"},
		},
	}, "abcdefghijkl", nil)
	require.NoError(t, err)

	h.verify(t, map[string][]helpers.OTRMessage{
		"tx.insert": {{
			Event: "i",
			Document: map[string]interface{}{
				"_id": "foo",
				"bar": "baz",
			},
			Fields: []string{
				"bar",
			},
		}},
		"tx.update": {{
			Event: "u",
			Document: map[string]interface{}{
				"_id": "foo",
				"bar": "quux",
			},
			Fields: []string{
				"bar",
			},
		}},
	})
}
