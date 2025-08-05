// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"fmt"
	"strconv"
)

// parseInt parses a string to int.
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// parsePositiveInt parses a string to a positive int.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}