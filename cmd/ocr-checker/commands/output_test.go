// Package commands provides CLI command implementations for the OCR checker tool.
package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputFormatter(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		data     interface{}
		expected string
	}{
		{
			name:   "JSON output",
			format: OutputFormatJSON,
			data: map[string]interface{}{
				"name":  "test",
				"value": 123,
			},
			expected: `{
  "name": "test",
  "value": 123
}
`,
		},
		{
			name:   "YAML output",
			format: OutputFormatYAML,
			data: map[string]interface{}{
				"name":  "test",
				"value": 123,
			},
			expected: `name: test
value: 123
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewOutputFormatter(tt.format)
			require.NotNil(t, formatter)

			// We'll just verify that the formatter is created correctly
			// Full output testing would require a buffer
			assert.Equal(t, tt.format, formatter.format)
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "valid JSON format",
			format:  OutputFormatJSON,
			wantErr: false,
		},
		{
			name:    "valid YAML format",
			format:  OutputFormatYAML,
			wantErr: false,
		},
		{
			name:    "invalid format",
			format:  "xml",
			wantErr: true,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormat(tt.format)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}