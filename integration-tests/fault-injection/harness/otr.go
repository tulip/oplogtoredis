package harness

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"

	promdata "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// OTRProcess represents a running oplogtoredis process
type OTRProcess struct {
	env  []string
	cmd  *exec.Cmd
	port int
}

// StartOTRProcess starts a Redis proc and returns a OTRProcess for further
// operations
func StartOTRProcess(mongoURL string, redisURL string, port int) *OTRProcess {
	return StartOTRProcessWithEnv(mongoURL, redisURL, port, []string{})
}

// StartOTRProcessWithEnv is like StartOTRProcess, but lets you customize
// the environment variables for oplogtoredis. OTR_MONGO_URL, OTR_REDIS_URL,
// OTR_LOG_DEBUG, and OTR_HTTP_SERVER_ADDR are always set for you, so you only
// need this function if you want to set options other than those.
func StartOTRProcessWithEnv(mongoURL string, redisURL string, port int, extraEnv []string) *OTRProcess {
	proc := OTRProcess{
		port: port,
		env: append([]string{
			"OTR_MONGO_URL=" + mongoURL,
			"OTR_REDIS_URL=" + redisURL,
			"OTR_LOG_DEBUG=true",
			"OTR_METADATA_PREFIX=" + randString(16),
			fmt.Sprintf("OTR_HTTP_SERVER_ADDR=0.0.0.0:%d", port),
		}, extraEnv...),
	}

	proc.Start()

	return &proc
}

// Start starts up the OTR process. This is automatically called by
// StartOTRProcess, so you should only need to call this if you've stopped
// the process.
func (proc *OTRProcess) Start() {
	log.Printf("Starting up oplogtoredis with HTTP on %d", proc.port)

	otrBin := os.Getenv("OTR_BIN")
	if otrBin == "" {
		otrBin = "/bin/oplogtoredis"
	}
	proc.cmd = exec.Command(otrBin) // #nosec
	proc.cmd.Stdout = makeLogStreamer(fmt.Sprintf("otr:%d", proc.port), "stdout")
	proc.cmd.Stderr = makeLogStreamer(fmt.Sprintf("otr:%d", proc.port), "stderr")
	proc.cmd.Env = proc.env
	err := proc.cmd.Start()

	if err != nil {
		panic("Error starting up OTR process: " + err.Error())
	}

	waitTCP(fmt.Sprintf("localhost:%d", proc.port))
	log.Printf("Started up oplogtoredis with HTTP on %d", proc.port)
}

// Stop kills the Redis proc.
func (proc *OTRProcess) Stop() {
	log.Printf("Stopping oplogtoredis with HTTP on %d", proc.port)

	err := proc.cmd.Process.Kill()
	if err != nil {
		panic(err)
	}

	waitTCPDown(fmt.Sprintf("localhost:%d", proc.port))

	log.Printf("Stopped oplogtoredis with HTTP on %d", proc.port)
}

// GetPromMetrics scrapes the prometheus metrics from the OTR process
func (proc *OTRProcess) GetPromMetrics() map[string]*promdata.MetricFamily {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", proc.port))
	if err != nil {
		panic(err)
	}

	metrics, err := (&expfmt.TextParser{}).TextToMetricFamilies(resp.Body)
	if err != nil {
		panic(err)
	}

	return metrics
}

func randString(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
