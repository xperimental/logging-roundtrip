package main

import (
	"net/http"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/config"
)

var log = &logrus.Logger{
	Out:          os.Stderr,
	Formatter:    &logrus.TextFormatter{},
	Hooks:        logrus.LevelHooks{},
	Level:        logrus.InfoLevel,
	ExitFunc:     os.Exit,
	ReportCaller: false,
}

func main() {
	cfg, err := config.Parse(os.Args[0], os.Args[1:])
	if err != nil {
		log.Fatalf("Can not load configuration: %v", err)
	}
	log.SetLevel(cfg.LogLevel)

	if err := http.ListenAndServe(cfg.Server.ListenAddress, nil); err != nil {
		log.Fatalf("Can not start server: %v", err)
	}
}
