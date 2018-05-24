package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/meteor/harness"
)

func TestArrayModification(t *testing.T) {
	meteor1, meteor2 := harness.Start()
	defer harness.Stop()

	meteor1.Send(harness.DDPMethod("insertCall", "arrayTest.initializeFixtures"))

	// Subscribe to arrayTest from both servers
	meteor1.Send(harness.DDPSub("subId", "arrayTest.pub"))
	meteor2.Send(harness.DDPSub("subId", "arrayTest.pub"))

	meteor1.ClearReceiveBuffer()
	meteor2.ClearReceiveBuffer()

	// Call increment method on meteor1
	meteor1.Send(harness.DDPMethod("methodCallId", "arrayTest.increment"))

	// On meteor1, we should get changed (for both records) and result, and then updated
	expectedChange1 := harness.DDPChanged("arrayTest", "test", harness.DDPData{
		"ary": []interface{}{
			map[string]interface{}{"filter": 10, "val": 0},
			map[string]interface{}{"filter": 20, "val": 1},
			map[string]interface{}{"filter": 30, "val": 0},
			map[string]interface{}{"filter": 40, "val": 0},
		},
	}, []string{})
	expectedChange2 := harness.DDPChanged("arrayTest", "test2", harness.DDPData{
		"ary": []interface{}{
			map[string]interface{}{"filter": 0, "val": 0},
			map[string]interface{}{"filter": 10, "val": 0},
			map[string]interface{}{"filter": 20, "val": 1},
			map[string]interface{}{"filter": 30, "val": 0},
		},
	}, []string{})

	meteor1.VerifyReceive(t, harness.DDPMsgGroup{
		harness.DDPResult("methodCallId", harness.DDPData{}),

		expectedChange1,
		expectedChange2,
	}, harness.DDPMsgGroup{
		harness.DDPUpdated([]string{"methodCallId"}),
	})

	// On meteor2, we should just get changed
	meteor2.VerifyReceive(t, harness.DDPMsgGroup{
		expectedChange1,
		expectedChange2,
	})
}
