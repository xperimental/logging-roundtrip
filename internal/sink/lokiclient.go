package sink

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/loki"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

var (
	errNoSinkURL  = errors.New("sink URL can not be empty")
	errNoQuery    = errors.New("query can not be empty")
	errHTTPStatus = errors.New("got non-ok HTTP status code")

	idPattern = regexp.MustCompile(`id=(\d+)`)
)

type command string

const (
	disconnectCommand command = "disconnect"

	defaultQueryInterval = 10 * time.Second
	defaultQueryLimit    = 1000

	lokiTailPath  = "loki/api/v1/tail"
	lokiQueryPath = "loki/api/v1/query_range"
)

type lokiClientSink struct {
	cfg   *config.LokiClientSink
	log   logrus.FieldLogger
	store *storage.Storage
	cmd   chan command
}

func newLokiClient(cfg *config.LokiClientSink, log logrus.FieldLogger, store *storage.Storage) (Sink, error) {
	if cfg.URL == "" {
		return nil, errNoSinkURL
	}

	if cfg.Query == "" {
		return nil, errNoQuery
	}

	return &lokiClientSink{
		cfg:   cfg,
		log:   log,
		store: store,
		cmd:   make(chan command, 1),
	}, nil
}

func (s *lokiClientSink) Disconnect() {
	s.cmd <- disconnectCommand
}

func (s *lokiClientSink) Start(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(s.cmd)

		for {
			if ctx.Err() != nil {
				return
			}

			err := s.receiveMessages(ctx)
			switch {
			case errors.Is(err, context.Canceled):
				return
			case errors.Is(err, net.ErrClosed), errors.Is(err, io.EOF):
				s.log.Debugf("Connection closed: %s", err)
				continue
			case err != nil:
				errCh <- err
				return
			}
		}
	}()
}

func (s *lokiClientSink) receiveMessages(ctx context.Context) error {
	client, err := s.createClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close(websocket.StatusNormalClosure, "exiting")

	if s.cfg.QueryInterval == 0 {
		s.cfg.QueryInterval = defaultQueryInterval
	}

	if s.cfg.QueryLimit == 0 {
		s.cfg.QueryLimit = defaultQueryLimit
	}

	s.log.Debugf("Query interval: %s", s.cfg.QueryInterval)
	queryTicker := time.NewTicker(s.cfg.QueryInterval)
	defer queryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.cmd:
			return net.ErrClosed
		case ts := <-queryTicker.C:
			go s.queryMessages(ctx, ts)
		default:
			msgType, msgBytes, err := client.Read(ctx)
			if err != nil {
				return err
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

			s.parseStreams(msg.Streams)
		}
	}
}

func (s *lokiClientSink) parseStreams(streams []loki.Stream) {
	for _, stream := range streams {
		for _, entry := range stream.Values {
			unixNanos, err := strconv.ParseInt(entry[0], 10, 64)
			if err != nil {
				s.log.Errorf("Error parsing timestamp: %s", err)
				continue
			}

			msgTime := time.Unix(0, unixNanos)
			idMatch := idPattern.FindString(entry[1])
			if idMatch == "" {
				continue
			}

			msgId, err := strconv.ParseUint(idMatch[3:], 10, 64)
			if err != nil {
				s.log.Errorf("Error parsing message id: %s", err)
				continue
			}

			s.store.Seen(msgId, msgTime)
		}
	}
}

func (s *lokiClientSink) createHTTPClient() *http.Client {
	c := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	if s.cfg.TLS != nil {
		c.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.cfg.TLS.InsecureSkipVerify,
			},
		}
	}

	return c
}

func (s *lokiClientSink) createClient(ctx context.Context) (*websocket.Conn, error) {
	tailURL, err := url.JoinPath(s.cfg.URL, lokiTailPath)
	if err != nil {
		return nil, fmt.Errorf("error creating tail URL: %w", err)
	}

	u, err := url.Parse(tailURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL %q: %w", s.cfg.URL, err)
	}

	vals := url.Values{
		"query": []string{s.cfg.Query},
		"start": []string{strconv.FormatInt(s.store.Startup().UnixNano(), 10)},
	}
	u.RawQuery = vals.Encode()
	s.log.Debugf("Tail URL: %s", u.String())

	opts := &websocket.DialOptions{}
	opts.HTTPClient = s.createHTTPClient()

	if s.cfg.TokenFile != "" {
		tokenBytes, err := os.ReadFile(s.cfg.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("can not read token file: %w", err)
		}
		token := strings.TrimSpace(string(tokenBytes))

		opts.HTTPHeader = http.Header{
			"Authorization": []string{
				fmt.Sprintf("Bearer %s", token),
			},
		}
		s.log.Debugf("Using bearer token: %s", token[:10])
	}

	conn, _, err := websocket.Dial(ctx, u.String(), opts)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(-1)

	return conn, nil
}

func (s *lokiClientSink) queryMessages(ctx context.Context, ts time.Time) {
	if err := s.queryMessagesInner(ctx, ts); err != nil {
		s.log.Errorf("Error querying messages: %s", err)
	}
}

func (s *lokiClientSink) queryMessagesInner(ctx context.Context, ts time.Time) error {
	queryURL, err := url.JoinPath(s.cfg.URL, lokiQueryPath)
	if err != nil {
		return fmt.Errorf("error creating query URL: %w", err)
	}

	u, err := url.Parse(queryURL)
	if err != nil {
		return fmt.Errorf("error parsing query URL %q: %w", s.cfg.URL, err)
	}

	vals := url.Values{
		"query":     []string{s.cfg.Query},
		"start":     []string{strconv.FormatInt(s.store.Startup().UnixNano(), 10)},
		"end":       []string{strconv.FormatInt(ts.UnixNano(), 10)},
		"direction": []string{"forward"},
		"limit":     []string{strconv.FormatUint(s.cfg.QueryLimit, 10)},
	}
	u.RawQuery = vals.Encode()
	s.log.Debugf("Query URL: %s", u.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating query request: %w", err)
	}

	if s.cfg.TokenFile != "" {
		tokenBytes, err := os.ReadFile(s.cfg.TokenFile)
		if err != nil {
			return fmt.Errorf("can not read token file: %w", err)
		}
		token := strings.TrimSpace(string(tokenBytes))

		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		s.log.Debugf("Using bearer token: %s", token[:10])
	}

	client := s.createHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error executing query request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("query request failed with status %d: %w", res.StatusCode, errHTTPStatus)
	}

	var response loki.Response
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("error parsing query response: %w", err)
	}

	s.parseStreams(response.Data.Result)
	return nil
}
