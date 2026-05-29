package store

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// GroupCommand maps Telegram slash command → klikcepat project_id.
// Saat user ketik /rtp di group, bot fetch link di project_id terkait.
type GroupCommand struct {
	Command     string    `json:"command"`      // tanpa "/" — contoh: "rtp", "daftar"
	ProjectID   int       `json:"project_id"`   // klikcepat project ID
	ProjectName string    `json:"project_name"` // cache nama project (display)
	Description string    `json:"description"`  // pendek, dipakai di Telegram autocomplete (max 256 char)
	CreatedAt   time.Time `json:"created_at"`
}

type GroupCommandStore struct {
	mu   sync.Mutex
	data []GroupCommand
}

func NewGroupCommandStore() *GroupCommandStore {
	s := &GroupCommandStore{}
	s.load()
	return s
}

func (s *GroupCommandStore) load() {
	b, err := os.ReadFile(dataDir + "/group_commands.json")
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &s.data)
}

func (s *GroupCommandStore) save() {
	b, _ := json.MarshalIndent(s.data, "", "  ")
	_ = os.WriteFile(dataDir+"/group_commands.json", b, 0644)
}

func (s *GroupCommandStore) GetAll() []GroupCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GroupCommand, len(s.data))
	copy(out, s.data)
	return out
}

// GetByCommand returns command match by case-insensitive name (without "/" prefix).
func (s *GroupCommandStore) GetByCommand(cmd string) (*GroupCommand, bool) {
	cmd = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cmd), "/"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data {
		if strings.EqualFold(s.data[i].Command, cmd) {
			gc := s.data[i]
			return &gc, true
		}
	}
	return nil, false
}

// Add stores a new command. Fails kalau command name udah ada (case-insensitive).
func (s *GroupCommandStore) Add(gc GroupCommand) error {
	gc.Command = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(gc.Command), "/"))
	if gc.Command == "" {
		return fmt.Errorf("command name kosong")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.data {
		if strings.EqualFold(existing.Command, gc.Command) {
			return fmt.Errorf("command /%s udah ada (project: %s)", gc.Command, existing.ProjectName)
		}
	}
	if gc.CreatedAt.IsZero() {
		gc.CreatedAt = time.Now()
	}
	s.data = append(s.data, gc)
	go s.save()
	return nil
}

func (s *GroupCommandStore) Delete(cmd string) bool {
	cmd = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cmd), "/"))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, gc := range s.data {
		if strings.EqualFold(gc.Command, cmd) {
			s.data = append(s.data[:i], s.data[i+1:]...)
			go s.save()
			return true
		}
	}
	return false
}
