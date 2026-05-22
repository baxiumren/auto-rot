package bot

import (
	"sync"

	tele "gopkg.in/telebot.v3"
)

type Step string

const (
	// Monitor steps
	StepMonitorAddDomain Step = "monitor_add_domain"
	StepMonitorAddLabel  Step = "monitor_add_label"
	StepMonitorRemove    Step = "monitor_remove"
	StepMonitorCheck     Step = "monitor_check"
	StepMonitorInterval  Step = "monitor_interval"

	// CF Redirect steps
	StepCFAddLabel     Step = "cf_add_label"
	StepCFAddZone      Step = "cf_add_zone"
	StepCFAddType      Step = "cf_add_type"
	StepCFAddRuleset   Step = "cf_add_ruleset"
	StepCFAddRuleID    Step = "cf_add_ruleid"
	StepCFChangeURL    Step = "cf_change_url"

	// Auto Rotator steps
	StepRotatorAddLabel Step = "rotator_add_label"
)

type Session struct {
	Step      Step
	Data      map[string]string
	PromptMsg *tele.Message
}

type sessionStore struct {
	mu   sync.Mutex
	data map[int64]*Session
}

func newSessionStore() *sessionStore {
	return &sessionStore{data: make(map[int64]*Session)}
}

func (s *sessionStore) Set(userID int64, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[userID] = sess
}

func (s *sessionStore) Get(userID int64) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data[userID]
	return sess, ok
}

func (s *sessionStore) Delete(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, userID)
}
