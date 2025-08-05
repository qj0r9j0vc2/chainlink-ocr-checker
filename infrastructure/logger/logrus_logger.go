package logger

import (
	"os"

	"chainlink-ocr-checker/domain/interfaces"
	"github.com/sirupsen/logrus"
)

// logrusLogger implements the Logger interface using logrus
type logrusLogger struct {
	logger *logrus.Entry
}

// NewLogrusLogger creates a new logrus-based logger
func NewLogrusLogger(level string) interfaces.Logger {
	log := logrus.New()
	
	// Set output
	log.SetOutput(os.Stdout)
	
	// Set format
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   false,
	})
	
	// Set level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)
	
	return &logrusLogger{
		logger: logrus.NewEntry(log),
	}
}

// Debug logs a debug message
func (l *logrusLogger) Debug(msg string, fields ...interface{}) {
	l.logger.WithFields(l.parseFields(fields...)).Debug(msg)
}

// Info logs an info message
func (l *logrusLogger) Info(msg string, fields ...interface{}) {
	l.logger.WithFields(l.parseFields(fields...)).Info(msg)
}

// Warn logs a warning message
func (l *logrusLogger) Warn(msg string, fields ...interface{}) {
	l.logger.WithFields(l.parseFields(fields...)).Warn(msg)
}

// Error logs an error message
func (l *logrusLogger) Error(msg string, fields ...interface{}) {
	l.logger.WithFields(l.parseFields(fields...)).Error(msg)
}

// Fatal logs a fatal message and exits
func (l *logrusLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.WithFields(l.parseFields(fields...)).Fatal(msg)
}

// WithFields returns a logger with additional fields
func (l *logrusLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return &logrusLogger{
		logger: l.logger.WithFields(fields),
	}
}

// WithError returns a logger with an error field
func (l *logrusLogger) WithError(err error) interfaces.Logger {
	return &logrusLogger{
		logger: l.logger.WithError(err),
	}
}

// parseFields converts variadic fields to logrus.Fields
func (l *logrusLogger) parseFields(fields ...interface{}) logrus.Fields {
	result := make(logrus.Fields)
	
	// Process pairs of key-value
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			result[key] = fields[i+1]
		}
	}
	
	return result
}