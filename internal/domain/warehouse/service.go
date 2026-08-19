package warehouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"google.golang.org/protobuf/proto"
)

const (
	itemService        = "gamepb.itempb.ItemService"
	defaultSellBatch   = 15
	normalContainerID  = int64(1011)
	organicContainerID = int64(1012)
)

var (
	ErrTransportRequired = errors.New("warehouse transport is required")
	ErrInvalidItem       = errors.New("warehouse item must have a positive id and count")
	ErrEmptyItems        = errors.New("warehouse item list must not be empty")
)

// GameTransport is the smallest game RPC surface required by this domain.
// session.Session and transport.Client both satisfy it, while tests can use a
// deterministic fake without opening a WebSocket.
type GameTransport interface {
	SendMsg(context.Context, transport.Command, proto.Message) (proto.Message, error)
}

// StatusCallbacks keeps status mutation outside the domain algorithm. A
// Runtime can wire these callbacks to StatusState; tests may leave them nil.
type StatusCallbacks struct {
	OnGold      func(int64)
	OnOperation func(string, float64)
	CurrentGold func() int64
}

// Config contains all account-local collaborators for a warehouse service.
// ConfigRepo is optional at runtime but remains an explicit seam for reading
// account automation settings in the composition root.
type Config struct {
	Transport     GameTransport
	ConfigRepo    store.ConfigRepo
	AccountID     string
	AutomationOn  func(context.Context, string) (bool, error)
	IsFruit       func(int64) bool
	Status        StatusCallbacks
	SellBatchSize int
	BatchDelay    time.Duration
}

// Item is the stable domain representation of one inventory entry.
type Item struct {
	ID    int64
	Count int64
	UID   int64
}

// Bag is a snapshot of the account inventory. Items are copied from the
// protobuf response so callers cannot mutate protocol-owned slices.
type Bag struct {
	Items []Item
}

type SellResult struct {
	Sold     []Item
	Rewards  []Item
	GoldGain int64
	Raw      *pb.SellReply
}

type UseResult struct {
	Items []Item
	Raw   *pb.UseReply
}

type BatchUseResult struct {
	UsedItems []Item
	Items     []Item
	Raw       *pb.BatchUseReply
}

type SellAllResult struct {
	Sold       []Item
	Skipped    []Item
	GoldGain   int64
	Automation bool
}

type ContainerHoursResult struct {
	Normal  float64
	Organic float64
}

// Service is the warehouse contract consumed by later domains.
type Service interface {
	ListBag(context.Context) (Bag, error)
	SellItem(context.Context, Item) (SellResult, error)
	SellItems(context.Context, []Item) (SellResult, error)
	SellAll(context.Context) (SellAllResult, error)
	UseItem(context.Context, int64, int64, []int64) (UseResult, error)
	BatchUse(context.Context, []UseEntry) (BatchUseResult, error)
}

// UseEntry describes one item in ItemService.BatchUse.
type UseEntry struct {
	ItemID int64
	Count  int64
	UID    int64
}

type service struct {
	transport    GameTransport
	config       store.ConfigRepo
	accountID    string
	automationOn func(context.Context, string) (bool, error)
	isFruit      func(int64) bool
	status       StatusCallbacks
	sellBatch    int
	batchDelay   time.Duration
}

var _ Service = (*service)(nil)

// New creates an account-local warehouse service.
func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	sellBatch := cfg.SellBatchSize
	if sellBatch <= 0 {
		sellBatch = defaultSellBatch
	}
	if cfg.BatchDelay < 0 {
		cfg.BatchDelay = 0
	}
	return &service{
		transport:    cfg.Transport,
		config:       cfg.ConfigRepo,
		accountID:    strings.TrimSpace(cfg.AccountID),
		automationOn: cfg.AutomationOn,
		isFruit:      cfg.IsFruit,
		status:       cfg.Status,
		sellBatch:    sellBatch,
		batchDelay:   cfg.BatchDelay,
	}, nil
}

// NewService is a descriptive constructor alias.
func NewService(cfg Config) (Service, error) { return New(cfg) }

func (s *service) ListBag(ctx context.Context) (Bag, error) {
	if err := checkContext(ctx); err != nil {
		return Bag{}, err
	}
	reply := new(pb.BagReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: itemService,
		MethodName:  "Bag",
		Response:    reply,
	}, &pb.BagRequest{})
	if err != nil {
		return Bag{}, fmt.Errorf("list bag: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.BagReply)
		if !ok {
			return Bag{}, fmt.Errorf("list bag returned %T, want *pb.BagReply", response)
		}
	}
	return Bag{Items: cloneItemsFromProto(bagItems(reply))}, nil
}

