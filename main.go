package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/log"

	"go.uber.org/zap"

	"github.com/globalsign/mgo"
	"github.com/go-redis/redis"
	"github.com/rwynn/gtm"
)

var wg sync.WaitGroup

type redisPub struct {
	channel string
	msg     []byte
}

func main() {
	defer log.RawLog.Sync()

	err := config.ParseEnv()
	if err != nil {
		panic("Error parsing environment variables: " + err.Error())
	}

	// We crate two goroutines:
	//
	// The readOplog goroutine reads messages from the oplog, and generates the
	// messages that we need to write to redis. It then writes them to a
	// buffered channel.
	//
	// The writeMessages goroutine reads messages from the buffered channel
	// and sends them to Redis.
	//
	// TODO PERF: Use a leaky buffer (https://github.com/tulip/oplogtoredis/issues/2)
	redisPubs := make(chan *redisPub, 10000)

	wg.Add(1)
	go readOplog(redisPubs, config.MongoURL())

	wg.Add(1)
	go writeMessages(redisPubs, config.RedisURL())

	// This won't ever return; it's just here so we keep running forever
	wg.Wait()
}

// Goroutine to read from the oplog and write messages to be published to Redis
func readOplog(redisPubs chan *redisPub, mongoURL string) {
	// Struct that matches the message format redis-oplog expects
	type outgoingMessageDocument struct {
		ID string `json:"_id"`
	}
	type outgoingMessage struct {
		Event  string                  `json:"e"`
		Doc    outgoingMessageDocument `json:"d"`
		Fields []string                `json:"f"`
	}

	defer wg.Done()

	// get a mgo session
	session, err := mgo.Dial(config.MongoURL())
	if err != nil {
		panic(err)
	}
	defer session.Close()

	session.SetMode(mgo.Monotonic, true)

	// Use gtm to tail to oplog
	//
	// TODO PERF: benchmark other oplog tailers (https://github.com/tulip/oplogtoredis/issues/3)
	//
	// TODO: pick up where we left off on restart (https://github.com/tulip/oplogtoredis/issues/4)
	ctx := gtm.Start(session, &gtm.Options{
		ChannelSize:       10000,
		BufferDuration:    100 * time.Millisecond,
		UpdateDataAsDelta: true,
		WorkerCount:       8,
	})
	defer ctx.Stop()

	for {
		// loop forever receiving events
		select {
		case err = <-ctx.ErrC:
			// Log errors we receive from mgo
			//
			// TODO TESTING: Test mongo failure modes (https://github.com/tulip/oplogtoredis/issues/5)
			log.RawLog.Error("Error tailing oplog", zap.Error(err))
		case op := <-ctx.OpC:
			log.Log.Debugw("Got oplog entry", "op", op)

			// Process an oplog entry
			//
			// TODO PERF: Add options for filtering to specific collections or
			// databases (https://github.com/tulip/oplogtoredis/issues/8)
			id, idOK := op.Id.(string)
			if !idOK {
				// TODO: Handle ObjectIDs (https://github.com/tulip/oplogtoredis/issues/9)
				log.Log.Errorw("op.ID was not a string",
					"id", op.Id)
				continue
			}

			// Construct the JSON we're going to send to Redis
			//
			// TODO PERF: consider a specialized JSON encoder
			// https://github.com/tulip/oplogtoredis/issues/13
			msg := outgoingMessage{
				Event:  eventNameForOperation(op),
				Doc:    outgoingMessageDocument{id},
				Fields: fieldsForOperation(op),
			}
			log.Log.Debugw("Sending outgoing message", "message", msg)
			msgJSON, err := json.Marshal(&msg)

			if err != nil {
				log.Log.Error("Error marshalling outgoing message",
					"msg", msg)

				continue
			}

			// We need to publish on both the full-collection channel and the
			// single-document channel
			redisPubs <- &redisPub{
				channel: op.Namespace,
				msg:     msgJSON,
			}
			redisPubs <- &redisPub{
				channel: op.Namespace + "::" + id,
				msg:     msgJSON,
			}
		}
	}
}

func eventNameForOperation(op *gtm.Op) string {
	if op.Operation == "d" {
		return "r"
	}
	return op.Operation
}

type updateType int

const (
	updateTypeUnknown updateType = iota
	updateTypeModification
	updateTypeReplacement
)

// Given a gtm.Op, returned the fields affected by that operation
//
// TODO: test this against more complicated mutations (https://github.com/tulip/oplogtoredis/issues/10)
// TODO TESTING: unit tests for this
func fieldsForOperation(op *gtm.Op) []string {
	if op.IsInsert() {
		return mapKeys(op.Data)
	} else if op.IsUpdate() {
		var fields []string
		opType := updateTypeUnknown

		for operationKey, operation := range op.Data {
			if operationKey == "$v" {
				// $v indicates the version of the update language and should be
				// ignored; it will likely be removed in a future version of
				// Mongo (https://jira.mongodb.org/browse/SERVER-32240)
				continue
			}

			if strings.HasPrefix(operationKey, "$") {
				// It's an update operator -- extract the changed fields
				// from the operation
				if opType == updateTypeReplacement {
					log.Log.Errorw("Oplog data for update contained a mix of $-prefix and non-$-prefix fields",
						"op.Data", op.Data)
				} else if opType == updateTypeUnknown {
					opType = updateTypeModification
				}

				operationMap, operationMapOK := operation.(map[string]interface{})
				if !operationMapOK {
					log.Log.Errorw("Oplog data for update contained $-prefixed key with a non-map value",
						"op.Data", op.Data)
					continue
				}

				fields = append(fields, mapKeys(operationMap)...)
			} else {
				// We're dealing with a replacement update that doesn't use $-operators;
				// the keys of the data are the changed fields.
				if opType == updateTypeModification {
					log.Log.Errorw("Oplog data for update contained a mix of $-prefix and non-$-prefix fields",
						"op.Data", op.Data)
				} else if opType == updateTypeUnknown {
					// TODO (https://github.com/tulip/oplogtoredis/issues/17):
					// Add a flag to the outgoing message indicating a replacement once
					// https://github.com/cult-of-coders/redis-oplog/issues/280 lands
					opType = updateTypeReplacement
				}

				fields = append(fields, operationKey)
			}
		}

		return fields
	}

	return []string{}
}

// Given a map, returns the keys of that map
//
// TODO TESTING: unit tests for this
func mapKeys(m map[string]interface{}) []string {
	fields := make([]string, len(m))

	i := 0
	for key := range m {
		fields[i] = key
		i++
	}

	return fields
}

// Goroutine that just reads messages and sends them to Redis. We don't do this
// inline above so that messages can queue up in the channel if we lose our
// redis connection
func writeMessages(redisPubs chan *redisPub, redisURL string) {
	defer wg.Done()

	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		panic("Could not parse Redis URL: " + err.Error())
	}

	client := redis.NewClient(redisOpts)
	defer client.Close()

	for {
		p := <-redisPubs

		// TODO TESTING: test Redis failure modes (https://github.com/tulip/oplogtoredis/issues/11)
		//
		// TODO: add an HA mode (https://github.com/tulip/oplogtoredis/issues/12)
		client.Publish(p.channel, p.msg)
	}
}
