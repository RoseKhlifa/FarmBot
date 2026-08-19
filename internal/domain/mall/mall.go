package mall

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

const mallServiceName = "gamepb.mallpb.MallService"

// Goods is the normalized mall item used by domain callers. Raw protobuf
// fields that use compact nested wire values are decoded into the typed price,
// limit and item ID fields while Raw remains available for future fields.
type Goods struct {
	ID       int64
	Name     string
	Type     int32
	ItemIDs  []int64
	Price    int64
	PriceRaw []byte
	Free     bool
	Limit    LimitInfo
	Limited  bool
	Discount string
	Raw      *pb.MallGoods
}

type LimitInfo struct {
	LimitCount int64
	BoughtNum  int64
}

type PurchaseResult struct {
	GoodsID    int64
	Count      int64
	RewardInfo []byte
	Result     []byte
	Raw        *pb.PurchaseResponse
}

type mallService struct {
	transport GameTransport
	warehouse warehouse.Service
	config    Config
	state     serviceState
}

// NewService constructs the mall/fertilizer service consumed by farm.
func NewService(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	return &mallService{transport: cfg.Transport, warehouse: cfg.Warehouse, config: cfg}, nil
}

// NewMallService is a descriptive constructor alias.
func NewMallService(cfg Config) (Service, error) { return NewService(cfg) }

func (s *mallService) GetMallGoodsList(ctx context.Context, slotType int32) ([]Goods, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if slotType <= 0 {
		slotType = MallSlotType
	}
	reply := new(pb.GetMallListBySlotTypeResponse)
	response, err := s.transport.SendMsg(ctx, command("GetMallListBySlotType", reply), &pb.GetMallListBySlotTypeRequest{SlotType: slotType})
	if err != nil {
		return nil, fmt.Errorf("get mall goods list: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetMallListBySlotTypeResponse)
		if !ok {
			return nil, fmt.Errorf("get mall goods list returned %T, want *pb.GetMallListBySlotTypeResponse", response)
		}
	}
	goods := make([]Goods, 0, len(reply.GetGoodsList()))
	for _, raw := range reply.GetGoodsList() {
		item := new(pb.MallGoods)
		if err := protoUnmarshal(raw, item); err != nil {
			// The Node service skips malformed entries and keeps the rest of the
			// slot usable. Preserve that behavior for a partially corrupt reply.
			continue
		}
		goods = append(goods, normalizeGoods(item, s.config.itemName))
	}
	return goods, nil
}

// GetGoodsList is a short alias used by domain callers.
func (s *mallService) GetGoodsList(ctx context.Context, slotType int32) ([]Goods, error) {
	return s.GetMallGoodsList(ctx, slotType)
}

func (s *mallService) PurchaseMallGoods(ctx context.Context, goodsID, count int64) (PurchaseResult, error) {
	if err := checkContext(ctx); err != nil {
		return PurchaseResult{}, err
	}
	if goodsID <= 0 {
		return PurchaseResult{}, ErrInvalidGoods
	}
	if count <= 0 || count > math.MaxInt32 {
		return PurchaseResult{}, ErrInvalidCount
	}
	reply := new(pb.PurchaseResponse)
	response, err := s.transport.SendMsg(ctx, command("Purchase", reply), &pb.PurchaseRequest{GoodsId: int32(goodsID), Count: int32(count)})
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("purchase mall goods %d: %w", goodsID, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.PurchaseResponse)
		if !ok {
			return PurchaseResult{}, fmt.Errorf("purchase mall goods returned %T, want *pb.PurchaseResponse", response)
		}
	}
	return PurchaseResult{
		GoodsID: int64(reply.GetGoodsId()), Count: int64(reply.GetCount()),
		RewardInfo: append([]byte(nil), reply.GetRewardInfo()...), Result: append([]byte(nil), reply.GetResult()...), Raw: reply,
	}, nil
}

// Purchase is a short alias for PurchaseMallGoods.
func (s *mallService) Purchase(ctx context.Context, goodsID, count int64) (PurchaseResult, error) {
	return s.PurchaseMallGoods(ctx, goodsID, count)
}