func (s *service) SellItem(ctx context.Context, item Item) (SellResult, error) {
	return s.SellItems(ctx, []Item{item})
}

func (s *service) SellItems(ctx context.Context, items []Item) (SellResult, error) {
	if err := checkContext(ctx); err != nil {
		return SellResult{}, err
	}
	protobufItems, err := toProtoItems(items)
	if err != nil {
		return SellResult{}, err
	}
	reply := new(pb.SellReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: itemService,
		MethodName:  "Sell",
		Response:    reply,
	}, &pb.SellRequest{Items: protobufItems})
	if err != nil {
		return SellResult{}, fmt.Errorf("sell items: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.SellReply)
		if !ok {
			return SellResult{}, fmt.Errorf("sell returned %T, want *pb.SellReply", response)
		}
	}
	return sellResult(reply), nil
}

// UseItem uses one inventory item, optionally targeting land IDs.
func (s *service) UseItem(ctx context.Context, itemID, count int64, landIDs []int64) (UseResult, error) {
	if err := checkContext(ctx); err != nil {
		return UseResult{}, err
	}
	if itemID <= 0 || count <= 0 {
		return UseResult{}, ErrInvalidItem
	}
	ids := append([]int64(nil), landIDs...)
	reply := new(pb.UseReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: itemService,
		MethodName:  "Use",
		Response:    reply,
	}, &pb.UseRequest{ItemId: itemID, Count: count, LandIds: ids})
	if err != nil {
		return UseResult{}, fmt.Errorf("use item %d: %w", itemID, err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.UseReply)
		if !ok {
			return UseResult{}, fmt.Errorf("use item returned %T, want *pb.UseReply", response)
		}
	}
	return UseResult{Items: cloneItemsFromProto(reply.GetItems()), Raw: reply}, nil
}

func (s *service) BatchUse(ctx context.Context, entries []UseEntry) (BatchUseResult, error) {
	if err := checkContext(ctx); err != nil {
		return BatchUseResult{}, err
	}
	if len(entries) == 0 {
		return BatchUseResult{}, ErrEmptyItems
	}
	items := make([]*pb.Item, 0, len(entries))
	for _, entry := range entries {
		if entry.ItemID <= 0 || entry.Count <= 0 {
			return BatchUseResult{}, ErrInvalidItem
		}
		items = append(items, &pb.Item{Id: entry.ItemID, Count: entry.Count, Uid: entry.UID})
	}
	reply := new(pb.BatchUseReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: itemService,
		MethodName:  "BatchUse",
		Response:    reply,
	}, &pb.BatchUseRequest{Items: items})
	if err != nil {
		return BatchUseResult{}, fmt.Errorf("batch use items: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.BatchUseReply)
		if !ok {
			return BatchUseResult{}, fmt.Errorf("batch use returned %T, want *pb.BatchUseReply", response)
		}
	}
	return BatchUseResult{
		UsedItems: cloneItemsFromProto(reply.GetUsedItems()),
		Items:     cloneItemsFromProto(reply.GetItems()),
		Raw:       reply,
	}, nil
}

// SellAll sells fruit entries in batches, retrying a failed batch item by
// item. Item 41221 is excluded from automation because the game rejects it
// even though it may be marked sellable in stale item metadata.
func (s *service) SellAll(ctx context.Context) (SellAllResult, error) {
	result := SellAllResult{}
	enabled, err := s.automationEnabled(ctx, "sell")
	if err != nil {
		return result, err
	}
	result.Automation = enabled
	if !enabled {
		return result, nil
	}
	bag, err := s.ListBag(ctx)
	if err != nil {
		return result, err
	}
	fruits := make([]Item, 0)
	for _, item := range bag.Items {
		if item.ID == 41221 || item.Count <= 0 || !s.isFruitItem(item.ID) {
			continue
		}
		fruits = append(fruits, item)
	}
	if len(fruits) == 0 {
		return result, nil
	}

	for start := 0; start < len(fruits); start += s.sellBatch {
		end := start + s.sellBatch
		if end > len(fruits) {
			end = len(fruits)
		}
		batch := fruits[start:end]
		sold, sellErr := s.SellItems(ctx, batch)
		if sellErr == nil {
			result.Sold = append(result.Sold, batch...)
			result.GoldGain += sold.GoldGain
		} else {
			for _, item := range batch {
				one, oneErr := s.SellItem(ctx, item)
				if oneErr != nil {
					result.Skipped = append(result.Skipped, item)
					continue
				}
				result.Sold = append(result.Sold, item)
				result.GoldGain += one.GoldGain
			}
		}
		if end < len(fruits) && s.batchDelay > 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(s.batchDelay):
			}
		}
	}
	if result.GoldGain > 0 && s.status.OnGold != nil {
		current := int64(0)
		if s.status.CurrentGold != nil {
			current = s.status.CurrentGold()
		}
		s.status.OnGold(current + result.GoldGain)
	}
	if s.status.OnOperation != nil && len(result.Sold) > 0 {
		s.status.OnOperation("sell", float64(totalItemCount(result.Sold)))
	}
	return result, nil
}

