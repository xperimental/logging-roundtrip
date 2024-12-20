package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type Storage struct {
	log         logrus.FieldLogger
	clock       func() time.Time
	startupTime time.Time

	messages    sync.Map
	metricCount prometheus.Counter
	metricDelay prometheus.Histogram
}

type message struct {
	ID            uint64
	Timestamp     time.Time
	Seen          bool
	SeenTimestamp time.Time
}

func (m message) String() string {
	return fmt.Sprintf("time=%s id=%d\n", m.Timestamp.UTC().Format(time.RFC3339Nano), m.ID)
}

func New(log logrus.FieldLogger, clock func() time.Time, registry prometheus.Registerer) *Storage {
	s := &Storage{
		log:         log,
		clock:       clock,
		startupTime: clock(),
		messages:    sync.Map{},
		metricCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "roundtrip_storage_messages_produced_total",
			Help: "Total number of messages produced by storage",
		}),
		metricDelay: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "roundtrip_storage_message_delay_seconds",
			Help: "Measured delay of produced messages.",
		}),
	}

	registry.MustRegister(
		s.metricCount,
		s.metricDelay,
	)
	return s
}

func (s *Storage) Start(ctx context.Context, wg *sync.WaitGroup, _ chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.printStatisticsInner()
			}
		}
	}()
}

func (s *Storage) Startup() time.Time {
	return s.startupTime
}

func (s *Storage) Create(id uint64, time time.Time) string {
	msg := message{
		ID:        id,
		Timestamp: time,
	}

	s.messages.Store(msg.ID, msg)
	s.metricCount.Inc()
	return msg.String()
}

func (s *Storage) Count() int {
	count := 0
	s.messages.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	return count
}

func (s *Storage) Seen(id uint64, t time.Time) {
	v, ok := s.messages.Load(id)
	if !ok {
		s.log.Warnf("Found unknown message with ID %v", id)
		return
	}

	msg := v.(message)
	if msg.Seen {
		// silently ignore duplicates
		return
	}

	msg.Seen = true
	msg.SeenTimestamp = t

	s.messages.Store(id, msg)

	delay := t.Sub(msg.Timestamp)
	s.metricDelay.Observe(delay.Seconds())
	s.log.Debugf("Message %v had delay %s", id, delay)
}

func (s *Storage) OldestUnseenTime() (time.Time, bool) {
	oldest := s.clock()
	haveUnseen := false

	s.messages.Range(func(_, v interface{}) bool {
		msg := v.(message)
		if msg.Seen {
			return true
		}

		haveUnseen = true
		if msg.Timestamp.Before(oldest) {
			oldest = msg.Timestamp
		}
		return true
	})

	return oldest, haveUnseen
}

func (s *Storage) printStatisticsInner() {
	created := 0
	seen := 0

	s.messages.Range(func(_, v interface{}) bool {
		created++

		msg := v.(message)
		if msg.Seen {
			seen++
		}

		return true
	})

	percent := float64(seen) / float64(created) * 100
	s.log.Infof("Message state: %d messages sent, %d seen (%.3f %%)", created, seen, percent)
}
