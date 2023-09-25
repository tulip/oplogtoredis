package harness

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	// We import the old mgo.v2 package in addition to the globalsign/mgo fork
	// -- juju/replicaset still uses mgo.v2 instead of globalsign/mgo
	legacymgo "github.com/juju/mgo/v2"
	"github.com/juju/replicaset"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

// MongoServer represents a 3-node Mongo replica set running on this host
type MongoServer struct {
	Addr               string
	replSetInitialized bool
	node1              *exec.Cmd
	node2              *exec.Cmd
	node3              *exec.Cmd
	dataPrefix         string
	DBName             string
}

// StartMongoServer starts a mongo replica set and returns a
// MongoServer for further operations
func StartMongoServer() *MongoServer {
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		panic("Error making temp dir: " + err.Error())
	}

	server := MongoServer{
		Addr:       "mongodb://127.0.0.1:27001,127.0.0.1:27002,127.0.0.1:27003/testdb?replicaSet=test",
		dataPrefix: dir,
	}

	server.Start()

	return &server
}

// Start starts up the Mongo replica set. This is automatically called by
// StartMongoServer, so you should only need to call this if you've stopped
// the replica set.
//
// This function does not return until the replica set is up and ready to accept
// connections.
func (server *MongoServer) Start() {
	// Start up the nodes
	server.node1 = server.startNode("mongo1", 27001)
	server.node2 = server.startNode("mongo2", 27002)
	server.node3 = server.startNode("mongo3", 27003)

	log.Print("Started up Mongo servers")

	if !server.replSetInitialized {
		// initialize replica set
		client := server.clientNoReplLegacyMgo()

		// Initiate
		err := replicaset.Initiate(client, "127.0.0.1:27001", "test", map[string]string{})
		if err != nil {
			panic("Error initiating replica set: " + err.Error())
		}
		log.Print("Finished initiating replicaset")

		// Add first member - need to add them one at a time in mongo 4.4
		err = replicaset.Add(client, replicaset.Member{
			Address: "127.0.0.1:27002"})
		if err != nil {
			panic("Error adding replica set member 27002: " + err.Error())
		}
		// Add second member - need to add them one at a time in mongo 4.4
		err = replicaset.Add(client, replicaset.Member{
			Address: "127.0.0.1:27003"})
		if err != nil {
			panic("Error adding replica set member 27003: " + err.Error())
		}
		log.Print("Finished adding members")

		// Wait for everything to be ready
		err = replicaset.WaitUntilReady(client, 60)
		if err != nil {
			panic("Error waiting for replica set ready after initialization: " + err.Error())
		}
		log.Print("Finished waiting for replicaset ready")

		// Print the status
		status, err := replicaset.CurrentStatus(client)
		if err != nil {
			panic("Error getting status: " + err.Error())
		}

		log.Printf("Initialized replica set: %#v", status)
		server.replSetInitialized = true
	}
}

// Client returns an mongo.Client configured to talk to the replica set
func (server *MongoServer) Client() *mongo.Client {
	cs, err := connstring.ParseAndValidate(server.Addr)
	if err != nil {
		panic("Could not parse MONGO_URL " + err.Error())
	}

	server.DBName = cs.Database

	clientOptions := options.Client()

	// This is true by default in mongo 4.4, but our failover tests require failed writes
	clientOptions.SetRetryWrites(false)

	clientOptions.ApplyURI(server.Addr)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		panic("Error connecting to Mongo")
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic("Error pinging primary Mongo for read")
	}

	err = client.Ping(ctx, readpref.Secondary())
	if err != nil {
		panic("Error pinging secondary Mongo for read")
	}

	return client
}

// clientLegacyMgo returns a legacymgo.Session configured to talk to the
// replica set
func (server *MongoServer) clientLegacyMgo() *legacymgo.Session {
	dialInfo, err := legacymgo.ParseURL(server.Addr)
	if err != nil {
		panic("Error parsing mongo URL: " + err.Error())
	}

	dialInfo.Timeout = 5 * time.Second
	client, err := legacymgo.DialWithInfo(dialInfo)

	if err != nil {
		panic("Error creating Mongo client (legacy mgo): " + err.Error())
	}

	return client
}

// clientNoReplLegacyMgo returns a legacymgo.Session configured to talk to one
// of the nodes
func (server *MongoServer) clientNoReplLegacyMgo() *legacymgo.Session {
	client, err := legacymgo.DialWithInfo(&legacymgo.DialInfo{
		Addrs:   []string{"127.0.0.1:27001"},
		Direct:  true,
		Timeout: 5 * time.Second,
	})

	if err != nil {
		panic("Error creating Mongo client (legacy mgo, no repl): " + err.Error())
	}

	return client
}

// Stop shuts down the Mongo server
func (server *MongoServer) Stop() {
	if err := server.node1.Process.Kill(); err != nil {
		panic("Error shutting down node 1: " + err.Error())
	}

	if err := server.node2.Process.Kill(); err != nil {
		panic("Error shutting down node 2: " + err.Error())
	}

	if err := server.node3.Process.Kill(); err != nil {
		panic("Error shutting down node 3: " + err.Error())
	}

	// Wait for them to stop
	waitTCPDown("127.0.0.1:27001")
	waitTCPDown("127.0.0.1:27002")
	waitTCPDown("127.0.0.1:27003")
}

// StepDown triggers a step-down of the primary
func (server *MongoServer) StepDown() {
	client := server.clientLegacyMgo()

	// Do the stepdown
	err := replicaset.StepDownPrimary(client)
	if err != nil {
		panic("Error triggering stepdown: " + err.Error())
	}
}

// startNode starts a single node of a mongo cluster, panicing on failure
func (server *MongoServer) startNode(name string, port int) *exec.Cmd {
	dbPath := filepath.Join(server.dataPrefix, name)
	err := os.MkdirAll(dbPath, 0700)
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(
		"mongod",
		"--replSet=test",
		fmt.Sprintf("--dbpath=%s", dbPath),
		fmt.Sprintf("--port=%d", port),
	) // #nosec

	cmd.Stderr = makeLogStreamer(name, "stderr")
	cmd.Stdout = makeLogStreamer(name, "stdout")

	err = cmd.Start()

	if err != nil {
		panic("Error starting up mongo node: " + err.Error())
	}

	waitTCP(fmt.Sprintf("127.0.0.1:%d", port))

	return cmd
}
