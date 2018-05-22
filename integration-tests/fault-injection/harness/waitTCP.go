package harness

import (
	"fmt"
	"log"
	"net"
	"time"
)

// waitTCP wait until the TCP service is accepting connections
//
// It times out and panics after 60 seconds.
func waitTCP(addr string) {
	log.Printf("Waiting for TCP to be available at %s", addr)
	// Try once a second to connect
	for startTime := time.Now(); time.Since(startTime) < 10*time.Second; time.Sleep(time.Second) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)

		if err == nil {
			// Connection successful
			log.Printf("TCP came up on %s", addr)
			closeErr := conn.Close()
			if closeErr != nil {
				log.Printf("Error closing TCP connection in waitTCP: %s", closeErr)
			}

			return
		}

		log.Printf("Tried to connect to %s, got error: %s. Will retry in 1 second.", addr, err)
	}

	// Timed out
	panic(fmt.Sprintf("Timeout out waiting for service to start on %s", addr))
}

// waitTCP wait until the TCP service is not accepting connections
//
// It times out and panics after 60 seconds.
func waitTCPDown(addr string) {
	log.Printf("Waiting for TCP to be down at %s", addr)
	// Try once a second to connect
	for startTime := time.Now(); time.Since(startTime) < 10*time.Second; time.Sleep(time.Second) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)

		if err != nil {
			// Connection failed
			log.Printf("TCP went down on %s", addr)
			return
		}

		closeErr := conn.Close()
		if closeErr != nil {
			log.Printf("Error closing TCP connection in waitTCP: %s", closeErr)
		}

		log.Printf("Tried to connect to %s, was successful. Will retry in 1 second.", addr)
	}

	// Timed out
	panic(fmt.Sprintf("Timeout out waiting for service to stop on %s", addr))
}
