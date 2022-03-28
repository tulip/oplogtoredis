package harness

import (
	"os"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
)

var server1Conn *DDPConn
var server2Conn *DDPConn

// Start connects to both Meteor servers and returns (server 1 conn, server 2 conn)
func Start() (*DDPConn, *DDPConn) {
	server1Conn, server2Conn = StartWithFixtures(helpers.DBData{})

	return server1Conn, server2Conn
}

// StartWithFixtures is like Start, but seeds the Mongo DB with a given set of
// records
func StartWithFixtures(fixtures helpers.DBData) (*DDPConn, *DDPConn) {
	helpers.SeedTestDB(fixtures)

	server1Conn = newDDPConn(os.Getenv("TESTAPP_1_URL"))
	server2Conn = newDDPConn(os.Getenv("TESTAPP_2_URL"))

	return server1Conn, server2Conn
}

// Stop disconnects from both meteor servers
func Stop() {
	server1Conn.Close()
	server2Conn.Close()
}
