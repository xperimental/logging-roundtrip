package sink

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/component"
	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/loki"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

var (
	errNoSinkURL = errors.New("sink URL can not be empty")
	errNoQuery   = errors.New("query can not be empty")

	idPattern = regexp.MustCompile(`id=(\d+)`)
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

		client, err := s.createClient(ctx)
		if err != nil {
			errCh <- err
			return
		}
		defer client.Close(websocket.StatusNormalClosure, "exiting")

		for {
			if ctx.Err() != nil {
				return
			}

			msgType, msgBytes, err := client.Read(ctx)
			switch {
			case errors.Is(err, net.ErrClosed):
				errCh <- fmt.Errorf("server closed connection: %w", err)
				return
			case err != nil:
				s.log.Errorf("Error getting websocket message: %s", err)
				continue
			}

			if msgType != websocket.MessageText {
				s.log.Warnf("Skipping non-text message: %s", msgBytes)
				continue
			}

			var msg loki.Message
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				s.log.Errorf("Error unmarshalling message: %s", err)
				continue
			}

			for _, stream := range msg.Streams {
				for _, entry := range stream.Values {
					unixNanos, err := strconv.ParseInt(entry[0], 10, 64)
					if err != nil {
						s.log.Errorf("Error parsing timestamp: %s", err)
						continue
					}

					msgTime := time.Unix(0, unixNanos)
					idMatch := idPattern.FindString(entry[1])
					msgId, err := strconv.ParseInt(idMatch[3:], 10, 64)
					if err != nil {
						s.log.Errorf("Error parsing message id: %s", err)
						continue
					}

					msgDelay := s.store.Seen(msgId, msgTime)
					s.log.Debugf("Message %v had %s delay.", msgId, msgDelay)
				}
			}
		}
	}()
}

func (s *Sink) createClient(ctx context.Context) (*websocket.Conn, error) {
	if s.cfg.URL == "" {
		return nil, errNoSinkURL
	}

	if s.cfg.Query == "" {
		return nil, errNoQuery
	}

	u, err := url.Parse(s.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL %q: %w", s.cfg.URL, err)
	}

	vals := url.Values{
		"query": []string{s.cfg.Query},
		"start": []string{strconv.FormatInt(s.store.Startup().UnixNano(), 10)},
	}
	u.RawQuery = vals.Encode()
	s.log.Debugf("Sink URL: %s", u.String())

	opts := &websocket.DialOptions{}
	if s.cfg.TLS != nil {
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: s.cfg.TLS.InsecureSkipVerify,
				},
			},
		}
	}

	if s.cfg.TokenFile != "" {
		tokenBytes, err := os.ReadFile(s.cfg.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("can not read token file: %w", err)
		}

		opts.HTTPHeader = http.Header{
			"Authorization": []string{
				fmt.Sprintf("Bearer %s", string(tokenBytes)),
			},
		}
	}

	conn, _, err := websocket.Dial(ctx, u.String(), opts)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
