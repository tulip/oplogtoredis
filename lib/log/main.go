// Package log is a small wrapper around go.uber.org/zap to provide logging
// facilities for oplogtoredis
package log

import (
	golog "log"
	"os"
	"time"

	"github.com/tulip/oplogtoredis/lib/config"

	"github.com/TheZeroSlave/zapsentry"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log is a zap Sugared logger (a logger with a convenient API). You'll almost
// always want to use this logger. See: https://godoc.org/go.uber.org/zap#hdr-Choosing_a_Logger
var Log *zap.SugaredLogger

// RawLog is a Zap unsugared logger (a logger that is extremely fast and type-
// safe, but has a clunkier API). See: https://godoc.org/go.uber.org/zap#hdr-Choosing_a_Logger
var RawLog *zap.Logger

var defaultSentryClient *sentry.Client

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

	Log = sentryInit(RawLog).Sugar()
}

func sentryInit(log *zap.Logger) *zap.Logger {
	errConfig := config.ParseEnv()
	if errConfig != nil {
		panic("Error parsing environment variables: " + errConfig.Error())
	}
	if !config.SentryEnabled() {
		return log
	}
	errSentry := sentry.Init(sentry.ClientOptions{
		Dsn:              config.SentryDSN(),
		Environment:      config.SentryEnvironment(),
		Release:          config.SentryRelease(),
		TracesSampleRate: 1.0,
		AttachStacktrace: true,
	})
	if errSentry != nil {
		// this is called before log is initialized
		panic("Failed to initialize Sentry: " + errSentry.Error())
	}

	cfg := zapsentry.Configuration{
		Level:             zapcore.ErrorLevel,
		EnableBreadcrumbs: true,
		BreadcrumbLevel:   zapcore.WarnLevel,
		Tags: map[string]string{
			"application": "oplogtoredis",
		},
	}

	defaultSentryClient = sentry.CurrentHub().Client()

	core, err := zapsentry.NewCore(cfg, zapsentry.NewSentryClientFromClient(defaultSentryClient))

	if err != nil {
		log.Warn("Failed to initialize zapsentry, not wrapping log", zap.Error(err))
		sentry.CaptureException(err)
		return log
	}

	log = zapsentry.AttachCoreToLogger(core, log).With(zapsentry.NewScope())

	log.Info("Sentry wrapper configured")

	return log
}

// Sync writes the log to its output stream (typically stdout/stderr). This should
// be called before the program exits to ensure buffered logs are written.
//
// The best way to do this is to call defer Sync() as the very first thing
// in the main() function to guarantee that the log will be synced before
// the program exits
func Sync() {
	err := RawLog.Sync()

	if config.SentryEnabled() {
		sentry.Flush(2 * time.Second)
	}

	if err != nil {
		golog.Printf("Error syncing zap log: %s", err)
	}
}
