// Package mall contains account-scoped services for the mall, mystery shop,
// month-card rewards and QQ VIP rewards.
package mall

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
)

const (
	OrganicFertilizerGoodsID   int64 = 1002
	InorganicFertilizerGoodsID int64 = 1003
	FreeGiftSlotType           int32 = 1
	MallSlotType               int32 = 1

	mallBuyCooldown             = 10 * time.Minute
	maxFertilizerRounds         = 100
	maxFertilizerPerRound int64 = 10
	maxMysteryCycleBuys         = 20
	defaultMysteryPeriod        = time.Hour
)

var (
	ErrTransportRequired = errors.New("mall transport is required")
	ErrInvalidGoods      = errors.New("mall goods ID must be positive")
	ErrInvalidCount      = errors.New("mall purchase count must be positive")
	ErrSchedulerRequired = errors.New("mall scheduler is required")
)

// GameTransport is the protocol boundary shared with the warehouse domain.
// Keeping the dependency at the transport interface makes every mall service
// deterministic in tests and account-local at runtime.
type GameTransport = warehouse.GameTransport

// Config contains account-local collaborators shared by all four mall
// services. Optional callbacks keep persistence and game configuration out of
// the domain package while allowing the Runtime composition root to provide
// the real values.
type Config struct {
	Transport  GameTransport
	Warehouse  warehouse.Service
	Now        func() time.Time
	ItemName   func(int64) string
	Automation func(context.Context, string) (bool, error)
	Balance    func(context.Context, int64) (int64, error)

	MysteryAutoBuyEnabled    func(context.Context) (bool, error)
	MysteryAutoBuyCurrencies func(context.Context) ([]int64, error)
}

// Service is the mall contract consumed by farm and other domains. It
// intentionally contains only mall operations; the other three services are
// exposed through Domains so a farm cannot accidentally depend on them.
type Service interface {
	GetMallGoodsList(context.Context, int32) ([]Goods, error)
	PurchaseMallGoods(context.Context, int64, int64) (PurchaseResult, error)
	BuyFertilizer(context.Context, FertilizerType, int64, bool) (int64, error)
	AutoBuyOrganicFertilizer(context.Context, bool) (int64, error)
	CheckAndBuyFertilizerByThreshold(context.Context, FertilizerType, int64, float64, bool) (FertilizerCheckResult, error)
	CheckAndBuyFertilizerBoth(context.Context, FertilizerCheckOptions) (FertilizerBothResult, error)
	BuyFreeGifts(context.Context, bool) (int, error)
	FertilizerDailyState() FertilizerDailyState
	FreeGiftDailyState() FreeGiftDailyState
}

// Domains is the complete P5-04 mall domain. Mall is the narrow Service
// injected into farm; the remaining services are independently schedulable.
type Domains struct {
	Mall      Service
	Mystery   *MysteryService
	MonthCard *MonthCardService
	QQVIP     *QQVIPService
}

// Domains also forwards the narrow mall contract. This keeps composition
// roots free to retain the four-service aggregate while callers that only need
// fertilizer or mall RPCs can use the aggregate anywhere a Service is needed.
func (d *Domains) mall() (Service, error) {
	if d == nil || d.Mall == nil {
		return nil, ErrTransportRequired
	}
	return d.Mall, nil
}

func (d *Domains) GetMallGoodsList(ctx context.Context, slotType int32) ([]Goods, error) {
	s, err := d.mall()
	if err != nil {
		return nil, err
	}
	return s.GetMallGoodsList(ctx, slotType)
}

func (d *Domains) PurchaseMallGoods(ctx context.Context, goodsID, count int64) (PurchaseResult, error) {
	s, err := d.mall()
	if err != nil {
		return PurchaseResult{}, err
	}
	return s.PurchaseMallGoods(ctx, goodsID, count)
}

func (d *Domains) BuyFertilizer(ctx context.Context, fertilizer FertilizerType, count int64, force bool) (int64, error) {
	s, err := d.mall()
	if err != nil {
		return 0, err
	}
	return s.BuyFertilizer(ctx, fertilizer, count, force)
}

func (d *Domains) AutoBuyOrganicFertilizer(ctx context.Context, force bool) (int64, error) {
	s, err := d.mall()
	if err != nil {
		return 0, err
	}
	return s.AutoBuyOrganicFertilizer(ctx, force)
}

func (d *Domains) CheckAndBuyFertilizerByThreshold(ctx context.Context, fertilizer FertilizerType, count int64, threshold float64, force bool) (FertilizerCheckResult, error) {
	s, err := d.mall()
	if err != nil {
		return FertilizerCheckResult{}, err
	}
	return s.CheckAndBuyFertilizerByThreshold(ctx, fertilizer, count, threshold, force)
}

