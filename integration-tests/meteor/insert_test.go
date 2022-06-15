package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tulip/oplogtoredis/integration-tests/meteor/harness"
)

func TestInsert(t *testing.T) {
	meteor1, meteor2 := harness.Start()
	defer harness.Stop()

	// Subscribe to tasks from both servers
	require.NoError(t, meteor1.Send(harness.DDPSub("subId", "tasks")))
	require.NoError(t, meteor2.Send(harness.DDPSub("subId", "tasks")))

	meteor1.ClearReceiveBuffer()
	meteor2.ClearReceiveBuffer()

	// Call insert method on meteor1
	require.NoError(t, meteor1.Send(harness.DDPMethod("methodCallId", "tasks.insert", "some text")))

	// On meteor1, we should get added and result, and then updated
	received := meteor1.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPResult("methodCallId", harness.DDPData{}),

		// We know this ID because we pass a fixed random seed with DDP method calls
		harness.DDPAdded("tasks", harness.DDPFirstRandomID, harness.DDPData{
			"text":     "some text",
			"owner":    "testuser",
			"username": "Test User",
		}),

		harness.DDPUpdated([]string{"methodCallId"}),
	})

	received.VerifyUpdatedComesAfterAllChanges(t)

	// On meteor2, we should just get added
	meteor2.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPAdded("tasks", harness.DDPFirstRandomID, harness.DDPData{
			"text":     "some text",
			"owner":    "testuser",
			"username": "Test User",
		}),
	})
}
