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

	messageCount uint64
	messageID    uint64
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

		s.messageID = s.cfg.StartID

		interval := time.Duration(float64(time.Second) / s.cfg.LogsPerSecond)
		s.log.Debugf("Source interval: %v", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		s.log.Debug("Started source.")
	loop:
		for {
			select {
			case <-ctx.Done():
				return
			case ts := <-ticker.C:
				msg := s.storage.Create(s.nextID(), ts)

				if _, err := s.output.Write([]byte(msg)); err != nil {
					errCh <- fmt.Errorf("error writing to output: %w", err)
				}

				s.messageCount++
				if s.cfg.NumberOfMessages > 0 && s.messageCount >= s.cfg.NumberOfMessages {
					s.log.Debugf("Reached number of messages limit: %v", s.cfg.NumberOfMessages)

					s.storage.SetAllSent()
					break loop
				}
			}
		}
	}()
}

func (s *source) nextID() uint64 {
	s.messageID++
	return s.messageID
}
