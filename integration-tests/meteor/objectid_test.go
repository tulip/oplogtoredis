package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tulip/oplogtoredis/integration-tests/meteor/harness"
)

func TestObjectID(t *testing.T) {
	meteor1, meteor2 := harness.Start()
	defer harness.Stop()

	require.NoError(t, meteor1.Send(harness.DDPMethod("insertCall", "objectIDTest.initializeFixtures")))

	// Subscribe to objectIDTest from both servers
	require.NoError(t, meteor1.Send(harness.DDPSub("subId", "objectIDTest.pub")))
	require.NoError(t, meteor2.Send(harness.DDPSub("subId", "objectIDTest.pub")))

	meteor1.ClearReceiveBuffer()
	meteor2.ClearReceiveBuffer()

	// Call increment method on meteor1
	require.NoError(t, meteor1.Send(harness.DDPMethod("methodCallId", "objectIDTest.increment")))

	// On meteor1, we should get changed and result, and then updated
	received := meteor1.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPResult("methodCallId", harness.DDPData{}),

		harness.DDPChanged("objectIDTest", "5ae7d0042b2acc1f1796c0b6", harness.DDPData{
			"value": 1,
		}, []string{}),

		harness.DDPUpdated([]string{"methodCallId"}),
	})

	received.VerifyUpdatedComesAfterAllChanges(t)

	// On meteor2, we should just get changed
	meteor2.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPChanged("objectIDTest", "5ae7d0042b2acc1f1796c0b6", harness.DDPData{
			"value": 1,
		}, []string{}),
	})
}
