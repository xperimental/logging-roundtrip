package storage

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type StoreOp func(messages map[int64]message) error

type Storage struct {
	log         logrus.FieldLogger
	clock       func() time.Time
	startupTime time.Time

	ops         chan StoreOp
	messages    map[int64]message
	nextID      atomic.Int64
	metricCount prometheus.Counter
	metricDelay prometheus.Histogram
}

type message struct {
	ID            int64
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
		ops:         make(chan StoreOp, 1),
		messages:    map[int64]message{},
		nextID:      atomic.Int64{},
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
		defer close(s.ops)

		for {
			select {
			case <-ctx.Done():
				return
			case op := <-s.ops:
				if err := op(s.messages); err != nil {
					s.log.Debugf("Error during store operation: %s", err)
				}
			}
		}
	}()
}

func (s *Storage) Startup() time.Time {
	return s.startupTime
}

func (s *Storage) Create() string {
	msg := message{
		ID:        s.nextID.Add(1),
		Timestamp: s.clock(),
	}

	s.ops <- func(messages map[int64]message) error {
		messages[msg.ID] = msg
		return nil
	}

	s.metricCount.Inc()
	return msg.String()
}

func (s *Storage) Count() int {
	resCh := make(chan int, 1)
	s.ops <- func(messages map[int64]message) error {
		resCh <- len(messages)
		return nil
	}

	return <-resCh
}

func (s *Storage) Seen(id int64, t time.Time) {
	s.ops <- func(messages map[int64]message) error {
		msg := s.messages[id]
		msg.Seen = true
		msg.SeenTimestamp = t
		s.messages[id] = msg

		delay := t.Sub(msg.Timestamp)
		s.metricDelay.Observe(delay.Seconds())
		s.log.Debugf("Message id=%v had delay %s", id, delay)

		return nil
	}
}

func (s *Storage) ResetSeen() {
	s.ops <- func(messages map[int64]message) error {
		for id := range s.messages {
			msg := s.messages[id]
			msg.Seen = false
			s.messages[id] = msg
		}

		return nil
	}
}
