package errors

import (
	"errors"
	"fmt"
)

// Domain errors
var (
	// ErrNotFound is returned when a requested resource is not found
	ErrNotFound = errors.New("resource not found")
	
	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")
	
	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timed out")
	
	// ErrConnection is returned when a connection error occurs
	ErrConnection = errors.New("connection error")
	
	// ErrUnauthorized is returned when an operation is not authorized
	ErrUnauthorized = errors.New("unauthorized")
	
	// ErrInternal is returned when an internal error occurs
	ErrInternal = errors.New("internal error")
)

// DomainError represents a domain-specific error with context
type DomainError struct {
	Type    error
	Message string
	Details map[string]interface{}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Type, e.Message)
	}
	return e.Type.Error()
}

// Is implements errors.Is interface
func (e *DomainError) Is(target error) bool {
	return errors.Is(e.Type, target)
}

// Unwrap implements errors.Unwrap interface
func (e *DomainError) Unwrap() error {
	return e.Type
}

// NewDomainError creates a new domain error
func NewDomainError(errType error, message string) *DomainError {
	return &DomainError{
		Type:    errType,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetails adds details to the domain error
func (e *DomainError) WithDetails(key string, value interface{}) *DomainError {
	e.Details[key] = value
	return e
}

// ValidationError represents a validation error with field-specific errors
type ValidationError struct {
	Fields map[string][]string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %d fields", len(e.Fields))
}

// AddFieldError adds a field-specific error
func (e *ValidationError) AddFieldError(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string][]string)
	}
	e.Fields[field] = append(e.Fields[field], message)
}

// HasErrors returns true if there are any field errors
func (e *ValidationError) HasErrors() bool {
	return len(e.Fields) > 0
}

// BlockchainError represents a blockchain-specific error
type BlockchainError struct {
	Operation   string
	ChainID     int64
	BlockNumber uint64
	Err         error
}

// Error implements the error interface
func (e *BlockchainError) Error() string {
	return fmt.Sprintf("blockchain error during %s on chain %d at block %d: %v",
		e.Operation, e.ChainID, e.BlockNumber, e.Err)
}

// Unwrap implements errors.Unwrap interface
func (e *BlockchainError) Unwrap() error {
	return e.Err
}

// RepositoryError represents a repository-specific error
type RepositoryError struct {
	Operation string
	Entity    string
	Err       error
}

// Error implements the error interface
func (e *RepositoryError) Error() string {
	return fmt.Sprintf("repository error during %s on %s: %v",
		e.Operation, e.Entity, e.Err)
}

// Unwrap implements errors.Unwrap interface
func (e *RepositoryError) Unwrap() error {
	return e.Err
}