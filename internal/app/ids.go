package app

import "sync"

// maxAnnounced bounds how many announced score ids are remembered.
const maxAnnounced = 4096

// idSet remembers the most recent ids added, so a replayed log line is recognized while a
// lower id from a restored database is not mistaken for one.
type idSet struct {
	mu   sync.Mutex
	ring []int64
	next int
	seen map[int64]string // id to the fingerprint of the row announced
	top  int64
}

func (s *idSet) has(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.seen[id]
	return ok
}

func (s *idSet) fingerprint(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[id]
}

func (s *idSet) add(id int64, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seen[id]; ok {
		s.seen[id] = fingerprint
		return
	}
	if s.seen == nil {
		s.seen = map[int64]string{}
	}
	if len(s.ring) < maxAnnounced {
		s.ring = append(s.ring, id)
	} else {
		delete(s.seen, s.ring[s.next])
		s.ring[s.next] = id
		s.next = (s.next + 1) % maxAnnounced
	}
	s.seen[id] = fingerprint
	s.top = max(s.top, id)
}

// highest returns the largest id added since the last reset.
func (s *idSet) highest() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.top
}

func (s *idSet) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

func (s *idSet) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ring, s.next, s.seen, s.top = nil, 0, nil, 0
}
