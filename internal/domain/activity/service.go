// Package activity contains account-scoped services for the game's seasonal
// activities. Each activity implementation lives in its own file; this file
// only owns the aggregate contract and shared state.
package activity

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

var (
	ErrTransportRequired  = errors.New("activity transport is required")
	ErrRawTransportNeeded = errors.New("activity raw transport is required for this operation")
	ErrActivityIDRequired = errors.New("activity id must be positive")
	ErrActivityCommand    = errors.New("activity command must be non-negative")
	ErrInvalidSlot        = errors.New("activity slot id must be positive")
	ErrInvalidCount       = errors.New("activity count must be positive")
	ErrNotClaimable       = errors.New("activity reward is not claimable")
	ErrAlreadyClaimed     = errors.New("activity reward already claimed today")
	ErrNoMaterial         = errors.New("activity has no usable material")
)

const (
	ActivityServiceName = "gamepb.activitypb.ActivityService"
	SeasonServiceName   = "gamepb.seasonpb.SeasonService"
	SolarServiceName    = "gamepb.solartermspb.SolarTermsService"

	NanguaActivityUID              = "NanGua"
	NanguaShopActivityID     int64 = 2026030200
	NanguaRandomActivityID   int64 = 2026030201
	NanguaShopBuyCommand     int64 = 2
	NanguaShopRefreshCommand int64 = 3

	HeluActivityUID              = "SAIJI"
	HeluActivityID         int64 = 2026072700
	HeluDrawActivityID     int64 = 2026072701
	HeluExchangeActivityID int64 = 2026072702
	HeluDrawCommand        int64 = 9
	HeluExchangeCommand    int64 = 1
	HeluCurrencyItemID     int64 = 1023

	QingmeiActivityUID              = "QingMeiActivity"
	QingmeiActivityID         int64 = 2026081200
	QingmeiSeedClaimID        int64 = 2026081201
	QingmeiWineActivityID     int64 = 2026081202
	QingmeiSeedClaimCommand   int64 = 4
	QingmeiWinePreviewCommand int64 = 14
	QingmeiWineBrewCommand    int64 = 15
	QingmeiWineSellCommand    int64 = 16
	QingmeiDailyGrantID       int64 = 3
	QingmeiSeedItemID         int64 = 21221
	QingmeiFruitItemID        int64 = 41221

	HeluPassportUID            = "SAIJI_PASSPORT"
	GuanxingActivityID   int64 = 2026072701
	GuanxingClaimCommand int64 = 21
	StarsandItemID       int64 = 1023
)

// GameTransport is the shared authenticated RPC surface from the warehouse
// domain. Keeping one transport seam makes every activity deterministic in
// tests and avoids importing a session lifecycle into this package.
type GameTransport = warehouse.GameTransport

// RawTransport is implemented by the authenticated game session and exposes
// opaque protobuf response bodies for services without generated schemas.
type RawTransport interface {
	SendMsgRaw(context.Context, transport.Command, proto.Message) ([]byte, *pb.Meta, error)
}

// Service is the complete activity aggregate. Methods return normalized
// values for handlers, while Core methods expose the generated protobuf reply
// when callers need protocol-level detail.
type Service interface {
	GetActivityGroup(context.Context, int64, string) (*pb.GetGroupReply, error)
	ListActivities(context.Context) ([]*pb.ActivityInfo, error)
	OperateActivity(context.Context, int64, int64, OperateOptions) (*pb.OperateReply, error)

	GetNanguaShop(context.Context) (NanguaShop, error)
	BuyNanguaShopItem(context.Context, int64, int64) (NanguaShop, error)
	RefreshNanguaShop(context.Context) (NanguaShop, error)

	GetHeluActivity(context.Context) (HeluActivity, error)
	DrawHeluGiftLotus(context.Context, HeluDrawOptions) (HeluDrawResult, error)
	ExchangeHeluShopItem(context.Context, int64, int64) (HeluExchangeResult, error)

	GetQingmeiActivity(context.Context) (QingmeiActivity, error)
	ClaimQingmeiSeeds(context.Context) (QingmeiClaimResult, error)
	BrewAndSellQingmeiWine(context.Context, QingmeiWineOptions) (QingmeiWineResult, error)

	GetSeasonPassport(context.Context) (SeasonPassport, error)
	ClaimSeasonPassportRewards(context.Context) (SeasonClaimResult, error)
	GetSolarTermsInfo(context.Context) (SolarTermsInfo, error)
	ClaimSolarTermsReward(context.Context, int64) (SolarClaimResult, error)

	GetGuanxingActivity(context.Context) (GuanxingActivity, error)
	ClaimGuanxingRewards(context.Context) (GuanxingClaimResult, error)
	GetStarsandActivity(context.Context) (StarsandActivity, error)
}

