package log

import (
	golog "log"
	"os"

	"go.uber.org/zap"
)

// Log is a zap Sugared logger (a logger with a convenient API). You'll almost
// always want to use this logger. See: https://godoc.org/go.uber.org/zap#hdr-Choosing_a_Logger
var Log *zap.SugaredLogger

// RawLog is a Zap unsugared logger (a logger that is extremely fast and type-
// safe, but has a clunkier API). See: https://godoc.org/go.uber.org/zap#hdr-Choosing_a_Logger
var RawLog *zap.Logger

// Initialize Log and RawLog
func init() {
	var logConfig zap.Config

	// The OPLOGTOREDIS_LOG_DEBUG flag controls development vs production config.
	//
	// In the development config, logs are enabled at the debug level and above,
	// DPanic-level logs really panic ("DPanic" means "panic only in development"),
	// includes stack traces on warn-level logs and above, and uses a
	// human-friendly console encoder.
	//
	// In the production config, logs are enabled at the info level and above,
	// DPanic-level logs do not panic, stack traces are only included in
	// error-level logs, and it uses a JSON encoder that lets us easily
	// ingest logs into ElasticSearch and other systems for searching large
	// volumes of logs. The production config also enables log sampling -- if
	// Zap sees more than 100 logs messages with the same level and message
	// within a second, it will stop printing that particular log message
	// until the next second (e.g. each log message is capped at 100/second) to
	// give an upper bound to the overhead of logging.
	if os.Getenv("OTR_LOG_DEBUG") != "" {
		logConfig = zap.NewDevelopmentConfig()
	} else {
		logConfig = zap.NewProductionConfig()
	}

	if os.Getenv("OTR_LOG_QUIET") != "" {
		logConfig.Level.SetLevel(zap.PanicLevel)
	}

	var err error
	RawLog, err = logConfig.Build()
	if err != nil {
		golog.Print(err)
		panic("Unable to create a logger")
	}

	Log = RawLog.Sugar()
}

// Sync writes the log to its output stream (typically stdout/stderr). This should
// be called before the program exits to ensure buffered logs are written.
//
// The best way to do this is to call defer Sync() as the very first thing
// in the main() function to guarantee that the log will be synced before
// the program exits
func Sync() {
	RawLog.Sync()
}
