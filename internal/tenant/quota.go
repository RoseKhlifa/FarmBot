package tenant

import (
	"context"
	"errors"
	"sync"
)

// Reservation is an in-process guard against two simultaneous account-create
// requests both passing a preflight count before either transaction commits.
type Reservation struct {
	manager *ReservationManager
	key     string
	active  bool
}

type ReservationManager struct {
	mu       sync.Mutex
	reserved map[string]int
}

func NewReservationManager() *ReservationManager {
	return &ReservationManager{reserved: make(map[string]int)}
}

func (m *ReservationManager) ReserveCreate(ctx context.Context, tenantID string, check func(context.Context, string) error) (*Reservation, error) {
	if m == nil {
		return nil, errors.New("tenant reservation manager is nil")
	}
	if check == nil {
		return nil, errors.New("tenant quota check is nil")
	}
	if err := check(ctx, tenantID); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.reserved[tenantID]++
	m.mu.Unlock()
	return &Reservation{manager: m, key: tenantID, active: true}, nil
}

func (r *Reservation) Release() {
	if r == nil || !r.active || r.manager == nil {
		return
	}
	r.manager.mu.Lock()
	if r.manager.reserved[r.key] > 1 {
		r.manager.reserved[r.key]--
	} else {
		delete(r.manager.reserved, r.key)
	}
	r.manager.mu.Unlock()
	r.active = false
}
