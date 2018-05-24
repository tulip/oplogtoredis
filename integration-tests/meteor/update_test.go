package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/meteor/harness"
)

func TestUpdate(t *testing.T) {
	meteor1, meteor2 := harness.Start()
	defer harness.Stop()

	// Subscribe to tasks from both servers
	meteor1.Send(harness.DDPMethod("insertCall", "tasks.insert", "some text"))

	meteor1.Send(harness.DDPSub("subId", "tasks"))
	meteor2.Send(harness.DDPSub("subId", "tasks"))

	meteor1.ClearReceiveBuffer()
	meteor2.ClearReceiveBuffer()

	// Call update method on meteor1
	meteor1.Send(harness.DDPMethod("methodCallId", "tasks.setChecked", harness.DDPFirstRandomID, true))

	// On meteor1, we should get changed and result, and then updated
	meteor1.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPResult("methodCallId", harness.DDPData{}),

		// We know this ID because we pass a fixed random seed with DDP method calls
		harness.DDPChanged("tasks", harness.DDPFirstRandomID, harness.DDPData{
			"checked": true,
		}, []string{}),
	}, harness.DDPMsgGroup{
		harness.DDPUpdated([]string{"methodCallId"}),
	})

	// On meteor2, we should just get changed
	meteor2.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPChanged("tasks", harness.DDPFirstRandomID, harness.DDPData{
			"checked": true,
		}, []string{}),
	})
}
