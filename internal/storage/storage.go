package storage

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Storage struct {
	sync.RWMutex
	messages    map[int64]message
	nextID      int64
	metricCount prometheus.Counter
}

type message struct {
	ID            int64
	Timestamp     time.Time
	Seen          bool
	SeenTimestamp time.Time
}

func New(registry prometheus.Registerer) *Storage {
	s := &Storage{
		messages: map[int64]message{},
		nextID:   0,
		metricCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "roundtrip_storage_messages_produced_total",
			Help: "Total number of messages produced by storage",
		}),
	}

	registry.MustRegister(s.metricCount)
	return s
}

func (s *Storage) Insert(t time.Time) int64 {
	s.Lock()
	defer s.Unlock()

	id := s.nextID
	s.nextID++

	s.messages[id] = message{
		ID:        id,
		Timestamp: t,
	}

	s.metricCount.Inc()
	return id
}

func (s *Storage) Count() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.messages)
}

func (s *Storage) Seen(id int64, t time.Time) {
	s.Lock()
	defer s.Unlock()

	msg := s.messages[id]
	msg.Seen = true
	msg.SeenTimestamp = t
	s.messages[id] = msg
}

func (s *Storage) ResetSeen() {
	s.Lock()
	defer s.Unlock()

	for id := range s.messages {
		msg := s.messages[id]
		msg.Seen = false
		s.messages[id] = msg
	}
}
