package sink

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

type Sink struct {
	cfg   config.SinkConfig
	log   logrus.FieldLogger
	store *storage.Storage
}

func New(cfg config.SinkConfig, log logrus.FieldLogger, store *storage.Storage) component.Component {
	return &Sink{
		cfg:   cfg,
		log:   log,
		store: store,
	}
}

func (s *Sink) Start(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()
	}()
}
