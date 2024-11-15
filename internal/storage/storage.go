package storage

import (
	"sync"
	"time"
)

type Storage struct {
	sync.RWMutex
	messages map[int64]message
	nextID   int64
}

type message struct {
	ID            int64
	Timestamp     time.Time
	Seen          bool
	SeenTimestamp time.Time
}

func New() *Storage {
	return &Storage{
		messages: map[int64]message{},
		nextID:   0,
	}
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
