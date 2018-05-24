package harness

// DDPSub constructs a DDP sub message
func DDPSub(id, name string, params ...interface{}) *DDPMsg {
	data := DDPData{
		"id":   id,
		"name": name,
	}

	if len(params) > 0 {
		data["params"] = params
	}

	return &DDPMsg{
		DDPType: "sub",
		Data:    data,
	}
}

// DDPAdded constructs a DDP added message
func DDPAdded(collection, id string, fields DDPData) *DDPMsg {
	return &DDPMsg{
		DDPType: "added",
		Data: DDPData{
			"collection": collection,
			"id":         id,
			"fields":     fields,
		},
	}
}

// DDPChanged constructs a DDP changed message
func DDPChanged(collection, id string, fields DDPData, cleared []string) *DDPMsg {
	data := DDPData{
		"collection": collection,
		"id":         id,
		"fields":     fields,
	}

	if len(cleared) > 0 {
		data["cleared"] = cleared
	}

	return &DDPMsg{
		DDPType: "changed",
		Data:    data,
	}
}

// DDPRemoved constructs a DDP removed message
func DDPRemoved(collection, id string) *DDPMsg {
	return &DDPMsg{
		DDPType: "removed",
		Data: DDPData{
			"collection": collection,
			"id":         id,
		},
	}
}

// DDPReady constructs a DDP ready message
func DDPReady(id string) *DDPMsg {
	return &DDPMsg{
		DDPType: "ready",
		Data:    DDPData{},
	}
}

// DDPMethod constructs a DDP method message
func DDPMethod(id, method string, params ...interface{}) *DDPMsg {
	data := DDPData{
		"method":     method,
		"id":         id,
		"randomSeed": DDPRandomSeed,
	}

	if len(params) > 0 {
		data["params"] = params
	}

	return &DDPMsg{
		DDPType: "method",
		Data:    data,
	}
}

// DDPResult constructs a DDP result message
func DDPResult(id string, result DDPData) *DDPMsg {
	data := DDPData{
		"id": id,
	}

	if len(result) > 0 {
		data["result"] = result
	}

	return &DDPMsg{
		DDPType: "result",
		Data:    data,
	}
}

// DDPUpdated constructs a DDP updated message
func DDPUpdated(methods []string) *DDPMsg {
	return &DDPMsg{
		DDPType: "updated",
		Data: DDPData{
			"methods": methods,
		},
	}
}