// SellAllFruits is a compatibility name for callers migrating from the Node
// service. It intentionally returns the structured result used by Go callers.
func (s *service) SellAllFruits(ctx context.Context) (SellAllResult, error) {
	return s.SellAll(ctx)
}

func (s *service) isFruitItem(id int64) bool {
	if s.isFruit != nil {
		return s.isFruit(id)
	}
	// Until the embedded game-config package lands, retain the Node ID family
	// convention as a conservative default. Production wiring should inject
	// the authoritative Plant.json classifier.
	return id >= 40000 && id < 2000000
}

func (s *service) automationEnabled(ctx context.Context, key string) (bool, error) {
	if s.automationOn != nil {
		return s.automationOn(ctx, key)
	}
	if s.config != nil && s.accountID != "" {
		raw, err := s.config.GetGlobal(ctx, "account:"+s.accountID+":automation")
		if err == nil {
			var values map[string]bool
			if json.Unmarshal(raw, &values) == nil {
				if enabled, ok := values[key]; ok {
					return enabled, nil
				}
			}
		}
	}
	return true, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func bagItems(reply *pb.BagReply) []*pb.Item {
	if reply == nil || reply.GetItemBag() == nil {
		return nil
	}
	return reply.GetItemBag().GetItems()
}

func sellResult(reply *pb.SellReply) SellResult {
	if reply == nil {
		return SellResult{}
	}
	rewards := cloneItemsFromProto(reply.GetGetItems())
	return SellResult{
		Sold:     cloneItemsFromProto(reply.GetSellItems()),
		Rewards:  rewards,
		GoldGain: goldFromItems(rewards),
		Raw:      reply,
	}
}

func toProtoItems(items []Item) ([]*pb.Item, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}
	result := make([]*pb.Item, 0, len(items))
	for _, item := range items {
		if item.ID <= 0 || item.Count <= 0 {
			return nil, ErrInvalidItem
		}
		result = append(result, &pb.Item{Id: item.ID, Count: item.Count, Uid: item.UID})
	}
	return result, nil
}

func cloneItemsFromProto(items []*pb.Item) []Item {
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, Item{ID: item.GetId(), Count: item.GetCount(), UID: item.GetUid()})
	}
	return result
}

func goldFromItems(items []Item) int64 {
	var total int64
	for _, item := range items {
		if item.ID == 1001 || item.ID == 500001 {
			total += item.Count
		}
	}
	return total
}

func totalItemCount(items []Item) int64 {
	var total int64
	for _, item := range items {
		if item.Count > 0 {
			total += item.Count
		}
	}
	return total
}

// ContainerHours reports the normal and organic fertilizer container values
// encoded as seconds in the inventory.
func ContainerHours(items []Item) ContainerHoursResult {
	result := ContainerHoursResult{}
	for _, item := range items {
		switch item.ID {
		case normalContainerID:
			result.Normal = float64(item.Count) / 3600
		case organicContainerID:
			result.Organic = float64(item.Count) / 3600
		}
	}
	return result
}

// MergeItems combines duplicate item IDs while retaining deterministic order.
// UID is retained only when all merged entries refer to the same UID.
func MergeItems(items []Item) []Item {
	merged := make(map[int64]Item, len(items))
	for _, item := range items {
		if item.ID <= 0 || item.Count <= 0 {
			continue
		}
		current, ok := merged[item.ID]
		if !ok {
			merged[item.ID] = item
			continue
		}
		current.Count += item.Count
		if current.UID != item.UID {
			current.UID = 0
		}
		merged[item.ID] = current
	}
	result := make([]Item, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