func (s *mallService) BuyFertilizer(ctx context.Context, fertilizer FertilizerType, targetCount int64, force bool) (int64, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	if targetCount < 0 {
		return 0, fmt.Errorf("fertilizer target count must not be negative")
	}
	goodsID, err := fertilizer.goodsID()
	if err != nil {
		return 0, err
	}
	if !s.state.buyAllowed(s.config.now(), force) {
		return 0, nil
	}
	goodsList, err := s.GetMallGoodsList(ctx, MallSlotType)
	if err != nil {
		return 0, err
	}
	var goods *Goods
	for index := range goodsList {
		if goodsList[index].ID == goodsID {
			goods = &goodsList[index]
			break
		}
	}
	if goods == nil {
		return 0, fmt.Errorf("fertilizer goods %d not found", goodsID)
	}
	price := goods.Price
	availableBalance, balanceKnown := s.balance(ctx, 1002)
	perRound := maxBuyPerRound(price, availableBalance)
	limit := targetCount
	var bought int64
	for round := 0; round < maxFertilizerRounds; round++ {
		if limit > 0 && bought >= limit {
			break
		}
		buyCount := perRound
		if limit > 0 && buyCount > limit-bought {
			buyCount = limit - bought
		}
		if buyCount <= 0 {
			break
		}
		if price > 0 && balanceKnown {
			if availableBalance < price {
				s.markNoGold()
				break
			}
			if buyCount > availableBalance/price {
				buyCount = availableBalance / price
				if buyCount <= 0 {
					s.markNoGold()
					break
				}
			}
		}
		_, purchaseErr := s.PurchaseMallGoods(ctx, goodsID, buyCount)
		if purchaseErr != nil {
			if perRound > 1 {
				perRound = 1
				continue
			}
			return bought, purchaseErr
		}
		bought += buyCount
		if price > 0 && balanceKnown {
			availableBalance -= price * buyCount
		}
		if err := waitContext(ctx, 300*time.Millisecond); err != nil {
			return bought, err
		}
	}
	if bought > 0 {
		s.state.mu.Lock()
		s.state.buyDoneDateKey = dateKey(s.config.now())
		s.state.buyLastSuccessAt = s.config.now()
		s.state.mu.Unlock()
	}
	return bought, nil
}

func (s *mallService) AutoBuyOrganicFertilizer(ctx context.Context, force bool) (int64, error) {
	return s.BuyFertilizer(ctx, FertilizerOrganic, 0, force)
}

func (s *mallService) AutoBuyFertilizer(ctx context.Context, fertilizer FertilizerType, targetCount int64, force bool) (int64, error) {
	return s.BuyFertilizer(ctx, fertilizer, targetCount, force)
}

func (s *mallService) CheckAndBuyFertilizerByThreshold(ctx context.Context, fertilizer FertilizerType, count int64, thresholdHours float64, force bool) (FertilizerCheckResult, error) {
	result := FertilizerCheckResult{Type: fertilizer, ThresholdHrs: thresholdHours}
	if count <= 0 || thresholdHours <= 0 {
		return result, fmt.Errorf("fertilizer count and threshold must be positive")
	}
	if s.warehouse == nil {
		return result, errors.New("warehouse service is required for fertilizer threshold checks")
	}
	bag, err := s.warehouse.ListBag(ctx)
	if err != nil {
		return result, fmt.Errorf("list bag for fertilizer threshold: %w", err)
	}
	hours := warehouse.ContainerHours(bag.Items)
	if fertilizer == FertilizerNormal {
		result.CurrentHours = hours.Normal
	} else {
		result.CurrentHours = hours.Organic
	}
	if result.CurrentHours >= thresholdHours {
		return result, nil
	}
	result.Needed = true
	bought, buyErr := s.BuyFertilizer(ctx, fertilizer, count, force)
	result.Bought = bought
	return result, buyErr
}

func (s *mallService) CheckAndBuyFertilizerBoth(ctx context.Context, options FertilizerCheckOptions) (FertilizerBothResult, error) {
	result := FertilizerBothResult{}
	if !options.BuyOrganic && !options.BuyNormal {
		return result, nil
	}
	if s.warehouse == nil {
		return result, errors.New("warehouse service is required for fertilizer threshold checks")
	}
	bag, err := s.warehouse.ListBag(ctx)
	if err != nil {
		return result, fmt.Errorf("list bag for fertilizer threshold: %w", err)
	}
	hours := warehouse.ContainerHours(bag.Items)
	result.OrganicCurrentHours, result.NormalCurrentHours = hours.Organic, hours.Normal
	if options.BuyOrganic && options.OrganicCount > 0 && options.OrganicThresholdHrs > 0 && hours.Organic < options.OrganicThresholdHrs {
		result.OrganicBought, err = s.BuyFertilizer(ctx, FertilizerOrganic, options.OrganicCount, options.Force)
		if err != nil {
			return result, err
		}
	}
	if options.BuyNormal && options.NormalCount > 0 && options.NormalThresholdHrs > 0 && hours.Normal < options.NormalThresholdHrs {
		result.NormalBought, err = s.BuyFertilizer(ctx, FertilizerNormal, options.NormalCount, true)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *mallService) BuyFreeGifts(ctx context.Context, force bool) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	now := s.config.now()
	s.state.mu.Lock()
	if !force && s.state.freeGiftDoneDateKey == dateKey(now) {
		s.state.mu.Unlock()
		return 0, nil
	}
	if !force && !s.state.freeGiftLastCheckAt.IsZero() && now.Sub(s.state.freeGiftLastCheckAt) < mallBuyCooldown {
		s.state.mu.Unlock()
		return 0, nil
	}
	s.state.freeGiftLastCheckAt = now
	s.state.mu.Unlock()
	goods, err := s.GetMallGoodsList(ctx, FreeGiftSlotType)
	if err != nil {
		return 0, err
	}
	claimed := 0
	for _, item := range goods {
		if !item.Free || item.ID <= 0 {
			continue
		}
		if _, err := s.PurchaseMallGoods(ctx, item.ID, 1); err != nil {
			continue
		}
		claimed++
	}
	s.state.mu.Lock()
	s.state.freeGiftDoneDateKey = dateKey(now)
	if claimed > 0 {
		s.state.freeGiftLastAt = s.config.now()
	}
	s.state.mu.Unlock()
	return claimed, nil
}

