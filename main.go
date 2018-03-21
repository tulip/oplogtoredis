package main

import (
	"encoding/json"
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

	config.ParseEnv()

	// We crate two goroutines:
	//
	// The readOplog goroutine reads messages from the oplog, and generates the
	// messages that we need to write to redis. It then writes them to a
	// buffered channel.
	//
	// The writeMessages goroutine reads messages from the buffered channel
	// and sends them to Redis.
	//
	// TODO PERF: Use a technique like https://golang.org/doc/effective_go.html#leaky_buffer
	// To avoid allocating a new outgoingMessage for each incoming message
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
	// TODO PERF: benchmark gtm vs. just tailing the oplog with mgo directly.
	// Balance perf improvements against whatever reliability/retry/reconnect
	// we get from GTM
	//
	// TODO: this starts reading from the end of the oplog. We should
	// periodically write the timestamp of the last processed message to Redis,
	// so we can pick up from where we left off on restart
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
			// TODO TESTING: what kinds of error can we get here? Are there
			// errors that we should respond to by re-connecting to Mongo?
			// What happens during mongo outages and step-downs?
			log.RawLog.Error("Error tailing oplog", zap.Error(err))
		case op := <-ctx.OpC:
			// Process an oplog entry
			//
			// TODO PERF: Add options for filtering to specific collections or
			// databases.
			id, idOK := op.Id.(string)
			if !idOK {
				// TODO DOC: document that we only handle string IDs, not
				// ObjectID ids. Or maybe stringify ObjectIDs?
				log.Log.Errorw("op.ID was not a string",
					"id", op.Id)
				continue
			}

			// Construct the JSON we're going to send to Redis
			msg := outgoingMessage{
				Event:  op.Operation,
				Doc:    outgoingMessageDocument{id},
				Fields: fieldsForOperation(op),
			}
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

// Given a gtm.Op, returned the fields affected by that operation
//
// TODO TESTING: unit tests for this
func fieldsForOperation(op *gtm.Op) []string {
	if op.IsInsert() {
		return mapKeys(op.Data)
	} else if op.IsUpdate() {
		var fields []string

		for operationKey, operation := range op.Data {
			if operationKey == "$v" {
				continue
			}

			// TODO TESTING: maybe we want to whitelist operators we expect
			// instead of accepting all operators?
			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				log.Log.Errorw("Oplog data for update contained a non-map",
					"op.Data", op.Data)
				continue
			}

			// TODO TESTING: we might need to duplicate this logic:
			// https://github.com/cult-of-coders/redis-oplog/blob/209334ac7687432da54b335e16cb4c56aff6212b/lib/utils/getFields.js#L21
			//
			// to properly handle things like { $set: { 'array.1.xx' } }
			// but maybe the oplog handles this for us?
			fields = append(fields, mapKeys(operationMap)...)
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

		// TODO TESTING: what failures modes are there here? what if Redis
		// is offline? Or we're publishing too fast for it to keep up?
		//
		// TODO HA: instead of publishing we should do something like:
		//   ok := SET <unique id from oplog> true NX EX 60
		//   if ok {
		//     // message was not already published
		//     PUBLISH <channel> <msg>
		//   }
		// Maybe use a lua script to do this atomically?
		client.Publish(p.channel, p.msg)
	}
}
