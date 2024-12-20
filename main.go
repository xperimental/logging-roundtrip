package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/sink"
	"github.com/xperimental/logging-roundtrip/internal/source"
	"github.com/xperimental/logging-roundtrip/internal/storage"
	"github.com/xperimental/logging-roundtrip/internal/web"
)

var (
	GitCommit = "unknown"

	log = &logrus.Logger{
		Out:          os.Stderr,
		Formatter:    &logrus.TextFormatter{},
		Hooks:        logrus.LevelHooks{},
		Level:        logrus.InfoLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
)

func main() {
	cfg, err := config.Parse(os.Args[0], os.Args[1:])
	if err != nil {
		log.Fatalf("Can not load configuration: %v", err)
	}
	log.SetLevel(cfg.LogLevel)
	log.Infof("logging-roundtrip (commit: %s)", GitCommit)

	registry := prometheus.NewRegistry()
	store := storage.New(log, time.Now, registry)

	s, err := sink.New(cfg.Sink, log, store)
	if err != nil {
		log.Fatalf("Can not initialize sink: %s", err)
	}

	components := []component.Component{
		store,
		s,
		source.New(cfg.Source, log, store),
		web.NewServer(cfg.Server, log, store, registry, s),
	}

	wg := &sync.WaitGroup{}
	errCh := make(chan error, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		for err := range errCh {
			log.Errorf("Fatal error: %v", err)
			cancel()
		}
	}()

	for _, c := range components {
		c.Start(ctx, wg, errCh)
	}

	log.Debug("All components running.")
	wg.Wait()
	close(errCh)
	log.Debug("All components stopped.")
}
