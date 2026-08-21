package account

import (
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

// RuntimePhase describes the lifecycle phase of one account runtime.
type RuntimePhase string

const (
	PhaseIdle     RuntimePhase = "idle"
	PhaseStarting RuntimePhase = "starting"
	PhaseOnline   RuntimePhase = "online"
	PhaseStopping RuntimePhase = "stopping"
	PhaseOffline  RuntimePhase = "offline"
	PhaseError    RuntimePhase = "error"
)

// StatusSnapshot is a point-in-time account runtime view. It contains only
// account-local state so callers can safely publish it to a realtime hub.
type StatusSnapshot struct {
	AccountID string       `json:"account_id"`
	Phase     RuntimePhase `json:"phase"`
	Running   bool         `json:"running"`
	Online    bool         `json:"online"`

	StartedAt        time.Time `json:"started_at,omitempty"`
	StoppedAt        time.Time `json:"stopped_at,omitempty"`
	LastTransitionAt time.Time `json:"last_transition_at,omitempty"`
	LastError        string    `json:"last_error,omitempty"`

	GID      int64  `json:"gid"`
	Name     string `json:"name"`
	Level    int64  `json:"level"`
	Gold     int64  `json:"gold"`
	Exp      int64  `json:"exp"`
	Coupon   int64  `json:"coupon"`
	GoldBean int64  `json:"gold_bean"`
	OpenID   string `json:"openid"`
	Avatar   string `json:"avatar"`

	NextAction   string             `json:"next_action,omitempty"`
	NextActionAt time.Time          `json:"next_action_at,omitempty"`
	Operations   map[string]float64 `json:"operations,omitempty"`
}

// StatusState owns the mutable runtime status for one account. The mutex is
// intentionally per instance; no account shares this state or its map.
type StatusState struct {
	mu       sync.RWMutex
	snapshot StatusSnapshot
	onChange func(StatusSnapshot)
}

// SetOnChange installs the account-local status publisher. The callback runs
// after the state lock is released, so realtime consumers cannot block status
// readers or deadlock a Runtime transition.
func (s *StatusState) SetOnChange(callback func(StatusSnapshot)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onChange = callback
	snapshot := cloneStatus(s.snapshot)
	s.mu.Unlock()
	if callback != nil {
		callback(snapshot)
	}
}

// NewStatusState creates an idle status container for accountID.
func NewStatusState(accountID string) *StatusState {
	return &StatusState{
		snapshot: StatusSnapshot{
			AccountID:  strings.TrimSpace(accountID),
			Phase:      PhaseIdle,
			Operations: make(map[string]float64),
		},
	}
}

// Snapshot returns a copy whose map cannot be mutated through the status
// owner. This keeps readers race-free even when domain loops update counters.
func (s *StatusState) Snapshot() StatusSnapshot {
	if s == nil {
		return StatusSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneStatus(s.snapshot)
}

// MarkStarting records a new start attempt.
func (s *StatusState) MarkStarting(now time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.Phase = PhaseStarting
		snapshot.Running = true
		snapshot.Online = false
		snapshot.StartedAt = now
		snapshot.StoppedAt = time.Time{}
		snapshot.LastError = ""
		snapshot.LastTransitionAt = now
	})
}

// MarkOnline records a successfully authenticated account.
func (s *StatusState) MarkOnline(state transport.UserState, now time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.Phase = PhaseOnline
		snapshot.Running = true
		snapshot.Online = true
		if snapshot.StartedAt.IsZero() {
			snapshot.StartedAt = now
		}
		snapshot.LastError = ""
		snapshot.LastTransitionAt = now
		applyUserState(snapshot, state)
	})
}

// MarkStopping records that shutdown has begun and makes the account appear
// offline before resource teardown completes.
func (s *StatusState) MarkStopping(now time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.Phase = PhaseStopping
		snapshot.Running = false
		snapshot.Online = false
		snapshot.LastTransitionAt = now
	})
}

// MarkOffline records a clean or failed stop. An empty error is a clean stop.
func (s *StatusState) MarkOffline(err error, now time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.Phase = PhaseOffline
		snapshot.Running = false
		snapshot.Online = false
		snapshot.StoppedAt = now
		snapshot.LastTransitionAt = now
		snapshot.LastError = errorString(err)
	})
}

// MarkError records a failed start while keeping the last account identity
// snapshot available for diagnostics.
func (s *StatusState) MarkError(err error, now time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.Phase = PhaseError
		snapshot.Running = false
		snapshot.Online = false
		snapshot.LastError = errorString(err)
		snapshot.LastTransitionAt = now
	})
}

// UpdateUser updates account resources without changing lifecycle state.
func (s *StatusState) UpdateUser(state transport.UserState) {
	s.with(func(snapshot *StatusSnapshot) { applyUserState(snapshot, state) })
}

// SetGold updates the account balance after a domain operation.
func (s *StatusState) SetGold(value int64) {
	s.with(func(snapshot *StatusSnapshot) { snapshot.Gold = value })
}

// SetNextAction publishes the next scheduled action and its deadline.
func (s *StatusState) SetNextAction(name string, at time.Time) {
	s.with(func(snapshot *StatusSnapshot) {
		snapshot.NextAction = strings.TrimSpace(name)
		snapshot.NextActionAt = at
	})
}

// SetOperation replaces one operation counter.
func (s *StatusState) SetOperation(name string, value float64) {
	s.with(func(snapshot *StatusSnapshot) {
		if snapshot.Operations == nil {
			snapshot.Operations = make(map[string]float64)
		}
		if key := strings.TrimSpace(name); key != "" {
			snapshot.Operations[key] = value
		}
	})
}

// AddOperation atomically increments one operation counter.
func (s *StatusState) AddOperation(name string, delta float64) {
	s.with(func(snapshot *StatusSnapshot) {
		if snapshot.Operations == nil {
			snapshot.Operations = make(map[string]float64)
		}
		if key := strings.TrimSpace(name); key != "" {
			snapshot.Operations[key] += delta
		}
	})
}

func (s *StatusState) with(update func(*StatusSnapshot)) {
	if s == nil || update == nil {
		return
	}
	s.mu.Lock()
	update(&s.snapshot)
	snapshot := cloneStatus(s.snapshot)
	callback := s.onChange
	s.mu.Unlock()
	if callback != nil {
		callback(snapshot)
	}
}

func cloneStatus(snapshot StatusSnapshot) StatusSnapshot {
	if snapshot.Operations != nil {
		snapshot.Operations = make(map[string]float64, len(snapshot.Operations))
		for key, value := range snapshot.Operations {
			snapshot.Operations[key] = value
		}
	}
	return snapshot
}

func applyUserState(snapshot *StatusSnapshot, state transport.UserState) {
	snapshot.GID = state.GID
	snapshot.Name = state.Name
	snapshot.Level = state.Level
	snapshot.Gold = state.Gold
	snapshot.Exp = state.Exp
	snapshot.Coupon = state.Coupon
	snapshot.GoldBean = state.GoldBean
	snapshot.OpenID = state.OpenID
	snapshot.Avatar = state.Avatar
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
