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

type Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type TransmissionsResponse struct {
	Result []Result `json:"result,omitempty"`
	Error  string   `json:"error,omitempty"`
}

type Result struct {
	RoundId      string           `json:"roundId"`
	Timestamp    time.Time        `json:"timestamp"`
	Observers    []ResultObserver `json:"observers"`
	Transmitters []ResultObserver `json:"transmitters"`
}

type ResultObserver struct {
	Idx     int            `json:"idx"`
	Address common.Address `json:"address"`
}

func InitializeViper() error {
	// Automatically bind environment variables
	viper.SetEnvPrefix("OCR") // All env vars should start with OCR_
	viper.AutomaticEnv()

	// Bind environment variables to specific configuration keys

	var err error

	err = viper.BindEnv("chain_id", "OCR_CHAIN_ID")
	err = viper.BindEnv("rpc_addr", "OCR_RPC_ADDR")
	err = viper.BindEnv("contract_address", "OCR_CONTRACT_ADDRESS")

	if err != nil {
		return errors.New(fmt.Sprintf("error initializing viper: %v", err))
	}

	return nil
}

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

func (c *Config) LoadConfig() error {
	if c == nil {
		return errors.New("config is nil")
	}
	var err error

	if c.OutputFormat == "" {
		c.OutputFormat = TextOutputFormat
	}

	if _, err = url.ParseRequestURI(c.RpcAddr); err != nil {
		return errors.Wrapf(err, "invalid RPC address %s", c.RpcAddr)
	}

	client, err := ethclient.Dial(c.RpcAddr)
	if err != nil {
		log.Fatalf("Error connecting to Ethereum client: %v", err)
	}

	c.Network = client

	// query chain id from the client
	chainID, err := c.Network.ChainID(context.Background())
	if err == nil {
		// if conf.ChainId and chainId are different, print a warning
		if chainID.Cmp(big.NewInt(c.ChainId)) != 0 {
			log.Warningf("chain id from config file (%d) and chain id from the client (%d) are different", c.ChainId, chainID.Uint64())
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
	TextOutputFormat = "text"
	JSONOutputFormat = "json"
)

type Config struct {
	LogLevel string `mapstructure:"log_level"`

	OutputFormat string `mapstructure:"output_format"`

	ChainId int64  `mapstructure:"chain_id"`
	RpcAddr string `mapstructure:"rpc_addr"`

	FlushEveryN int `mapstructure:"flush_every"`

	Database Database `mapstructure:"database"`

	Repository *repository.Repository

	Stdout *bufio.Writer
	Stderr *bufio.Writer

	Network *ethclient.Client
}

type Decoder interface {
	Decode(v interface{}) (err error)
}

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

func (c *Config) WithStdout(writer *bufio.Writer) *Config {
	cp := *c
	cp.Stdout = writer
	return &cp
}
func (c *Config) WithStderr(writer *bufio.Writer) *Config {
	cp := *c
	c.Stderr = writer
	return &cp
}

func (c *Config) isValid() error {
	if c.OutputFormat != TextOutputFormat && c.OutputFormat != JSONOutputFormat {
		return fmt.Errorf("invalid output format: %s", c.OutputFormat)
	}
	if c.ChainId == 0 {
		return errors.New("chain_id is required")
	}
	if c.RpcAddr == "" {
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

	if out, err = c.printErr(out, c.Stderr); err != nil {
		panic(err)
	}
	os.Exit(1)
}

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
	defer writer.Flush()

	if c.OutputFormat != TextOutputFormat {
		// append new-line for formats besides YAML
		_, err = writer.Write([]byte("\n"))
		if err != nil {
			return nil, err
		}
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
	defer writer.Flush()

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

	return out, nil
}
