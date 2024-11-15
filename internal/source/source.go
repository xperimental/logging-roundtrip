package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

var errInvalidLogsPerSecond = errors.New("logsPerSecond needs to be a positive number")

type source struct {
	cfg     config.SourceConfig
	log     logrus.FieldLogger
	storage *storage.Storage
	output  io.Writer
}

func New(cfg config.SourceConfig, log *logrus.Logger, storage *storage.Storage) component.Component {
	return &source{
		cfg:     cfg,
		log:     log,
		storage: storage,
		output:  os.Stdout,
	}
}

func (s *source) Start(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		if s.cfg.LogsPerSecond <= 0 {
			errCh <- errInvalidLogsPerSecond
			return
		}

		interval := time.Duration(float64(time.Second) / s.cfg.LogsPerSecond)
		s.log.Debugf("Source interval: %v", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		s.log.Debug("Started source.")
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				id := s.storage.Insert(t)
				msg := fmt.Sprintf("time=%s id=%d\n", t.UTC().Format(time.RFC3339Nano), id)

				if _, err := s.output.Write([]byte(msg)); err != nil {
					errCh <- fmt.Errorf("error writing to output: %w", err)
				}
			}
		}
	}()
}
