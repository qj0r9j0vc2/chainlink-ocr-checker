//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestFetchCommand_E2E(t *testing.T) {
	// Skip if not in E2E mode
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E test. Set RUN_E2E_TESTS=true to run")
	}
	
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "ocr-checker-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer func() { _ = os.Remove("ocr-checker-test") }()
	
	// Create test config
	configPath := "test-config.toml"
	configContent := `
log_level = "info"
chain_id = 137
rpc_addr = "https://polygon-rpc.com"
`
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)
	defer func() { _ = os.Remove(configPath) }()
	
	// Test fetch command
	t.Run("fetch command with valid parameters", func(t *testing.T) {
		outputPath := "test-results.yaml"
		defer func() { _ = os.Remove(outputPath) }()
		
		cmd := exec.Command("./ocr-checker-test", 
			"fetch",
			"--config", configPath,
			"--output", outputPath,
			"0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce", // Example contract
			"1",    // Start round
			"10",   // End round
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", output)
		}
		
		// For E2E test, we expect it might fail due to RPC limits
		// Just check that the command runs
		assert.Contains(t, string(output), "Fetching transmissions")
	})
	
	t.Run("fetch command with invalid parameters", func(t *testing.T) {
		cmd := exec.Command("./ocr-checker-test", 
			"fetch",
			"--config", configPath,
			"invalid-address",
			"1",
			"10",
		)
		
		output, err := cmd.CombinedOutput()
		assert.Error(t, err)
		assert.Contains(t, string(output), "invalid")
	})
}

func TestParseCommand_E2E(t *testing.T) {
	// Skip if not in E2E mode
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E test. Set RUN_E2E_TESTS=true to run")
	}
	
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "ocr-checker-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer func() { _ = os.Remove("ocr-checker-test") }()
	
	// Create test input file
	inputPath := "test-input.yaml"
	testData := map[string]interface{}{
		"contract_address": "0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce",
		"start_round":      1,
		"end_round":        10,
		"transmissions": []map[string]interface{}{
			{
				"contract_address":    "0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce",
				"epoch":               1,
				"round":               1,
				"observer_index":      0,
				"transmitter_address": "0x1234567890123456789012345678901234567890",
				"block_number":        12345,
				"block_timestamp":     "2023-01-01T00:00:00Z",
			},
		},
	}
	
	data, err := yaml.Marshal(testData)
	require.NoError(t, err)
	
	err = os.WriteFile(inputPath, data, 0600)
	require.NoError(t, err)
	defer func() { _ = os.Remove(inputPath) }()
	
	// Test parse command
	t.Run("parse command with day grouping", func(t *testing.T) {
		outputPath := "test-parse-output.txt"
		
		cmd := exec.Command("./ocr-checker-test", 
			"parse",
			"--output", outputPath,
			"--format", "text",
			inputPath,
			"day",
		)
		
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Command output: %s", output)
		defer func() { _ = os.Remove(outputPath) }()
		
		// Check output file exists
		_, err = os.Stat(outputPath)
		assert.NoError(t, err)
		
		// Read and verify output
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "Observer Activity Report")
		assert.Contains(t, string(content), "Group By: day")
	})
}

func TestVersionCommand_E2E(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "ocr-checker-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer func() { _ = os.Remove("ocr-checker-test") }()
	
	// Test version command
	cmd := exec.Command("./ocr-checker-test", "version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	
	assert.Contains(t, string(output), "OCR Checker")
	assert.Contains(t, string(output), "Version:")
	assert.Contains(t, string(output), "Go Version:")
}