package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

var errConfigFileRequired = errors.New("config file is required")

type Config struct {
	ConfigFile string       `yaml:"-"`
	LogLevel   logrus.Level `yaml:"logLevel"`
	Server     ServerConfig `yaml:"server"`
	Source     SourceConfig `yaml:"source"`
	Sink       SinkConfig   `yaml:"sink"`
}

type ServerConfig struct {
	ListenAddress string     `yaml:"listenAddress"`
	TLS           *TLSConfig `yaml:"tls"`
}

type SourceConfig struct {
	StartID          uint64  `yaml:"startID"`
	LogsPerSecond    float64 `yaml:"logsPerSecond"`
	NumberOfMessages uint64  `yaml:"numberOfMessages"`
}

type SinkType string

const (
	SinkTypeLokiClient SinkType = "lokiClient"
)

type SinkConfig struct {
	Type       SinkType        `yaml:"type"`
	LokiClient *LokiClientSink `yaml:"lokiClient"`
}

type LokiClientSink struct {
	URL           string        `yaml:"url"`
	TLS           *TLSConfig    `yaml:"tls"`
	TokenFile     string        `yaml:"tokenFile"`
	Query         string        `yaml:"query"`
	QueryInterval time.Duration `yaml:"queryInterval"`
	QueryLimit    uint64        `yaml:"queryLimit"`
	DisableTail   bool          `yaml:"disableTail"`
}

type TLSConfig struct {
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	CertificateFile    string `yaml:"certificateFile"`
	KeyFile            string `yaml:"keyFile"`
}

func Parse(cmd string, args []string) (*Config, error) {
	cfg := &Config{
		ConfigFile: "config.yaml",
		LogLevel:   logrus.InfoLevel,
		Server: ServerConfig{
			ListenAddress: ":8080",
		},
		Source: SourceConfig{
			LogsPerSecond: 1,
		},
	}

	flags := pflag.NewFlagSet(cmd, pflag.ContinueOnError)
	flags.StringVarP(&cfg.ConfigFile, "config-file", "c", cfg.ConfigFile, "Path to configuration file")
	err := flags.Parse(args)
	if err != nil {
		return nil, err
	}

	if cfg.ConfigFile == "" {
		return nil, errConfigFileRequired
	}

	file, err := os.Open(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("can not read config file: %w", err)
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("can not parse config file: %w", err)
	}

	return cfg, nil
}
