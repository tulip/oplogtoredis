package main

import (
	"fmt"
	"log"
)

func init() {
	// Set a prefix for our logger
	log.SetPrefix(fmt.Sprintf("[%10s] ", "test"))
}
