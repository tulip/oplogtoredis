package harness

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/kvz/logstreamer"
)

func makeLog(name string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("[%10s] ", name), log.Ldate|log.Ltime)
}

func makeLogStreamer(name string, stdoutOrStderr string) io.Writer {
	return logstreamer.NewLogstreamer(makeLog(name), stdoutOrStderr, false)
}
