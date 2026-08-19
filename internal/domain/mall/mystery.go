package mall

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

const mysteryServiceName = "gamepb.mysteryshoppb.MysteryShopService"

var currencyNames = map[int64]string{
	1001: "金币",
	1002: "点券",
	1005: "金豆豆",
}

type MysteryOffer struct {
	Active        bool
	NPCID         int64
	ItemID        int64
	ItemType      int64
	ItemCount     int64
	ItemName      string
	CurrencyID    int64
	CurrencyName  string
	Price         int64
	OriginalPrice int64
	Discount      int64
	Purchased     bool
	StartTime     time.Time
	EndTime       time.Time
	Raw           *pb.GetActiveNPCReply
}

type MysteryPurchaseResult struct {
	RewardItemID int64
	RewardCount  int64
	Purchased    bool
	Raw          *pb.BuyReply
}

type MysteryDailyState struct {
	LastRunAt time.Time
	Bought    int
}

type MysteryService struct {
	transport GameTransport
	config    Config

	stateMu sync.Mutex
	state   MysteryDailyState
}

func NewMysteryService(cfg Config) *MysteryService {
	return &MysteryService{transport: cfg.Transport, config: cfg}
}

func (s *MysteryService) GetActiveMysteryShop(ctx context.Context) (MysteryOffer, error) {
	if err := checkContext(ctx); err != nil {
		return MysteryOffer{}, err
	}
	if err := ensureTransport(s.transport); err != nil {
		return MysteryOffer{}, err
	}
	request := &pb.GetActiveNPCRequest{}
	read := func() (MysteryOffer, error) {
		reply := new(pb.GetActiveNPCReply)
		response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: mysteryServiceName, MethodName: "GetActiveNPC", Response: reply, Timeout: 60 * time.Second}, request)
		if err != nil {
			return MysteryOffer{}, err
		}
		if response != nil {
			var ok bool
			reply, ok = response.(*pb.GetActiveNPCReply)
			if !ok {
				return MysteryOffer{}, fmt.Errorf("get active mystery shop returned %T, want *pb.GetActiveNPCReply", response)
			}
		}
		offer := normalizeMysteryOffer(reply, s.config.now())
		if offer.ItemID > 0 {
			offer.ItemName = s.config.itemName(offer.ItemID)
		}
		return offer, nil
	}
	offer, err := read()
	if err == nil {
		return offer, nil
	}
	// The Node implementation retries one timeout. Keeping one retry here
	// makes transient gateway delays transparent without an unbounded loop.
	return read()
}

func (s *MysteryService) BuyMysteryShopGoods(ctx context.Context, npcID int64) (MysteryPurchaseResult, error) {
	if err := checkContext(ctx); err != nil {
		return MysteryPurchaseResult{}, err
	}
	if err := ensureTransport(s.transport); err != nil {
		return MysteryPurchaseResult{}, err
	}
	if npcID <= 0 {
		return MysteryPurchaseResult{}, fmt.Errorf("mystery NPC ID must be positive")
	}
	reply := new(pb.BuyReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: mysteryServiceName, MethodName: "Buy", Response: reply}, &pb.BuyRequest{NpcId: npcID})
	if err != nil {
		return MysteryPurchaseResult{}, fmt.Errorf("buy mystery shop NPC %d: %w", npcID, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.BuyReply)
		if !ok {
			return MysteryPurchaseResult{}, fmt.Errorf("buy mystery shop returned %T, want *pb.BuyReply", response)
		}
	}
	reward := reply.GetReward()
	purchased := reply.GetNpc() != nil && reply.GetNpc().GetPurchased()
	return MysteryPurchaseResult{RewardItemID: int64(reward.GetItemId()), RewardCount: int64(reward.GetCount()), Purchased: purchased, Raw: reply}, nil
}

func (s *MysteryService) AbandonMysteryShop(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := ensureTransport(s.transport); err != nil {
		return err
	}
	reply := new(pb.AbandonReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{ServiceName: mysteryServiceName, MethodName: "Abandon", Response: reply}, &pb.AbandonRequest{})
	if err != nil {
		return fmt.Errorf("abandon mystery shop: %w", err)
	}
	if response != nil {
		if _, ok := response.(*pb.AbandonReply); !ok {
			return fmt.Errorf("abandon mystery shop returned %T, want *pb.AbandonReply", response)
		}
	}
	return nil
}

// RunAutoBuyOnce performs one offer query and purchase, matching the startup
// behavior of mystery-scheduler.js.
func (s *MysteryService) RunAutoBuyOnce(ctx context.Context) (bool, error) {
	if enabled, err := s.autoBuyEnabled(ctx); err != nil || !enabled {
		return false, err
	}
	currencies, err := s.autoBuyCurrencies(ctx)
	if err != nil {
		return false, err
	}
	if len(currencies) == 0 {
		return false, nil
	}
	offer, err := s.GetActiveMysteryShop(ctx)
	if err != nil {
		return false, err
	}
	if !offer.Active || !containsCurrency(currencies, offer.CurrencyID) {
		return false, nil
	}
	_, err = s.BuyMysteryShopGoods(ctx, offer.NPCID)
	if err != nil {
		return false, err
	}
	s.recordRun(1)
	return true, nil
}