func (d *Domains) CheckAndBuyFertilizerBoth(ctx context.Context, options FertilizerCheckOptions) (FertilizerBothResult, error) {
	s, err := d.mall()
	if err != nil {
		return FertilizerBothResult{}, err
	}
	return s.CheckAndBuyFertilizerBoth(ctx, options)
}

func (d *Domains) BuyFreeGifts(ctx context.Context, force bool) (int, error) {
	s, err := d.mall()
	if err != nil {
		return 0, err
	}
	return s.BuyFreeGifts(ctx, force)
}

func (d *Domains) FertilizerDailyState() FertilizerDailyState {
	s, err := d.mall()
	if err != nil {
		return FertilizerDailyState{}
	}
	return s.FertilizerDailyState()
}

func (d *Domains) FreeGiftDailyState() FreeGiftDailyState {
	s, err := d.mall()
	if err != nil {
		return FreeGiftDailyState{}
	}
	return s.FreeGiftDailyState()
}

// New constructs all four account-local services.
func New(cfg Config) (*Domains, error) { return NewDomains(cfg) }

// NewDomains is the explicit aggregate constructor used by composition roots.
func NewDomains(cfg Config) (*Domains, error) {
	mallService, err := NewService(cfg)
	if err != nil {
		return nil, err
	}
	return &Domains{
		Mall:      mallService,
		Mystery:   NewMysteryService(cfg),
		MonthCard: NewMonthCardService(cfg),
		QQVIP:     NewQQVIPService(cfg),
	}, nil
}

var _ Service = (*Domains)(nil)

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c Config) itemName(id int64) string {
	if c.ItemName != nil {
		if value := strings.TrimSpace(c.ItemName(id)); value != "" {
			return value
		}
	}
	return fmt.Sprintf("物品#%d", id)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func ensureTransport(transport GameTransport) error {
	if transport == nil {
		return ErrTransportRequired
	}
	return nil
}

func dateKey(now time.Time) string { return now.Local().Format("2006-01-02") }

type serviceState struct {
	mu sync.Mutex

	lastBuyAt           time.Time
	buyDoneDateKey      string
	buyLastSuccessAt    time.Time
	pausedNoGoldDateKey string
	freeGiftDoneDateKey string
	freeGiftLastAt      time.Time
	freeGiftLastCheckAt time.Time
}

func (s *serviceState) buyAllowed(now time.Time, force bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !force && !s.lastBuyAt.IsZero() && now.Sub(s.lastBuyAt) < mallBuyCooldown {
		return false
	}
	s.lastBuyAt = now
	return true
}

// FertilizerType identifies the two mall fertilizer goods.
type FertilizerType string

const (
	FertilizerOrganic FertilizerType = "organic"
	FertilizerNormal  FertilizerType = "normal"
)

func (t FertilizerType) goodsID() (int64, error) {
	switch strings.ToLower(strings.TrimSpace(string(t))) {
	case "organic", "org", "有机", "":
		return OrganicFertilizerGoodsID, nil
	case "normal", "inorganic", "无机":
		return InorganicFertilizerGoodsID, nil
	default:
		return 0, fmt.Errorf("unknown fertilizer type %q", t)
	}
}

// FertilizerCheckOptions controls a dual-container threshold check.
type FertilizerCheckOptions struct {
	BuyOrganic          bool
	OrganicCount        int64
	OrganicThresholdHrs float64
	BuyNormal           bool
	NormalCount         int64
	NormalThresholdHrs  float64
	Force               bool
}

type FertilizerCheckResult struct {
	Type         FertilizerType
	Bought       int64
	CurrentHours float64
	ThresholdHrs float64
	Needed       bool
}

type FertilizerBothResult struct {
	OrganicBought       int64
	NormalBought        int64
	OrganicCurrentHours float64
	NormalCurrentHours  float64
}

type FertilizerDailyState struct {
	Key               string
	DoneToday         bool
	PausedNoGoldToday bool
	LastSuccessAt     time.Time
}

type FreeGiftDailyState struct {
	Key         string
	DoneToday   bool
	LastCheckAt time.Time
	LastClaimAt time.Time
}

// schedulerRegistrar is intentionally structural: account.RuntimeScheduler
// satisfies it without making account import the mall package.
type schedulerRegistrar interface {
	Every(string, time.Duration, time.Duration, any) error
}

type schedulerStopper interface {
	Stop(string) bool
}

var _ Service = (*mallService)(nil)
