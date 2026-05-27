package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// KlikcepatRotator maps a klikcepat link to a Monitor pool label
// for auto-swap when target domain gets blocked.
type KlikcepatRotator struct {
	ID        string    `json:"id"`         // slug from Label
	Label     string    `json:"label"`      // user-friendly name
	LinkID    int       `json:"link_id"`    // klikcepat link ID
	LinkURL   string    `json:"link_url"`   // displayed slug for UI
	LinkType  string    `json:"link_type"`  // "link" or "biolink"
	PoolLabel string    `json:"pool_label"` // Monitor label for backup pool
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type KlikcepatRotatorStore struct {
	mu   sync.Mutex
	data []KlikcepatRotator
}

func NewKlikcepatRotatorStore() *KlikcepatRotatorStore {
	s := &KlikcepatRotatorStore{}
	s.load()
	return s
}

func (s *KlikcepatRotatorStore) load() {
	b, err := os.ReadFile(dataDir + "/klikcepat_rotators.json")
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &s.data)
}

func (s *KlikcepatRotatorStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	_ = os.WriteFile(dataDir+"/klikcepat_rotators.json", b, 0644)
}

func (s *KlikcepatRotatorStore) GetAll() []KlikcepatRotator {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]KlikcepatRotator, len(s.data))
	copy(out, s.data)
	return out
}

func (s *KlikcepatRotatorStore) GetByID(id string) (*KlikcepatRotator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].ID == id {
			r := s.data[i]
			return &r, true
		}
	}
	return nil, false
}

func (s *KlikcepatRotatorStore) GetByLinkID(linkID int) (*KlikcepatRotator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].LinkID == linkID {
			r := s.data[i]
			return &r, true
		}
	}
	return nil, false
}

// Add creates a new rotator. Returns error if Label slug already exists.
func (s *KlikcepatRotatorStore) Add(r KlikcepatRotator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = slugify(r.Label)
	for _, existing := range s.data {
		if existing.ID == r.ID {
			return fmt.Errorf("rotator dengan label %q udah ada", r.Label)
		}
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.Active = true
	s.data = append(s.data, r)
	go s.save()
	return nil
}

func (s *KlikcepatRotatorStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.data {
		if r.ID == id {
			s.data = append(s.data[:i], s.data[i+1:]...)
			go s.save()
			return true
		}
	}
	return false
}

// Toggle flips Active. Returns new state, and whether found.
func (s *KlikcepatRotatorStore) Toggle(id string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].ID == id {
			s.data[i].Active = !s.data[i].Active
			go s.save()
			return s.data[i].Active, true
		}
	}
	return false, false
}