// Config wires account-local collaborators. RawTransport is optional when a
// caller only uses formal ActivityService protobuf operations; a session that
// supports SendMsgRaw can be supplied directly through Transport.
type Config struct {
	Transport    GameTransport
	RawTransport RawTransport
	Warehouse    warehouse.Service
	ItemName     func(int64) string
	Now          func() time.Time
	Connected    func() bool

	HeluRequestGap   time.Duration
	HeluRefreshDelay time.Duration
	QingmeiStepDelay time.Duration
}

type service struct {
	transport    GameTransport
	raw          RawTransport
	warehouse    warehouse.Service
	itemName     func(int64) string
	now          func() time.Time
	connected    func() bool
	heluGap      time.Duration
	heluRefresh  time.Duration
	qingmeiDelay time.Duration

	stateMu            sync.Mutex
	qingmeiClaimedDate string
}

var _ Service = (*service)(nil)

// New constructs one account-local activity aggregate.
func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	raw := cfg.RawTransport
	if raw == nil {
		if candidate, ok := cfg.Transport.(RawTransport); ok {
			raw = candidate
		}
	}
	return &service{
		transport:    cfg.Transport,
		raw:          raw,
		warehouse:    cfg.Warehouse,
		itemName:     cfg.ItemName,
		now:          now,
		connected:    cfg.Connected,
		heluGap:      maxDuration(cfg.HeluRequestGap),
		heluRefresh:  maxDuration(cfg.HeluRefreshDelay),
		qingmeiDelay: maxDuration(cfg.QingmeiStepDelay),
	}, nil
}

// NewService is a descriptive constructor alias.
func NewService(cfg Config) (Service, error) { return New(cfg) }

// Close is provided so the aggregate can be stored as a Runtime domain.
func (s *service) Close() error { return nil }

// ActivityItem is the stable representation of an item embedded in an
// activity payload. Count is retained as an alias for JSON/API callers.
type ActivityItem struct {
	ItemID    int64
	ItemCount int64
	Count     int64
	UID       int64
	Name      string
}

// RandomShopItem describes one normalized random-shop slot.
type RandomShopItem struct {
	ID             int64
	Name           string
	Item           ActivityItem
	Cost           ActivityItem
	StockCount     int64
	BoughtCount    int64
	RemainingCount int64
	Special        bool
	SoldOut        bool
	Purchasable    bool
	StatusLabel    string
}

type RandomShop struct {
	Items                   []RandomShopItem
	NextRefreshTime         int64
	ManualRefreshCost       int64
	ManualRefreshCurrencyID int64
	ManualRefreshExtraValue int64
	MaxManualRefreshCount   int64
	ManualRefreshUsedCount  int64
}

type ExchangeShopItem struct {
	ID                  int64
	Sort                int64
	Status              int64
	Owned               bool
	Name                string
	Item                ActivityItem
	Cost                ActivityItem
	Extra               string
	IsRepeatable        bool
	ExchangeLimit       int64
	OwnedBlocksExchange bool
}

type NanguaShop struct {
	UID                string
	Title              string
	RandomActivityID   int64
	ExchangeActivityID int64
	RandomShop         RandomShop
	ExchangeShop       []ExchangeShopItem
	ActivityCount      int
	ActivityIDs        []int64
}

type DrawPoolItem struct {
	ID          int64
	Rarity      int64
	Item        ActivityItem
	Probability string
}

type DrawInfo struct {
	FreeMax        int64
	FreeUsed       int64
	FreeRemaining  int64
	PaidMax        int64
	PaidUsed       int64
	PaidRemaining  int64
	PaidCurrencyID int64
	PaidPrice      int64
	FallbackPrice  int64
	RewardPool     []DrawPoolItem
	Actions        DrawActions
}

type HeluActivity struct {
	UID                string
	Title              string
	ActivityID         int64
	DrawActivityID     int64
	ExchangeActivityID int64
	Draw               DrawInfo
	ExchangeShop       []ExchangeShopItem
	HeluBalance        int64
	ActivityCount      int
}

