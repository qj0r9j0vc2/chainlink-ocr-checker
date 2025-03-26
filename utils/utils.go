package utils

import (
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// LogLevel converts vague log level name into typed level.
func LogLevel(s string) log.Level {
	switch s {
	case "1", "error":
		return log.ErrorLevel
	case "2", "warn":
		return log.WarnLevel
	case "3", "info":
		return log.InfoLevel
	case "4", "debug":
		return log.DebugLevel
	default:
		return log.FatalLevel
	}
}

// OrShutdown fatals the app if there was an error.
func OrShutdown(err error) {
	if err != nil && err != grpc.ErrServerStopped {
		log.WithError(err).Fatalln("unable to start ocr-checker")
	}
}
