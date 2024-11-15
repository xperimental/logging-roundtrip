package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type Config struct {
	ConfigFile string       `yaml:"-"`
	LogLevel   logrus.Level `yaml:"logLevel"`
	Server     ServerConfig `yaml:"server"`
	Source     SourceConfig `yaml:"source"`
	Sink       SinkConfig   `yaml:"sink"`
}

type ServerConfig struct {
	ListenAddress string `yaml:"listenAddress"`
}

type SourceConfig struct {
	LogsPerSecond float64 `yaml:"logsPerSecond"`
}

type SinkConfig struct {
	URL string     `yaml:"url"`
	TLS *TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	InsecureSkipVerify bool `yaml:"insecureSkipVerify"`
}

func Parse(cmd string, args []string) (*Config, error) {
	cfg := &Config{
		ConfigFile: "config.yaml",
		Server: ServerConfig{
			ListenAddress: ":8080",
		},
	}

	flags := pflag.NewFlagSet(cmd, pflag.ContinueOnError)
	flags.StringVarP(&cfg.ConfigFile, "config-file", "c", cfg.ConfigFile, "Path to configuration file")
	err := flags.Parse(args)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
