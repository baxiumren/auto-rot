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
	StepMonitorForceAdd  Step = "monitor_force_add"

	// CF Redirect steps
	StepCFAddLabel    Step = "cf_add_label"
	StepCFAddDomain   Step = "cf_add_domain"   // baru: input nama domain (auto-discover)
	StepCFAddPickType Step = "cf_add_picktype" // kalau ada v1+v2 dua-duanya
	StepCFAddPickRule Step = "cf_add_pickrule" // kalau ada >1 rule
	StepCFChangeURL Step = "cf_change_url"
	StepCFBulkPick  Step = "cf_bulk_pick" // sedang pilih-pilih rule (checkbox)
	StepCFBulkURL   Step = "cf_bulk_url"  // sudah pilih, lagi ketik URL

	// New Domain Registration flow (kalau domain belum di CF)
	StepCFNewConfirm   Step = "cf_new_confirm"    // confirm register?
	StepCFNewPickType  Step = "cf_new_pick_type"  // V1 atau V2 redirect?
	StepCFNewTargetURL Step = "cf_new_target_url" // URL tujuan redirect

	// Auto Rotator steps
	StepRotatorAddLabel Step = "rotator_add_label"
	StepRotatorBulkPick Step = "rotator_bulk_pick" // pilih banyak CF rule (checkbox)
	StepRotatorBulkPool Step = "rotator_bulk_pool" // pilih pool untuk semua

	// Settings steps
	StepSettingsEmail     Step = "settings_email"
	StepSettingsKey       Step = "settings_key"
	StepSettingsBothEmail Step = "settings_both_email"
	StepSettingsBothKey   Step = "settings_both_key"

	// Global search
	StepGlobalSearch Step = "global_search"
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