// RunAutoBuyCycle buys at most maxMysteryCycleBuys offers and stops when the
// server keeps returning the same NPC, preventing a stuck queue loop.
func (s *MysteryService) RunAutoBuyCycle(ctx context.Context) (int, error) {
	if enabled, err := s.autoBuyEnabled(ctx); err != nil || !enabled {
		return 0, err
	}
	currencies, err := s.autoBuyCurrencies(ctx)
	if err != nil {
		return 0, err
	}
	if len(currencies) == 0 {
		return 0, nil
	}
	bought := 0
	var lastNPC int64 = -1
	for i := 0; i < maxMysteryCycleBuys; i++ {
		offer, err := s.GetActiveMysteryShop(ctx)
		if err != nil {
			return bought, err
		}
		if !offer.Active || offer.NPCID == lastNPC || !containsCurrency(currencies, offer.CurrencyID) {
			break
		}
		if _, err := s.BuyMysteryShopGoods(ctx, offer.NPCID); err != nil {
			return bought, err
		}
		bought++
		lastNPC = offer.NPCID
	}
	if bought > 0 {
		s.recordRun(bought)
	}
	return bought, nil
}

// RegisterAutoBuy attaches the hourly mystery check to an account scheduler.
// The caller owns scheduler lifecycle; no package-level timer is created.
func (s *MysteryService) RegisterAutoBuy(scheduler schedulerRegistrar) error {
	if scheduler == nil {
		return ErrSchedulerRequired
	}
	return scheduler.Every("mall:mystery-auto-buy", defaultMysteryPeriod, 0, func(ctx context.Context) error {
		_, err := s.RunAutoBuyCycle(ctx)
		return err
	})
}

// StartAutoBuy performs the immediate startup check and registers the hourly
// task. It is convenient for Runtime startup and remains cancellation-aware.
func (s *MysteryService) StartAutoBuy(ctx context.Context, scheduler schedulerRegistrar) error {
	enabled, err := s.autoBuyEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if _, err := s.RunAutoBuyOnce(ctx); err != nil {
		return err
	}
	return s.RegisterAutoBuy(scheduler)
}

func (s *MysteryService) StopAutoBuy(scheduler schedulerStopper) bool {
	if scheduler == nil {
		return false
	}
	return scheduler.Stop("mall:mystery-auto-buy")
}

func (s *MysteryService) DailyState() MysteryDailyState {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.state
}

func (s *MysteryService) autoBuyEnabled(ctx context.Context) (bool, error) {
	if s.config.MysteryAutoBuyEnabled == nil {
		return true, nil
	}
	return s.config.MysteryAutoBuyEnabled(ctx)
}

func (s *MysteryService) autoBuyCurrencies(ctx context.Context) ([]int64, error) {
	if s.config.MysteryAutoBuyCurrencies == nil {
		return nil, nil
	}
	values, err := s.config.MysteryAutoBuyCurrencies(ctx)
	if err != nil {
		return nil, err
	}
	return append([]int64(nil), values...), nil
}

func (s *MysteryService) recordRun(bought int) {
	s.stateMu.Lock()
	s.state.LastRunAt = s.config.now()
	s.state.Bought += bought
	s.stateMu.Unlock()
}

func normalizeMysteryOffer(reply *pb.GetActiveNPCReply, now time.Time) MysteryOffer {
	if reply == nil {
		return MysteryOffer{}
	}
	npc := reply.GetNpc()
	if npc == nil {
		return MysteryOffer{Active: false, StartTime: unixSeconds(reply.GetStartTime()), EndTime: unixSeconds(reply.GetEndTime()), Raw: reply}
	}
	end := unixSeconds(reply.GetEndTime())
	active := reply.GetActive() && !npc.GetPurchased()
	if !end.IsZero() && !end.After(now) {
		active = false
	}
	return MysteryOffer{Active: active, NPCID: npc.GetNpcId(), ItemID: int64(npc.GetItemId()), ItemType: int64(npc.GetItemType()), ItemCount: int64(npc.GetItemCount()), CurrencyID: int64(npc.GetCurrencyId()), CurrencyName: currencyName(npc.GetCurrencyId()), Price: npc.GetPrice(), OriginalPrice: npc.GetOriginalPrice(), Discount: int64(npc.GetDiscount()), Purchased: npc.GetPurchased(), StartTime: unixSeconds(reply.GetStartTime()), EndTime: end, Raw: reply}
}

func NormalizeMysteryOffer(reply *pb.GetActiveNPCReply, now time.Time) MysteryOffer {
	return normalizeMysteryOffer(reply, now)
}

func unixSeconds(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func currencyName(id int32) string {
	if name, ok := currencyNames[int64(id)]; ok {
		return name
	}
	return fmt.Sprintf("货币%d", id)
}

func containsCurrency(values []int64, wanted int64) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
