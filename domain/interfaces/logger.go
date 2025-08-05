// Package interfaces defines contracts and interfaces for the OCR checker domain layer.
// It contains interfaces for blockchain operations, repositories, use cases, and logging.
package interfaces

// Logger represents the logging interface.
type Logger interface {
	// Debug logs a debug message.
	Debug(msg string, fields ...interface{})
	
	// Info logs an info message.
	Info(msg string, fields ...interface{})
	
	// Warn logs a warning message.
	Warn(msg string, fields ...interface{})
	
	// Error logs an error message.
	Error(msg string, fields ...interface{})
	
	// Fatal logs a fatal message and exits.
	Fatal(msg string, fields ...interface{})
	
	// WithFields returns a logger with additional fields.
	WithFields(fields map[string]interface{}) Logger
	
	// WithError returns a logger with an error field.
	WithError(err error) Logger
}