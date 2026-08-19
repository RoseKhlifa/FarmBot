package farm

import (
	"context"
	"errors"
	"fmt"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
)

const (
	plantService = "gamepb.plantpb.PlantService"
	shopService  = "gamepb.shoppb.ShopService"

	NormalFertilizerID  int64 = 1011
	OrganicFertilizerID int64 = 1012
	SeedShopID          int64 = 2
)

var (
	ErrTransportRequired = errors.New("farm transport is required")
	ErrInvalidLandID     = errors.New("farm land id must be positive")
	ErrInvalidSeedID     = errors.New("farm seed id must be positive")
	ErrInvalidShopID     = errors.New("farm shop id must be positive")
)

// GameTransport is the protocol boundary consumed by the farm domain.
// transport.Client satisfies it, while tests can provide a deterministic fake.
type GameTransport interface {
	SendMsg(context.Context, transport.Command, proto.Message) (proto.Message, error)
}

// API is the raw RPC surface used by the farm algorithms. It intentionally
// contains no automation policy or account-global state.
type API interface {
	GetAllLands(context.Context) (*pb.AllLandsReply, error)
	Harvest(context.Context, []int64) (*pb.HarvestReply, error)
	WaterLand(context.Context, []int64) (*pb.WaterLandReply, error)
	Farming(context.Context, []int64) (*pb.FarmingReply, error)
	WeedOut(context.Context, []int64) (*pb.WeedOutReply, error)
	Insecticide(context.Context, []int64) (*pb.InsecticideReply, error)
	Fertilize(context.Context, []int64, int64) (*pb.FertilizeReply, error)
	RemovePlant(context.Context, []int64) (*pb.RemovePlantReply, error)
	Plant(context.Context, int64, []int64, bool) (*pb.PlantReply, error)
	UpgradeLand(context.Context, int64) (*pb.UpgradeLandReply, error)
	UnlockLand(context.Context, int64, bool) (*pb.UnlockLandReply, error)
	GetShopProfiles(context.Context) (*pb.ShopProfilesReply, error)
	GetShopInfo(context.Context, int64) (*pb.ShopInfoReply, error)
	BuyGoods(context.Context, int64, int64, int64) (*pb.BuyGoodsReply, error)
}

// APIConfig configures one account-local RPC adapter.
type APIConfig struct {
	Transport GameTransport
	HostGID   int64

	// OnOperationLimits forwards gateway operation-limit updates to the
	// account/runtime status layer without importing that layer here.
	OnOperationLimits func([]*pb.OperationLimit)
}

// RPCAPI implements API on top of the generated protobuf transport.
type RPCAPI struct {
	transport        GameTransport
	hostGID          int64
	onOperationLimit func([]*pb.OperationLimit)
}

var _ API = (*RPCAPI)(nil)

// NewAPI creates an account-local farm RPC adapter.
func NewAPI(cfg APIConfig) (*RPCAPI, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	return &RPCAPI{
		transport:        cfg.Transport,
		hostGID:          cfg.HostGID,
		onOperationLimit: cfg.OnOperationLimits,
	}, nil
}

// NewRPCAPI is a descriptive constructor alias.
func NewRPCAPI(cfg APIConfig) (*RPCAPI, error) { return NewAPI(cfg) }

