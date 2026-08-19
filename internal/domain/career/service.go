// Package career provides the account-scoped career statistics RPC service.
package career

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

const careerService = "gamepb.careerpb.CareerService"

var ErrTransportRequired = errors.New("career transport is required")

// GameTransport reuses the account-local protocol boundary from warehouse.
type GameTransport = warehouse.GameTransport

// ItemInfo contains optional game-config data used to decorate a statistic.
type ItemInfo struct {
	Name   string
	Image  string
	Level  int32
	Rarity int32
}

// Config contains account-local collaborators for a career service.
type Config struct {
	Transport  GameTransport
	LookupItem func(int64) (ItemInfo, bool)
	ItemName   func(int64) string
	ItemImage  func(int64) string
}

// Stat is a decorated career statistic.
type Stat struct {
	ID     int64
	Count  int64
	Name   string
	Image  string
	Level  int32
	Rarity int32
}

// LevelStat is a decorated level-unlock statistic.
type LevelStat struct {
	ID    int64
	Count int64
	Level int32
	Name  string
	Image string
}

type Player struct {
	GID    int64
	Name   string
	Avatar string
	OpenID string
	Level  int32
	Exp    int64
}

type Meta struct {
	AchievedLevels int32
	StatsTotal     int64
	StatsCount     int64
}

// Info is the stable domain result. Raw retains the generated response for
// callers that need fields not yet promoted into the domain contract.
type Info struct {
	Items      []Stat
	LevelStats []LevelStat
	Player     Player
	Meta       Meta
	Raw        *pb.CareerInfoGetReply
}

// Service is the career domain contract.
type Service interface {
	GetCareerInfo(context.Context) (Info, error)
}

type service struct {
	transport  GameTransport
	lookupItem func(int64) (ItemInfo, bool)
	itemName   func(int64) string
	itemImage  func(int64) string
}

var _ Service = (*service)(nil)

// New constructs an account-local career service.
func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	return &service{
		transport:  cfg.Transport,
		lookupItem: cfg.LookupItem,
		itemName:   cfg.ItemName,
		itemImage:  cfg.ItemImage,
	}, nil
}

// NewService is an explicit constructor alias used by composition roots.
func NewService(cfg Config) (Service, error) { return New(cfg) }

func (s *service) GetCareerInfo(ctx context.Context) (Info, error) {
	if err := checkContext(ctx); err != nil {
		return Info{}, err
	}
	reply := new(pb.CareerInfoGetReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: careerService,
		MethodName:  "CareerInfoGet",
		Response:    reply,
	}, &pb.CareerInfoGetRequest{})
	if err != nil {
		return Info{}, fmt.Errorf("get career info: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.CareerInfoGetReply)
		if !ok {
			return Info{}, fmt.Errorf("career info returned %T, want *pb.CareerInfoGetReply", response)
		}
	}
	return s.toInfo(reply), nil
}

func (s *service) toInfo(reply *pb.CareerInfoGetReply) Info {
	if reply == nil {
		return Info{}
	}
	result := Info{
		Items:      make([]Stat, 0, len(reply.GetItems())),
		LevelStats: make([]LevelStat, 0, len(reply.GetLevelStats())),
		Player: Player{
			GID: reply.GetGid(), Name: reply.GetName(), Avatar: reply.GetAvatar(),
			OpenID: reply.GetOpenid(), Level: reply.GetLevel(), Exp: reply.GetExp(),
		},
		Meta: Meta{
			AchievedLevels: reply.GetAchievedLevels(),
			StatsTotal:     reply.GetStatsTotal(), StatsCount: reply.GetStatsCount(),
		},
		Raw: reply,
	}
	for _, item := range reply.GetItems() {
		if item == nil || item.GetFruitId() <= 0 || item.GetCount() <= 0 {
			continue
		}
		result.Items = append(result.Items, s.decorate(item.GetFruitId(), item.GetCount(), 0))
	}
	for _, item := range reply.GetLevelStats() {
		if item == nil || item.GetFruitId() <= 0 {
			continue
		}
		decorated := s.decorate(item.GetFruitId(), item.GetCount(), item.GetLevel())
		result.LevelStats = append(result.LevelStats, LevelStat{
			ID: decorated.ID, Count: decorated.Count, Level: decorated.Level,
			Name: decorated.Name, Image: decorated.Image,
		})
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].Count > result.Items[j].Count })
	return result
}

func (s *service) decorate(id, count int64, level int32) Stat {
	info, ok := ItemInfo{}, false
	if s.lookupItem != nil {
		info, ok = s.lookupItem(id)
	}
	name := strings.TrimSpace(info.Name)
	if !ok || name == "" {
		if s.itemName != nil {
			name = strings.TrimSpace(s.itemName(id))
		}
	}
	if name == "" {
		name = fmt.Sprintf("物品 %d", id)
	}
	image := strings.TrimSpace(info.Image)
	if image == "" && s.itemImage != nil {
		image = strings.TrimSpace(s.itemImage(id))
	}
	if level == 0 {
		level = info.Level
	}
	return Stat{ID: id, Count: count, Name: name, Image: image, Level: level, Rarity: info.Rarity}
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
