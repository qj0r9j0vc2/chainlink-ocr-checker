// Package config provides configuration constants and utilities for the OCR checker application.
// It contains flag constants, database configuration, and shared configuration types.
package config

import (
	"bufio"
	"chainlink-ocr-checker/repository"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io"
	"math/big"
	"net/url"
	"os"
	"time"
)

// Response represents a generic API response structure.
type Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// TransmissionsResponse represents a response containing transmission results.
type TransmissionsResponse struct {
	Result []Result `json:"result,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// Result represents a single OCR transmission result.
type Result struct {
	RoundID      string           `json:"roundId"`
	Timestamp    time.Time        `json:"timestamp"`
	Observers    []ResultObserver `json:"observers"`
	Transmitters []ResultObserver `json:"transmitters"`
}

// ResultObserver represents an observer or transmitter in a result.
type ResultObserver struct {
	Idx     int            `json:"idx"`
	Address common.Address `json:"address"`
}

// InitializeViper sets up Viper configuration with environment variable bindings.
func InitializeViper() error {
	// Automatically bind environment variables
	viper.SetEnvPrefix("OCR") // All env vars should start with OCR_
	viper.AutomaticEnv()

	// Bind environment variables to specific configuration keys

	if err := viper.BindEnv("chain_id", "OCR_CHAIN_ID"); err != nil {
		return fmt.Errorf("error initializing viper: %w", err)
	}
	if err := viper.BindEnv("rpc_addr", "OCR_RPC_ADDR"); err != nil {
		return fmt.Errorf("error initializing viper: %w", err)
	}
	if err := viper.BindEnv("contract_address", "OCR_CONTRACT_ADDRESS"); err != nil {
		return fmt.Errorf("error initializing viper: %w", err)
	}

	return nil
}

// NewConfig creates a new configuration instance from a file path.
func NewConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml")

	var err error
	if err = viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err = viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadConfig loads configuration from various sources.
func (c *Config) LoadConfig() error {
	if c == nil {
		return errors.New("config is nil")
	}
	var err error

	if c.OutputFormat == "" {
		c.OutputFormat = TextOutputFormat
	}

	if _, err = url.ParseRequestURI(c.RPCAddr); err != nil {
		return errors.Wrapf(err, "invalid RPC address %s", c.RPCAddr)
	}

	client, err := ethclient.Dial(c.RPCAddr)
	if err != nil {
		log.Fatalf("Error connecting to Ethereum client: %v", err)
	}

	c.Network = client

	// query chain id from the client
	chainID, err := c.Network.ChainID(context.Background())
	if err == nil {
		// if conf.ChainId and chainId are different, print a warning
		if chainID.Cmp(big.NewInt(c.ChainID)) != 0 {
			log.Warningf(
				"chain id from config file (%d) and chain id from the client (%d) are different",
				c.ChainID,
				chainID.Uint64(),
			)
		}
	}

	if c.FlushEveryN == 0 {
		c.FlushEveryN = 10 // Default flushing
	}

	c.Stdout = bufio.NewWriter(os.Stdout)
	c.Stderr = bufio.NewWriter(os.Stderr)

	db, err := GetDatabase(c.Database)
	if err != nil {
		return err
	}

	c.Repository, err = newRepository(db)
	if err != nil {
		return err
	}

	return c.isValid()
}

var (
	// TextOutputFormat is the text output format identifier.
	TextOutputFormat = "text"
	// JSONOutputFormat is the JSON output format identifier.
	JSONOutputFormat = "json"
)

// Config represents the application configuration structure.
type Config struct {
	LogLevel string `mapstructure:"log_level"`

	OutputFormat string `mapstructure:"output_format"`

	ChainID int64  `mapstructure:"chain_id"`
	RPCAddr string `mapstructure:"rpc_addr"`

	FlushEveryN int `mapstructure:"flush_every"`

	Database Database `mapstructure:"database"`

	Repository *repository.Repository

	Stdout *bufio.Writer
	Stderr *bufio.Writer

	Network *ethclient.Client
}

// Decoder defines the interface for decoding data.
type Decoder interface {
	Decode(v interface{}) (err error)
}

// NewDecoder creates a new decoder based on the configured output format.
func (c *Config) NewDecoder(r io.Reader) Decoder {
	switch c.OutputFormat {
	case TextOutputFormat:
		return yaml.NewDecoder(r)
	case JSONOutputFormat:
		return json.NewDecoder(r)
	default:
		log.Fatalln(errors.Errorf("invalid output format: %s", c.OutputFormat))
	}
	return nil
}

// WithStdout sets the standard output writer.
func (c *Config) WithStdout(writer *bufio.Writer) *Config {
	cp := *c
	cp.Stdout = writer
	return &cp
}
// WithStderr sets the standard error writer.
func (c *Config) WithStderr(writer *bufio.Writer) *Config {
	cp := *c
	c.Stderr = writer
	return &cp
}

func (c *Config) isValid() error {
	if c.OutputFormat != TextOutputFormat && c.OutputFormat != JSONOutputFormat {
		return fmt.Errorf("invalid output format: %s", c.OutputFormat)
	}
	if c.ChainID == 0 {
		return errors.New("chain_id is required")
	}
	if c.RPCAddr == "" {
		return errors.New("rpc_addr is required")
	}
	if c.FlushEveryN < 1 {
		return errors.New("flush_every must be at least 1")
	}

	return nil
}

func (c *Config) Error(msg error) {
	toPrint := &TransmissionsResponse{
		Error: msg.Error(),
	}

	out, err := json.Marshal(toPrint)
	if err != nil {
		panic(err)
	}

	if _, err = c.printErr(out, c.Stderr); err != nil {
		panic(err)
	}
	os.Exit(1)
}

// Print formats and prints the response according to the configured output format.
func (c *Config) Print(toPrint *Response) ([]byte, error) {
	out, err := json.Marshal(toPrint)
	if err != nil {
		return nil, err
	}

	return c.printOutput(out, c.Stdout)
}

func (c *Config) printOutput(out []byte, writer *bufio.Writer) ([]byte, error) {
	if c.OutputFormat == TextOutputFormat {
		// handle text format by decoding and re-encoding JSON as YAML
		var j interface{}

		err := json.Unmarshal(out, &j)
		if err != nil {
			return nil, err
		}

		out, err = yaml.Marshal(j)
		if err != nil {
			return nil, err
		}
	}

	if writer == nil {
		writer = bufio.NewWriter(os.Stdout)
	}

	_, err := writer.Write(out)
	if err != nil {
		return nil, err
	}

	if c.OutputFormat != TextOutputFormat {
		// append new-line for formats besides YAML
		_, err = writer.Write([]byte("\n"))
		if err != nil {
			return nil, err
		}
	}
	
	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Config) printErr(out []byte, writer *bufio.Writer) ([]byte, error) {
	if c.OutputFormat == TextOutputFormat {
		// handle text format by decoding and re-encoding JSON as YAML
		var j interface{}

		err := json.Unmarshal(out, &j)
		if err != nil {
			return nil, err
		}

		out, err = yaml.Marshal(j)
		if err != nil {
			return nil, err
		}
	}
	if writer == nil {
		writer = bufio.NewWriter(os.Stderr)
	}

	_, err := writer.Write(out)
	if err != nil {
		return nil, err
	}

	if c.OutputFormat != TextOutputFormat {
		// append new-line for formats besides YAML
		_, err = writer.Write([]byte("\n"))
		if err != nil {
			return nil, err
		}
	}
	
	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return out, nil
}
