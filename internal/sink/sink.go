package sink

import (
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

var (
	ErrUnknownSinkType = errors.New("unknown sink type")
	ErrEmptySinkConfig = errors.New("empty sink config")
)

type Sink interface {
	component.Component
	Disconnect()
}

func New(cfg config.SinkConfig, log logrus.FieldLogger, store *storage.Storage) (Sink, error) {
	switch cfg.Type {
	case config.SinkTypeLokiClient:
		if cfg.LokiClient == nil {
			return nil, ErrEmptySinkConfig
		}

		return newLokiClient(cfg.LokiClient, log, store), nil
	default:
		return nil, ErrUnknownSinkType
	}
}
