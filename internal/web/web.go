package web

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/xperimental/logging-roundtrip/internal/config"
	"github.com/xperimental/logging-roundtrip/internal/sink"
	"github.com/xperimental/logging-roundtrip/internal/storage"
)

var errNeedCertificateAndKey = errors.New("need both certificate and key to start TLS server")

type Server struct {
	cfg           config.ServerConfig
	log           logrus.FieldLogger
	listenAddress string
	server        *http.Server
}

func NewServer(cfg config.ServerConfig, log logrus.FieldLogger, store *storage.Storage, registry prometheus.Gatherer, sink sink.Sink) *Server {
	m := mux.NewRouter()
	m.Path("/api/complete").Methods(http.MethodGet).HandlerFunc(completeHandler(store))
	m.Path("/api/live").Methods(http.MethodGet).Handler(livenessHandler())
	m.Path("/api/store/count").Methods(http.MethodGet).Handler(countHandler(store))
	m.Path("/debug/disconnect").Methods(http.MethodGet).Handler(disconnectHandler(sink))
	m.Path("/metrics").Methods(http.MethodGet).Handler(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	s := &Server{
		cfg:           cfg,
		log:           log.WithField("component", "server"),
		listenAddress: cfg.ListenAddress,
		server: &http.Server{
			Addr:    cfg.ListenAddress,
			Handler: m,
		},
	}

	return s
}

func (s *Server) Start(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	wg.Add(1)
	go func() {
		<-ctx.Done()
		s.log.Debug("Shutting down server...")
		if err := s.server.Shutdown(context.Background()); err != nil {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()

		if s.cfg.TLS != nil {
			tls := s.cfg.TLS
			if tls.CertificateFile == "" || tls.KeyFile == "" {
				errCh <- errNeedCertificateAndKey
				return
			}

			s.log.Infof("Starting TLS server on %s", s.listenAddress)
			if err := s.server.ListenAndServeTLS(tls.CertificateFile, tls.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}

		s.log.Infof("Starting server on %s", s.listenAddress)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
}