type HeluDrawOptions struct {
	Mode  string
	Count int64
}

type DrawAction struct {
	Count      int64
	Available  bool
	Cost       int64
	CurrencyID int64
	Type       string
	Label      string
}

type DrawActions struct {
	One   DrawAction
	Batch DrawAction
}

type DrawReward struct {
	SlotID int64
	Item   ActivityItem
	Flag   int64
}

type HeluDrawResult struct {
	OK           bool
	Count        int64
	ExpectedCost int64
	Mode         string
	CurrencyID   int64
	Rewards      []DrawReward
	Items        []ActivityItem
	Cost         ActivityItem
	Activity     HeluActivity
}

type HeluExchangeResult struct {
	OK         bool
	SlotID     int64
	Price      int64
	Count      int64
	TotalPrice int64
	CurrencyID int64
	Item       ExchangeShopItem
	Activity   HeluActivity
}

type QingmeiActivity struct {
	UID             string
	Title           string
	ActivityID      int64
	ClaimActivityID int64
	WineActivityID  int64
	Claimed         bool
	Claimable       bool
	StartTime       int64
	EndTime         int64
	Reward          ActivityItem
	Material        ActivityItem
}

type QingmeiClaimResult struct {
	OK             bool
	AlreadyClaimed bool
	ClaimedCount   int64
	BeforeCount    int64
	AfterCount     int64
	Rewards        []ActivityItem
	Qingmei        QingmeiActivity
}

type QingmeiWineOptions struct {
	Share     bool
	BrewSteps int
}

type QingmeiBrew struct {
	WineType  int64
	Cost      int64
	Price     int64
	CanDouble bool
}

type QingmeiWineSale struct {
	Multiple int64
	Gold     int64
	Item     ActivityItem
}

type QingmeiWineResult struct {
	OK                  bool
	BeforeMaterialCount int64
	AfterMaterialCount  int64
	ConsumedCount       int64
	Preview             int64
	Brews               []QingmeiBrew
	Brew                QingmeiBrew
	Shared              bool
	Sell                QingmeiWineSale
	Activity            QingmeiActivity
}

type SeasonRewardTier struct {
	Level          int64
	FreeRewards    []ActivityItem
	PremiumRewards []ActivityItem
}

type SeasonPassport struct {
	UID                 string
	Title               string
	ActivityID          int64
	CurrentLevel        int64
	Score               int64
	CurrentProgress     int64
	NextLevelNeed       int64
	MaxLevel            int64
	FreeClaimedLevel    int64
	PremiumClaimedLevel int64
	ClaimableLevels     int64
	RewardTiers         []SeasonRewardTier
	ConfigText          string
	StartTime           int64
	EndTime             int64
}

type SeasonClaimResult struct {
	OK            bool
	Rewards       []ActivityItem
	Passport      SeasonPassport
	ClaimedLevels int64
}

type SolarTerm struct {
	ID          int64
	Status      int64
	StatusLabel string
	Claimable   bool
	StartTime   int64
	EndTime     int64
	Title       string
	Rewards     []ActivityItem
}

type SolarTermsInfo struct {
	NowTime        int64
	Terms          []SolarTerm
	ClaimableCount int
	CurrentTerm    *SolarTerm
	TipsText       string
}

type SolarClaimResult struct {
	OK         bool
	TermID     int64
	Rewards    []ActivityItem
	Term       *SolarTerm
	SolarTerms SolarTermsInfo
}

type GuanxingNode struct {
	ID          int64
	Day         int64
	Name        string
	Category    string
	Explain     string
	Links       string
	Unlocked    bool
	Claimed     bool
	Claimable   bool
	StatusLabel string
	Rewards     []ActivityItem
}

type GuanxingActivity struct {
	ActivityID     int64
	Title          string
	SeasonTitle    string
	StartTime      int64
	EndTime        int64
	NowTime        int64
	CurrentDay     int64
	TotalDays      int
	Nodes          []GuanxingNode
	ClaimableCount int
	ClaimedCount   int
	UnlockedCount  int
	PendingRewards []ActivityItem
	Warning        string
}

type GuanxingClaimResult struct {
	OK             bool
	Claimed        bool
	AlreadyClaimed bool
	ClaimedNodes   []GuanxingNode
	Rewards        []ActivityItem
	Activity       GuanxingActivity
}

type StarsandActivity struct {
	ItemID int64
	Title  string
	Note   string
}

func maxDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}
