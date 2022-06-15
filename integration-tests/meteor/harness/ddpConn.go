package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kylelemons/godebug/pretty"
)

// DDPConn represents a connection to a Meteor server. It allows you to send
// messages, and make assertions about the messages that are sent back
type DDPConn struct {
	url              string
	ws               *websocket.Conn
	receivedMessages chan *DDPMsg
	writeLock        sync.Mutex
}

// DDPData is a simple alias for EJSON data
type DDPData map[string]interface{}

// DDPMsg represents a single message sent to or received from a Meteor server
type DDPMsg struct {
	DDPType string
	Data    DDPData
}

// DDPMsgGroup is a slice of *DDPMsg
type DDPMsgGroup []*DDPMsg

func newDDPConn(url string) *DDPConn {
	conn := DDPConn{
		url:              url,
		receivedMessages: make(chan *DDPMsg, 1000),
	}

	var err error
	conn.ws, _, err = websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		panic(err)
	}

	conn.handshake()
	go conn.backgroundHandler()

	return &conn
}

// Send a message to the server
func (conn *DDPConn) Send(msg *DDPMsg) error {
	conn.writeLock.Lock()
	defer conn.writeLock.Unlock()

	outgoingMessage := map[string]interface{}{
		"msg": msg.DDPType,
	}
	for k, v := range msg.Data {
		outgoingMessage[k] = v
	}

	return conn.ws.WriteJSON(outgoingMessage)
}

// VerifyReceive reads messages from the Meteor server until no messages have
// been received for 3 seconds, and then checks that the received messages match
// the expected messages.
//
// VerifyReceive takes an arbitrary number of "message groups". When checking
// against the received messages, messages within a single group can be in
// any order.
//
// For example, if we specify our expected groups as [(a b), (c d), (e)],
// then (b a d c e) would be OK, but (a c b d e), (a b c d), and (a b c d e f)
// would not be OK.
func (conn *DDPConn) VerifyReceive(t *testing.T, expectedMessageGroups ...DDPMsgGroup) DDPMsgGroup {
	actualMessages := conn.receiveAll()

	if diff := compareDDP(actualMessages, expectedMessageGroups); diff != "" {
		t.Errorf("Got incorrect messages (-got +want)\n%s", diff)
	}

	return actualMessages
}

// ClearReceiveBuffer clears the buffer of messages that will be compared on the
// next call to VerifyReceive. It doesn't return until no message have been
// received for 3 seconds.
func (conn *DDPConn) ClearReceiveBuffer() {
	conn.receiveAll()
}

func (conn *DDPConn) receiveAll() []*DDPMsg {
	actualMessages := []*DDPMsg{}

	for {
		select {
		case msg := <-conn.receivedMessages:
			actualMessages = append(actualMessages, msg)
		case <-time.After(3 * time.Second):
			return actualMessages
		}
	}
}

// Close closes a websocket
func (conn *DDPConn) Close() {
	err := conn.ws.Close()
	if err != nil {
		fmt.Printf("Error while closing websocket: %s\n", err)
	}
}

// This is run in a background goroutine for all WebsocketConns. It reads all
// DDP messages and writes them to the receivedMessages channel
func (conn *DDPConn) backgroundHandler() {
	for {
		msg, err := conn.recvDDP()
		if err != nil {
			fmt.Printf("Got error from websocket connection to %s, closing connection: %s\n", conn.url, err)

			err := conn.ws.Close()
			if err != nil {
				fmt.Printf("Error closing webocket connection to %s: %s\n", conn.url, err)
			}

			return
		}

		fmt.Printf("[%s] Received: %#v\n", conn.url, msg)
		conn.receivedMessages <- msg
	}
}

func (conn *DDPConn) handshake() {
	// Send meteor handshake
	err := conn.ws.WriteJSON(map[string]interface{}{
		"msg":     "connect",
		"version": "1",
		"support": []string{"1"},
	})
	if err != nil {
		panic(err)
	}

	// Wait for response
	msg, err := conn.recvDDP()
	if err != nil {
		panic(err)
	}
	if msg.DDPType != "connected" {
		panic(fmt.Sprintf("sent connect message, got unexpected response: %#v", msg))
	}
}

// Receives a single message from the websocket and parses it into a DDPMsg.
// Not thread-safe; only used from the background goroutine
func (conn *DDPConn) recvDDP() (*DDPMsg, error) {
	for {
		msgType, data, err := conn.ws.ReadMessage()
		if err != nil {
			return nil, err
		}

		if msgType != websocket.TextMessage {
			return nil, fmt.Errorf("Got non-text message while receiving a DDP message: %d", msgType)
		}

		var parsedData map[string]interface{}
		err = json.Unmarshal(data, &parsedData)
		if err != nil {
			return nil, err
		}

		ddpType, ok := parsedData["msg"].(string)
		if !ok {
			fmt.Printf("Got DDP message without msg field, ignoring: %#v\n", parsedData)
			continue
		}

		delete(parsedData, "msg")

		return &DDPMsg{
			DDPType: ddpType,
			Data:    parsedData,
		}, nil
	}
}

// Ensures that the given message group (which should be the set of messages
// that are sent in response to a Method) has no modification messages (added/changed/removed)
// coming *after* the "updated" message -- in DDP, the "updated" message indicates
// that all modifications resulting from the method have been sent
func (msgGroup DDPMsgGroup) VerifyUpdatedComesAfterAllChanges(t *testing.T) {
	updatedReceived := false
	for _, msg := range msgGroup {
		if msg.DDPType == "updated" {
			updatedReceived = true
			continue
		}

		if updatedReceived && (msg.DDPType == "added" || msg.DDPType == "changed" || msg.DDPType == "removed") {
			t.Errorf("Received a \"%s\" message after an \"updated\" message in message group:\n%s", msg.DDPType, pretty.Sprint(msgGroup))
		}
	}
}