func (a *RPCAPI) GetAllLands(ctx context.Context) (*pb.AllLandsReply, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.AllLandsReply)
	response, err := a.send(ctx, "AllLands", reply, &pb.AllLandsRequest{HostGid: a.hostGID})
	if err != nil {
		return nil, fmt.Errorf("get all lands: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("get all lands: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) Harvest(ctx context.Context, landIDs []int64) (*pb.HarvestReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.HarvestReply)
	response, err := a.send(ctx, "Harvest", reply, &pb.HarvestRequest{
		LandIds: ids,
		HostGid: a.hostGID,
		IsAll:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("harvest: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("harvest: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) WaterLand(ctx context.Context, landIDs []int64) (*pb.WaterLandReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.WaterLandReply)
	response, err := a.send(ctx, "WaterLand", reply, &pb.WaterLandRequest{LandIds: ids, HostGid: a.hostGID})
	if err != nil {
		return nil, fmt.Errorf("water land: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("water land: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) Farming(ctx context.Context, landIDs []int64) (*pb.FarmingReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.FarmingReply)
	response, err := a.send(ctx, "Farming", reply, &pb.FarmingRequest{LandIds: ids, HostGid: a.hostGID})
	if err != nil {
		return nil, fmt.Errorf("farming: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("farming: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) WeedOut(ctx context.Context, landIDs []int64) (*pb.WeedOutReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.WeedOutReply)
	response, err := a.send(ctx, "WeedOut", reply, &pb.WeedOutRequest{LandIds: ids, HostGid: a.hostGID})
	if err != nil {
		return nil, fmt.Errorf("weed out: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("weed out: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) Insecticide(ctx context.Context, landIDs []int64) (*pb.InsecticideReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.InsecticideReply)
	response, err := a.send(ctx, "Insecticide", reply, &pb.InsecticideRequest{LandIds: ids, HostGid: a.hostGID})
	if err != nil {
		return nil, fmt.Errorf("insecticide: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("insecticide: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) Fertilize(ctx context.Context, landIDs []int64, fertilizerID int64) (*pb.FertilizeReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	if fertilizerID <= 0 {
		return nil, fmt.Errorf("%w: fertilizer", ErrInvalidLandID)
	}
	reply := new(pb.FertilizeReply)
	response, err := a.send(ctx, "Fertilize", reply, &pb.FertilizeRequest{LandIds: ids, FertilizerId: fertilizerID})
	if err != nil {
		return nil, fmt.Errorf("fertilize: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("fertilize: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) RemovePlant(ctx context.Context, landIDs []int64) (*pb.RemovePlantReply, error) {
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.RemovePlantReply)
	response, err := a.send(ctx, "RemovePlant", reply, &pb.RemovePlantRequest{LandIds: ids})
	if err != nil {
		return nil, fmt.Errorf("remove plant: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("remove plant: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) Plant(ctx context.Context, seedID int64, landIDs []int64, autoSlave bool) (*pb.PlantReply, error) {
	if seedID <= 0 {
		return nil, ErrInvalidSeedID
	}
	ids, err := cleanIDs(landIDs)
	if err != nil {
		return nil, err
	}
	reply := new(pb.PlantReply)
	response, err := a.send(ctx, "Plant", reply, &pb.PlantRequest{Items: []*pb.PlantItem{{SeedId: seedID, LandIds: ids, AutoSlave: autoSlave}}})
	if err != nil {
		return nil, fmt.Errorf("plant: %w", err)
	}
	result, err := responseAs(response, reply)
	if err != nil {
		return nil, fmt.Errorf("plant: %w", err)
	}
	a.notifyLimits(result.GetOperationLimits())
	return result, nil
}

func (a *RPCAPI) UpgradeLand(ctx context.Context, landID int64) (*pb.UpgradeLandReply, error) {
	if landID <= 0 {
		return nil, ErrInvalidLandID
	}
	reply := new(pb.UpgradeLandReply)
	response, err := a.send(ctx, "UpgradeLand", reply, &pb.UpgradeLandRequest{LandId: landID})
	if err != nil {
		return nil, fmt.Errorf("upgrade land %d: %w", landID, err)
	}
	return responseAs(response, reply)
}

func (a *RPCAPI) UnlockLand(ctx context.Context, landID int64, shared bool) (*pb.UnlockLandReply, error) {
	if landID <= 0 {
		return nil, ErrInvalidLandID
	}
	reply := new(pb.UnlockLandReply)
	response, err := a.send(ctx, "UnlockLand", reply, &pb.UnlockLandRequest{LandId: landID, DoShared: shared})
	if err != nil {
		return nil, fmt.Errorf("unlock land %d: %w", landID, err)
	}
	return responseAs(response, reply)
}

func (a *RPCAPI) GetShopProfiles(ctx context.Context) (*pb.ShopProfilesReply, error) {
	reply := new(pb.ShopProfilesReply)
	response, err := a.sendShop(ctx, "ShopProfiles", reply, &pb.ShopProfilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("get shop profiles: %w", err)
	}
	return responseAs(response, reply)
}

func (a *RPCAPI) GetShopInfo(ctx context.Context, shopID int64) (*pb.ShopInfoReply, error) {
	if shopID <= 0 {
		return nil, ErrInvalidShopID
	}
	reply := new(pb.ShopInfoReply)
	response, err := a.sendShop(ctx, "ShopInfo", reply, &pb.ShopInfoRequest{ShopId: shopID})
	if err != nil {
		return nil, fmt.Errorf("get shop info %d: %w", shopID, err)
	}
	return responseAs(response, reply)
}

func (a *RPCAPI) BuyGoods(ctx context.Context, goodsID, count, price int64) (*pb.BuyGoodsReply, error) {
	if goodsID <= 0 || count <= 0 || price < 0 {
		return nil, fmt.Errorf("invalid goods purchase: goods=%d count=%d price=%d", goodsID, count, price)
	}
	reply := new(pb.BuyGoodsReply)
	response, err := a.sendShop(ctx, "BuyGoods", reply, &pb.BuyGoodsRequest{GoodsId: goodsID, Num: count, Price: price})
	if err != nil {
		return nil, fmt.Errorf("buy goods %d: %w", goodsID, err)
	}
	return responseAs(response, reply)
}

func (a *RPCAPI) send(ctx context.Context, method string, response, request proto.Message) (proto.Message, error) {
	return a.sendService(ctx, plantService, method, response, request)
}

func (a *RPCAPI) sendShop(ctx context.Context, method string, response, request proto.Message) (proto.Message, error) {
	return a.sendService(ctx, shopService, method, response, request)
}

func (a *RPCAPI) sendService(ctx context.Context, service, method string, response, request proto.Message) (proto.Message, error) {
	if a == nil || a.transport == nil {
		return nil, ErrTransportRequired
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return a.transport.SendMsg(ctx, transport.Command{
		ServiceName: service,
		MethodName:  method,
		Response:    response,
	}, request)
}

func (a *RPCAPI) notifyLimits(limits []*pb.OperationLimit) {
	if len(limits) > 0 && a != nil && a.onOperationLimit != nil {
		a.onOperationLimit(limits)
	}
}

func responseAs[T proto.Message](response proto.Message, fallback T) (T, error) {
	if response == nil {
		return fallback, nil
	}
	result, ok := response.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("transport returned %T, want %T", response, fallback)
	}
	return result, nil
}

func cleanIDs(ids []int64) ([]int64, error) {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidLandID
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, ErrInvalidLandID
	}
	return result, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
