package bot

import (
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// SessionTTL: session auto-expire setelah ini (cegah stale state nyangkut)
const SessionTTL = 5 * time.Minute

type Step string

const (
	// Monitor steps
	StepMonitorAddDomain Step = "monitor_add_domain"
	StepMonitorAddLabel  Step = "monitor_add_label"
	StepMonitorRemove    Step = "monitor_remove"
	StepMonitorCheck     Step = "monitor_check"        // user lagi ketik domain
	StepMonitorCheckSrc  Step = "monitor_check_source" // user lagi pilih source (Kominfo/Nawala)
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

	// Klikcepat Settings
	StepSettingsKlikcepatURL    Step = "settings_klikcepat_url"
	StepSettingsKlikcepatKey    Step = "settings_klikcepat_key"
	StepSettingsKlikcepatDomain   Step = "settings_klikcepat_domain"
	StepSettingsKlikcepatDomMapID Step = "settings_klikcepat_dommap_id"   // input domain ID
	StepSettingsKlikcepatDomMapHost Step = "settings_klikcepat_dommap_host" // input host

	// Klikcepat Link CRUD
	StepKlikcepatAddType        Step = "klikcepat_add_type"
	StepKlikcepatAddTitle       Step = "klikcepat_add_title"
	StepKlikcepatAddSlug        Step = "klikcepat_add_slug"
	StepKlikcepatAddLocationURL Step = "klikcepat_add_location"
	StepKlikcepatAddProject     Step = "klikcepat_add_project"

	StepKlikcepatEditPickField Step = "klikcepat_edit_pickfield"
	StepKlikcepatEditValue     Step = "klikcepat_edit_value"

	// Klikcepat Project CRUD
	StepKlikcepatProjectAddName  Step = "klikcepat_project_add_name"
	StepKlikcepatProjectAddColor Step = "klikcepat_project_add_color"
	StepKlikcepatProjectEditName Step = "klikcepat_project_edit_name"

	// Klikcepat Rotator wizard (entry from Auto Rotator)
	StepKlikcepatRotatorPickLink Step = "klikcepat_rot_picklink"
	StepKlikcepatRotatorPickPool Step = "klikcepat_rot_pickpool"
	StepKlikcepatRotatorAddLabel Step = "klikcepat_rot_label"

	// Klikcepat Bulk Setup
	StepKlikcepatRotBulkPick Step = "klc_bulk_pick"
	StepKlikcepatRotBulkPool Step = "klc_bulk_pool"

	// Klikcepat Block Rotator wizard (biolink button rotation)
	StepKlcBlockRotPickBlock Step = "klc_blk_pickblock"
	StepKlcBlockRotPickPool  Step = "klc_blk_pickpool"
	StepKlcBlockRotLabel     Step = "klc_blk_label"

	// Klikcepat Bulk Block Rotator
	StepKlcBlockBulkPick  Step = "klc_blkb_pick"
	StepKlcBlockBulkPool  Step = "klc_blkb_pool"
	StepKlcBlockBulkLabel Step = "klc_blkb_label"

	// Bulk label prompts (user kasih prefix label setelah pilih pool)
	StepRotatorBulkLabel      Step = "rotator_bulk_label"
	StepKlikcepatRotBulkLabel Step = "klc_bulk_label"

	// Group Commands wizard
	StepGroupCmdInputName     Step = "gcmd_input_name"
	StepGroupCmdInputDesc     Step = "gcmd_input_desc"

	// Klikcepat Edit search query
	StepKlikcepatEditSearchInputSL Step = "klc_edit_srch_in_sl"
	StepKlikcepatEditSearchInputBL Step = "klc_edit_srch_in_bl"
)

type Session struct {
	Step      Step
	Data      map[string]string
	PromptMsg *tele.Message
	CreatedAt time.Time // untuk TTL — session auto-expire setelah SessionTTL
}

type sessionStore struct {
	mu   sync.Mutex
	data map[int64]*Session
}

func newSessionStore() *sessionStore {
	s := &sessionStore{data: make(map[int64]*Session)}
	go s.cleanupLoop() // periodic cleanup orphan/expired sessions
	return s
}

func (s *sessionStore) Set(userID int64, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Set/refresh CreatedAt setiap kali session di-update
	sess.CreatedAt = time.Now()
	s.data[userID] = sess
}

func (s *sessionStore) Get(userID int64) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data[userID]
	if !ok {
		return nil, false
	}
	// Cek TTL — kalau expired, hapus & return not found
	if !sess.CreatedAt.IsZero() && time.Since(sess.CreatedAt) > SessionTTL {
		delete(s.data, userID)
		return nil, false
	}
	return sess, true
}

func (s *sessionStore) Delete(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, userID)
}

// cleanupLoop periodik hapus session yang udah expired.
// Cegah memory leak + stale state nyangkut.
func (s *sessionStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for userID, sess := range s.data {
			if !sess.CreatedAt.IsZero() && time.Since(sess.CreatedAt) > SessionTTL {
				delete(s.data, userID)
			}
		}
		s.mu.Unlock()
	}
}
