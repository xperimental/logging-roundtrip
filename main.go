package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/sink"
	"github.com/xperimental/logging-roundtrip/internal/source"
	"github.com/xperimental/logging-roundtrip/internal/storage"
	"github.com/xperimental/logging-roundtrip/internal/web"
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

	registry := prometheus.NewRegistry()
	store := storage.New(registry)

	components := []component.Component{
		source.New(cfg.Source, log, store),
		sink.New(cfg.Sink, log, store),
		web.NewServer(cfg.Server, log, store, registry),
	}

	wg := &sync.WaitGroup{}
	errCh := make(chan error, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case err := <-errCh:
				log.Errorf("Fatal error: %v", err)
				cancel()
			}
		}

		close(errCh)
	}()

	for _, c := range components {
		c.Start(ctx, wg, errCh)
	}

	log.Debug("All components running.")
	wg.Wait()
	log.Debug("All components stopped.")
}
