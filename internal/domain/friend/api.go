package friend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"google.golang.org/protobuf/proto"
)

type Platform string

const (
	PlatformQQ     Platform = "qq"
	PlatformWechat Platform = "wechat"
)

type Friend struct {
	GID              int64     `json:"gid"`
	OpenID           string    `json:"open_id,omitempty"`
	Name             string    `json:"name,omitempty"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	Remark           string    `json:"remark,omitempty"`
	Level            int64     `json:"level,omitempty"`
	Gold             int64     `json:"gold,omitempty"`
	Plant            *pb.Plant `json:"plant,omitempty"`
	AuthorizedStatus int32     `json:"authorized_status,omitempty"`
}

type Application struct {
	GID                     int64 `json:"gid"`
	TimeAt                  int64 `json:"time_at,omitempty"`
	OpenID, Name, AvatarURL string
	Level                   int64 `json:"level,omitempty"`
}

type APIConfig struct {
	Transport  GameTransport
	AccountID  string
	MyGID      int64
	Platform   Platform
	Cache      store.CacheRepo
	Now        func() time.Time
	QuietHours string
	BatchSize  int
}

type API struct {
	transport GameTransport
	accountID string
	myGID     int64
	platform  Platform
	cache     store.CacheRepo
	now       func() time.Time
	quiet     QuietHours
	batchSize int
}

func NewAPI(cfg APIConfig) (*API, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Platform == "" {
		cfg.Platform = PlatformQQ
	}
	quiet, err := ParseQuietHours(cfg.QuietHours)
	if err != nil {
		return nil, err
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 35
	}
	return &API{transport: cfg.Transport, accountID: accountText(cfg.AccountID), myGID: cfg.MyGID, platform: cfg.Platform, cache: cfg.Cache, now: cfg.Now, quiet: quiet, batchSize: cfg.BatchSize}, nil
}

func NormalizeGIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func DedupeFriends(values []*pb.GameFriend) []Friend {
	result := make([]Friend, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value == nil || value.GetGid() <= 0 {
			continue
		}
		if _, ok := seen[value.GetGid()]; ok {
			continue
		}
		seen[value.GetGid()] = struct{}{}
		result = append(result, friendFromProto(value))
	}
	return result
}

func friendFromProto(value *pb.GameFriend) Friend {
	return Friend{GID: value.GetGid(), OpenID: value.GetOpenId(), Name: value.GetName(), AvatarURL: value.GetAvatarUrl(), Remark: value.GetRemark(), Level: value.GetLevel(), Gold: value.GetGold(), Plant: value.GetPlant(), AuthorizedStatus: value.GetAuthorizedStatus()}
}

func (a *API) GetAllFriends(ctx context.Context) ([]Friend, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if a != nil && a.cache != nil && a.accountID != "" {
		if cached, err := a.readFriends(ctx); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}
	var friends []*pb.GameFriend
	if a.platform == PlatformWechat {
		reply, err := a.callFriend(ctx, "GetAll", &pb.GetAllReply{}, &pb.GetAllRequest{})
		if err != nil {
			return nil, err
		}
		if typed, ok := reply.(*pb.GetAllReply); ok && typed != nil {
			friends = typed.GetGameFriends()
		}
	} else {
		gids, _ := a.knownGIDs(ctx)
		if len(gids) > 0 {
			for start := 0; start < len(gids); start += a.batchSize {
				end := start + a.batchSize
				if end > len(gids) {
					end = len(gids)
				}
				reply, err := a.callFriend(ctx, "GetGameFriends", &pb.GetAllReply{}, &pb.GetGameFriendsRequest{Gids: gids[start:end]})
				if err != nil {
					friends = nil
					break
				}
				if typed, ok := reply.(*pb.GetAllReply); ok && typed != nil {
					friends = append(friends, typed.GetGameFriends()...)
				}
			}
		}
		if len(friends) == 0 {
			reply, err := a.callFriend(ctx, "SyncAll", &pb.SyncAllReply{}, &pb.SyncAllRequest{})
			if err == nil {
				if typed, ok := reply.(*pb.SyncAllReply); ok && typed != nil {
					friends = typed.GetGameFriends()
				}
			}
			if err != nil || len(friends) == 0 {
				reply2, err2 := a.callFriend(ctx, "GetAll", &pb.GetAllReply{}, &pb.GetAllRequest{})
				if err2 != nil {
					if err != nil {
						return nil, err
					}
					return nil, err2
				}
				if typed, ok := reply2.(*pb.GetAllReply); ok && typed != nil {
					friends = typed.GetGameFriends()
				}
			}
		}
	}
	result := DedupeFriends(friends)
	if a != nil && a.cache != nil && a.accountID != "" {
		_ = a.writeFriends(ctx, result)
		_ = a.writeKnownGIDs(ctx, result)
	}
	return result, nil
}

// GetFriendList is the descriptive name used by composition roots migrating
// from the Node friend facade.
func (a *API) GetFriendList(ctx context.Context) ([]Friend, error) { return a.GetAllFriends(ctx) }
func (a *API) KnownGIDs(ctx context.Context) ([]int64, error)      { return a.knownGIDs(ctx) }
func (a *API) IsInQuietHours(now ...time.Time) bool                { return a.InQuietHours(now...) }
func (a *API) AddToBlacklist(ctx context.Context, gid int64, reason string) error {
	return a.AddBlacklist(ctx, gid, reason)
}
func (a *API) RemoveFromBlacklist(ctx context.Context, gid int64) error {
	return a.RemoveBlacklist(ctx, gid)
}

func (a *API) GetApplications(ctx context.Context) ([]Application, bool, error) {
	reply, err := a.callFriend(ctx, "GetApplications", &pb.GetApplicationsReply{}, &pb.GetApplicationsRequest{})
	if err != nil {
		return nil, false, err
	}
	p := reply.(*pb.GetApplicationsReply)
	out := make([]Application, 0, len(p.GetApplications()))
	for _, v := range p.GetApplications() {
		if v != nil && v.GetGid() > 0 {
			out = append(out, Application{GID: v.GetGid(), TimeAt: v.GetTimeAt(), OpenID: v.GetOpenId(), Name: v.GetName(), AvatarURL: v.GetAvatarUrl(), Level: v.GetLevel()})
		}
	}
	return out, p.GetBlockApplications(), nil
}
func (a *API) AcceptFriends(ctx context.Context, gids []int64) ([]Friend, error) {
	p, err := a.callFriend(ctx, "AcceptFriends", &pb.AcceptFriendsReply{}, &pb.AcceptFriendsRequest{FriendGids: NormalizeGIDs(gids)})
	if err != nil {
		return nil, err
	}
	return DedupeFriends(p.(*pb.AcceptFriendsReply).GetFriends()), nil
}
func (a *API) RejectFriends(ctx context.Context, gids []int64) error {
	_, err := a.callFriend(ctx, "RejectFriends", &pb.RejectFriendsReply{}, &pb.RejectFriendsRequest{FriendGids: NormalizeGIDs(gids)})
	return err
}
func (a *API) DeleteFriend(ctx context.Context, gid int64) error {
	if gid <= 0 {
		return ErrInvalidGID
	}
	_, err := a.callFriend(ctx, "DelFriend", &pb.DelFriendReply{}, &pb.DelFriendRequest{FriendGid: gid})
	return err
}
func (a *API) ApplyFriend(ctx context.Context, gid int64, token string) error {
	if gid <= 0 {
		return ErrInvalidGID
	}
	_, err := a.callFriend(ctx, "ApplyFriend", &pb.ApplyFriendReply{}, &pb.ApplyFriendRequest{Gid: gid, Token: token})
	return err
}
func (a *API) SetBlockApplications(ctx context.Context, block bool) error {
	_, err := a.callFriend(ctx, "SetBlockApplications", &pb.SetBlockApplicationsReply{}, &pb.SetBlockApplicationsRequest{Block: block})
	return err
}

func (a *API) EnterFriendFarm(ctx context.Context, gid int64) (*pb.EnterReply, error) {
	if gid <= 0 {
		return nil, ErrInvalidGID
	}
	reply := new(pb.EnterReply)
	response, err := a.transport.SendMsg(ctx, transport.Command{ServiceName: VisitServiceName, MethodName: "Enter", Response: reply}, &pb.EnterRequest{HostGid: gid, Reason: 2})
	if err != nil {
		if IsErrorCode(err, 1002003) {
			_ = a.AddBlacklist(ctx, gid, "enter banned")
		}
		if IsErrorCode(err, 1002002) {
			_ = a.RemoveKnownFriend(ctx, gid)
		}
		return nil, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.EnterReply)
		if !ok {
			return nil, fmt.Errorf("enter returned %T", response)
		}
	}
	return reply, nil
}
func (a *API) LeaveFriendFarm(ctx context.Context, gid int64) error {
	if gid <= 0 {
		return ErrInvalidGID
	}
	_, err := a.transport.SendMsg(ctx, transport.Command{ServiceName: VisitServiceName, MethodName: "Leave", Response: new(pb.LeaveReply)}, &pb.LeaveRequest{HostGid: gid})
	return err
}
func (a *API) CheckCanOperate(ctx context.Context, hostGID, operationID int64) (bool, error) {
	reply := new(pb.CheckCanOperateReply)
	response, err := a.transport.SendMsg(ctx, transport.Command{ServiceName: PlantServiceName, MethodName: "CheckCanOperate", Response: reply}, &pb.CheckCanOperateRequest{HostGid: hostGID, OperationId: operationID})
	if err != nil {
		return false, err
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.CheckCanOperateReply)
		if !ok {
			return false, fmt.Errorf("check operate returned %T", response)
		}
	}
	return reply.GetCanOperate(), nil
}

func (a *API) knownGIDs(ctx context.Context) ([]int64, error) {
	if a == nil || a.cache == nil || a.accountID == "" {
		return nil, ErrCacheRequired
	}
	value, err := a.cache.GetKnownFriendGIDs(ctx, a.accountID)
	if err != nil {
		return nil, err
	}
	var gids []int64
	if err = json.Unmarshal(value.Payload, &gids); err != nil {
		return nil, err
	}
	return NormalizeGIDs(gids), nil
}
func (a *API) writeKnownGIDs(ctx context.Context, friends []Friend) error {
	gids := make([]int64, 0, len(friends))
	for _, f := range friends {
		gids = append(gids, f.GID)
	}
	raw, _ := json.Marshal(gids)
	return a.cache.PutKnownFriendGIDs(ctx, a.accountID, store.CacheValue{Payload: raw, UpdatedAt: a.now().UnixMilli()})
}
func (a *API) readFriends(ctx context.Context) ([]Friend, error) {
	value, err := a.cache.GetFriendList(ctx, a.accountID)
	if err != nil {
		return nil, err
	}
	var friends []Friend
	err = json.Unmarshal(value.Payload, &friends)
	return friends, err
}
func (a *API) writeFriends(ctx context.Context, friends []Friend) error {
	raw, _ := json.Marshal(friends)
	return a.cache.PutFriendList(ctx, a.accountID, store.CacheValue{Payload: raw, UpdatedAt: a.now().UnixMilli()})
}
func (a *API) RemoveKnownFriend(ctx context.Context, gid int64) error {
	if a == nil || a.cache == nil {
		return ErrCacheRequired
	}
	if err := a.cache.RemoveFriendFromCache(ctx, a.accountID, strconv.FormatInt(gid, 10)); err != nil {
		return err
	}
	// CacheRepo deliberately exposes invalidation rather than a package-global
	// mutable list. Rewrite the known-GID snapshot after removing one friend.
	if value, err := a.cache.GetKnownFriendGIDs(ctx, a.accountID); err == nil {
		var gids []int64
		if json.Unmarshal(value.Payload, &gids) == nil {
			filtered := gids[:0]
			for _, known := range gids {
				if known != gid {
					filtered = append(filtered, known)
				}
			}
			raw, _ := json.Marshal(NormalizeGIDs(filtered))
			return a.cache.PutKnownFriendGIDs(ctx, a.accountID, store.CacheValue{Payload: raw, UpdatedAt: a.now().UnixMilli()})
		}
	}
	return nil
}
func (a *API) AddBlacklist(ctx context.Context, gid int64, reason string) error {
	if a == nil || a.cache == nil {
		return ErrCacheRequired
	}
	return a.cache.UpsertBlacklist(ctx, store.BlacklistEntry{AccountID: a.accountID, GID: strconv.FormatInt(gid, 10), Reason: reason, AddedAt: a.now().UnixMilli()})
}
func (a *API) RemoveBlacklist(ctx context.Context, gid int64) error {
	if a == nil || a.cache == nil {
		return ErrCacheRequired
	}
	return a.cache.DeleteBlacklist(ctx, a.accountID, strconv.FormatInt(gid, 10))
}
func (a *API) Blacklist(ctx context.Context) (map[int64]store.BlacklistEntry, error) {
	if a == nil || a.cache == nil {
		return nil, ErrCacheRequired
	}
	entries, err := a.cache.ListBlacklist(ctx, a.accountID)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]store.BlacklistEntry, len(entries))
	for _, e := range entries {
		gid, er := strconv.ParseInt(e.GID, 10, 64)
		if er == nil && gid > 0 {
			out[gid] = e
		}
	}
	return out, nil
}

func (a *API) callFriend(ctx context.Context, method string, response, request proto.Message) (proto.Message, error) {
	if a == nil || a.transport == nil {
		return nil, ErrTransportRequired
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return a.transport.SendMsg(ctx, transport.Command{ServiceName: FriendServiceName, MethodName: method, Response: response}, request)
}

type QuietWindow struct {
	Start, End    int
	CrossMidnight bool
}
type QuietHours []QuietWindow

func ParseQuietHours(value string) (QuietHours, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var result QuietHours
	for _, part := range strings.Split(value, ",") {
		bits := strings.Split(strings.TrimSpace(part), "-")
		if len(bits) != 2 {
			return nil, ErrQuietHours
		}
		start, err := parseClock(bits[0])
		if err != nil {
			return nil, ErrQuietHours
		}
		end, err := parseClock(bits[1])
		if err != nil {
			return nil, ErrQuietHours
		}
		result = append(result, QuietWindow{Start: start, End: end, CrossMidnight: start > end})
	}
	return result, nil
}
func parseClock(value string) (int, error) {
	bits := strings.Split(strings.TrimSpace(value), ":")
	if len(bits) != 2 {
		return 0, errors.New("clock")
	}
	h, e1 := strconv.Atoi(bits[0])
	m, e2 := strconv.Atoi(bits[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, errors.New("clock")
	}
	return h*60 + m, nil
}
func (q QuietHours) Contains(now time.Time) bool {
	minute := now.Hour()*60 + now.Minute()
	for _, w := range q {
		if w.Start == w.End || (!w.CrossMidnight && minute >= w.Start && minute < w.End) || (w.CrossMidnight && (minute >= w.Start || minute < w.End)) {
			return true
		}
	}
	return false
}
func (a *API) InQuietHours(now ...time.Time) bool {
	if a == nil {
		return false
	}
	clock := a.now()
	if len(now) > 0 {
		clock = now[0]
	}
	return a.quiet.Contains(clock)
}
func IsErrorCode(err error, code int64) bool {
	var gateway *transport.GatewayError
	if errors.As(err, &gateway) && gateway.Meta != nil {
		return gateway.Meta.GetErrorCode() == code
	}
	return strings.Contains(errString(err), fmt.Sprintf("code=%d", code))
}
func ParseRPCErrorCode(err error) int64 {
	var gateway *transport.GatewayError
	if errors.As(err, &gateway) && gateway.Meta != nil {
		return gateway.Meta.GetErrorCode()
	}
	return 0
}
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