func (s *mallService) FertilizerDailyState() FertilizerDailyState {
	now := s.config.now()
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return FertilizerDailyState{Key: "fertilizer_buy", DoneToday: s.state.buyDoneDateKey == dateKey(now), PausedNoGoldToday: s.state.pausedNoGoldDateKey == dateKey(now), LastSuccessAt: s.state.buyLastSuccessAt}
}

func (s *mallService) FreeGiftDailyState() FreeGiftDailyState {
	now := s.config.now()
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return FreeGiftDailyState{Key: "mall_free_gifts", DoneToday: s.state.freeGiftDoneDateKey == dateKey(now), LastCheckAt: s.state.freeGiftLastCheckAt, LastClaimAt: s.state.freeGiftLastAt}
}

func (s *mallService) balance(ctx context.Context, currencyID int64) (int64, bool) {
	if s.config.Balance == nil {
		return 0, false
	}
	value, err := s.config.Balance(ctx, currencyID)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func (s *mallService) markNoGold() {
	s.state.mu.Lock()
	s.state.pausedNoGoldDateKey = dateKey(s.config.now())
	s.state.mu.Unlock()
}

func maxBuyPerRound(price, balance int64) int64 {
	if price <= 0 || balance <= 0 {
		return maxFertilizerPerRound
	}
	value := balance / price
	if value < 1 {
		return 1
	}
	if value > maxFertilizerPerRound {
		return maxFertilizerPerRound
	}
	return value
}

func normalizeGoods(item *pb.MallGoods, itemName func(int64) string) Goods {
	if item == nil {
		return Goods{}
	}
	id := int64(item.GetGoodsId())
	name := item.GetName()
	if name == "" && itemName != nil {
		name = itemName(id)
	}
	return Goods{ID: id, Name: name, Type: item.GetType(), ItemIDs: ParseMallItemIDs(item.GetItemIds()), Price: ParseMallPriceValue(item.GetPrice()), PriceRaw: append([]byte(nil), item.GetPrice()...), Free: item.GetIsFree(), Limit: ParseMallLimitInfo(item.GetLimit()), Limited: item.GetIsLimited(), Discount: item.GetDiscount(), Raw: item}
}

// ParseMallPriceValue parses the nested one-field protobuf used by MallGoods.
func ParseMallPriceValue(raw []byte) int64 {
	fields := parseVarintFields(raw)
	if value, ok := fields[1]; ok {
		return int64(value)
	}
	return 0
}

// ParseMallLimitInfo parses fields 1 (limit count) and 2 (bought count).
func ParseMallLimitInfo(raw []byte) LimitInfo {
	fields := parseVarintFields(raw)
	return LimitInfo{LimitCount: int64(fields[1]), BoughtNum: int64(fields[2])}
}

// ParseMallItemIDs parses repeated field 1 from the nested item ID message.
func ParseMallItemIDs(raw []byte) []int64 {
	values := parseRepeatedVarintField(raw, 1)
	return append([]int64(nil), values...)
}

func parseVarintFields(raw []byte) map[uint64]uint64 {
	fields := make(map[uint64]uint64)
	for len(raw) > 0 {
		key, value, consumed, ok := readVarintField(raw)
		if !ok {
			break
		}
		fields[key] = value
		raw = raw[consumed:]
	}
	return fields
}

func parseRepeatedVarintField(raw []byte, wanted uint64) []int64 {
	values := make([]int64, 0)
	for len(raw) > 0 {
		key, value, consumed, ok := readVarintField(raw)
		if !ok {
			break
		}
		if key == wanted {
			values = append(values, int64(value))
		}
		raw = raw[consumed:]
	}
	return values
}

func readVarintField(raw []byte) (uint64, uint64, int, bool) {
	key, keyN := binary.Uvarint(raw)
	if keyN <= 0 {
		return 0, 0, 0, false
	}
	fieldNumber, wireType := key>>3, key&7
	if wireType != 0 {
		return 0, 0, 0, false
	}
	value, valueN := binary.Uvarint(raw[keyN:])
	if valueN <= 0 {
		return 0, 0, 0, false
	}
	return fieldNumber, value, keyN + valueN, true
}

func command(method string, response proto.Message) transport.Command {
	return transport.Command{ServiceName: mallServiceName, MethodName: method, Response: response}
}

func protoUnmarshal(raw []byte, target proto.Message) error {
	return proto.Unmarshal(raw, target)
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return checkContext(ctx)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
