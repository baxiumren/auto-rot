package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// KlikcepatBlockRotator maps a klikcepat biolink BLOCK (button) to a Monitor pool label
// for auto-swap when block destination domain gets blocked.
//
// Beda dari KlikcepatRotator (target shortlink location_url):
//   - Block rotator target field location_url DI BLOCK INI (bagian dari biolink)
//   - Update via /api/biolink-blocks/{id} (custom endpoint)
type KlikcepatBlockRotator struct {
	ID            string    `json:"id"`              // slug dari Label
	Label         string    `json:"label"`           // user-friendly name
	BlockID       int       `json:"block_id"`        // biolinks_blocks.biolink_block_id
	BiolinkID     int       `json:"biolink_id"`      // parent biolink link_id (untuk display)
	BiolinkSlug   string    `json:"biolink_slug"`    // parent biolink slug (display)
	BiolinkDomain int       `json:"biolink_domain"`  // parent biolink domain_id (display)
	BlockName     string    `json:"block_name"`      // settings.name dari block (LOGIN, DAFTAR, dll)
	PoolLabel     string    `json:"pool_label"`      // Monitor label for backup pool
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type KlikcepatBlockRotatorStore struct {
	mu   sync.Mutex
	data []KlikcepatBlockRotator
}

func NewKlikcepatBlockRotatorStore() *KlikcepatBlockRotatorStore {
	s := &KlikcepatBlockRotatorStore{}
	s.load()
	return s
}

func (s *KlikcepatBlockRotatorStore) load() {
	b, err := os.ReadFile(dataDir + "/klikcepat_block_rotators.json")
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &s.data)
}

func (s *KlikcepatBlockRotatorStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	_ = os.WriteFile(dataDir+"/klikcepat_block_rotators.json", b, 0644)
}

func (s *KlikcepatBlockRotatorStore) GetAll() []KlikcepatBlockRotator {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]KlikcepatBlockRotator, len(s.data))
	copy(out, s.data)
	return out
}

func (s *KlikcepatBlockRotatorStore) GetByID(id string) (*KlikcepatBlockRotator, bool) {
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

func (s *KlikcepatBlockRotatorStore) GetByBlockID(blockID int) (*KlikcepatBlockRotator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if s.data[i].BlockID == blockID {
			r := s.data[i]
			return &r, true
		}
	}
	return nil, false
}

func (s *KlikcepatBlockRotatorStore) Add(r KlikcepatBlockRotator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = slugify(r.Label)
	for _, existing := range s.data {
		if existing.ID == r.ID {
			return fmt.Errorf("block rotator dengan label %q udah ada", r.Label)
		}
		if existing.BlockID == r.BlockID {
			return fmt.Errorf("block ID %d udah punya rotator (label: %s)", r.BlockID, existing.Label)
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

func (s *KlikcepatBlockRotatorStore) Delete(id string) bool {
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

func (s *KlikcepatBlockRotatorStore) Toggle(id string) (bool, bool) {
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
