// Package social groups the small share, invite, interaction and dog-gift
// services. They share one account-local game transport but remain thin RPC
// wrappers for callers that need only one capability.
package social

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	shareService  = "gamepb.sharepb.ShareService"
	userService   = "gamepb.userpb.UserService"
	dogService    = "gamepb.dogpb.DogService"
	shareDailyKey = "daily_share"
)

var (
	ErrTransportRequired = errors.New("social transport is required")
	ErrInvalidInvite     = errors.New("invite must include a positive uid and openid")
)

// GameTransport reuses the account-local protocol boundary from warehouse.
type GameTransport = warehouse.GameTransport

// Config contains account-local collaborators. No package-level network or
// daily state is used, so multiple accounts can run independently.
type Config struct {
	Transport          GameTransport
	CropName           func(int64) string
	Now                func() time.Time
	ShareCheckCooldown time.Duration
	InviteDelay        time.Duration
	ShareConfigID      string
	ShareSceneID       uint64
	HostGID            int64
}

type ShareDailyState struct {
	Key         string
	DoneToday   bool
	LastCheckAt time.Time
	LastClaimAt time.Time
}

type InviteCode struct {
	UID         int64
	OpenID      string
	ShareKey    string
	ShareSource string
	DocID       string
}

type InviteResult struct {
	OK      bool
	BodyLen int
}

type InviteProcessResult struct {
	Attempts int
	Success  int
	Failed   int
	Errors   []error
}

type InteractRecord struct {
	Key           string
	ServerTimeSec int64
	ServerTimeMS  int64
	ActionType    int32
	ActionLabel   string
	VisitorGID    int64
	Nick          string
	AvatarURL     string
	CropID        int64
	CropName      string
	CropCount     int32
	Times         int32
	FromType      int32
	Level         int32
	LandID        int32
	Flag1         int32
	Flag2         int32
	ActionDetail  string
}

type DogGiftStatus struct {
	OK        bool
	Claimable int64
	Raw       *pb.GetDogInfoReply
}

type DogGiftResult struct {
	OK      bool
	Claimed int64
	Raw     *pb.GetDogInfoReply
}

// Service is the aggregate social contract. Low-level methods return the
// generated protobuf replies; normalized helpers are provided where the Node
// implementation exposed a stable domain shape.
type Service interface {
	CheckCanShare(context.Context) (*pb.CheckCanShareReply, error)
	ReportShare(context.Context) (*pb.ReportShareReply, error)
	ClaimShareReward(context.Context) (*pb.ClaimShareRewardReply, error)
	PerformDailyShare(context.Context, bool) (bool, error)
	ShareDailyState() ShareDailyState

	SendReportArkClick(context.Context, InviteCode) (InviteResult, error)
	ProcessInviteCodes(context.Context, []InviteCode) (InviteProcessResult, error)

	GetInteractRecords(context.Context) ([]InteractRecord, error)
	GetDogGiftStatus(context.Context) (DogGiftStatus, error)
	ClaimDogGifts(context.Context) (DogGiftResult, error)
}

type service struct {
	transport     GameTransport
	cropName      func(int64) string
	now           func() time.Time
	shareCooldown time.Duration
	inviteDelay   time.Duration
	shareConfigID string
	shareSceneID  uint64
	hostGID       int64
	stateMu       sync.Mutex
	doneDateKey   string
	lastCheckAt   time.Time
	lastClaimAt   time.Time
}

var _ Service = (*service)(nil)

func New(cfg Config) (Service, error) {
	if cfg.Transport == nil {
		return nil, ErrTransportRequired
	}
	cooldown := cfg.ShareCheckCooldown
	if cooldown == 0 {
		cooldown = 10 * time.Minute
	}
	if cooldown < 0 {
		cooldown = 0
	}
	delay := cfg.InviteDelay
	if delay < 0 {
		delay = 0
	}
	configID := strings.TrimSpace(cfg.ShareConfigID)
	if configID == "" {
		configID = "1008"
	}
	sceneID := cfg.ShareSceneID
	if sceneID == 0 {
		sceneID = 7
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &service{
		transport: cfg.Transport, cropName: cfg.CropName, now: now,
		shareCooldown: cooldown, inviteDelay: delay,
		shareConfigID: configID, shareSceneID: sceneID, hostGID: cfg.HostGID,
	}, nil
}

func NewService(cfg Config) (Service, error) { return New(cfg) }

func (s *service) CheckCanShare(ctx context.Context) (*pb.CheckCanShareReply, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.CheckCanShareReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: shareService, MethodName: "CheckCanShare", Response: reply,
	}, &pb.CheckCanShareRequest{})
	if err != nil {
		return nil, fmt.Errorf("check share availability: %w", err)
	}
	return shareReply(response, reply)
}

func (s *service) ReportShare(ctx context.Context) (*pb.ReportShareReply, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.ReportShareReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: shareService, MethodName: "ReportShare", Response: reply,
	}, &pb.ReportShareRequest{Shared: true})
	if err != nil {
		return nil, fmt.Errorf("report share: %w", err)
	}
	return shareReply(response, reply)
}

func (s *service) ClaimShareReward(ctx context.Context) (*pb.ClaimShareRewardReply, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	reply := new(pb.ClaimShareRewardReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: shareService, MethodName: "ClaimShareReward", Response: reply,
	}, &pb.ClaimShareRewardRequest{Claimed: true})
	if err != nil {
		return nil, fmt.Errorf("claim share reward: %w", err)
	}
	return shareReply(response, reply)
}

func shareReply[T proto.Message](response proto.Message, fallback T) (T, error) {
	if response == nil {
		return fallback, nil
	}
	typed, ok := response.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("social RPC returned %T", response)
	}
	return typed, nil
}

func (s *service) PerformDailyShare(ctx context.Context, force bool) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	now := s.now()
	today := dateKey(now)
	s.stateMu.Lock()
	if !force && s.doneDateKey == today {
		s.stateMu.Unlock()
		return false, nil
	}
	if !force && !s.lastCheckAt.IsZero() && now.Sub(s.lastCheckAt) < s.shareCooldown {
		s.stateMu.Unlock()
		return false, nil
	}
	s.lastCheckAt = now
	s.stateMu.Unlock()

	canShare, err := s.CheckCanShare(ctx)
	if err != nil {
		return false, err
	}
	if !canShare.GetCanShare() {
		s.markShareDone(today)
		return false, nil
	}
	reported, err := s.ReportShare(ctx)
	if err != nil {
		return false, err
	}
	if !reported.GetSuccess() {
		return false, errors.New("report share was rejected")
	}
	claimed, err := s.ClaimShareReward(ctx)
	if err != nil {
		if isAlreadyClaimed(err) {
			s.markShareDone(today)
			return false, nil
		}
		return false, err
	}
	if !claimed.GetSuccess() {
		return false, errors.New("share reward was rejected")
	}
	s.stateMu.Lock()
	s.lastClaimAt = s.now()
	s.doneDateKey = today
	s.stateMu.Unlock()
	return true, nil
}

func (s *service) ShareDailyState() ShareDailyState {
	now := s.now()
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return ShareDailyState{Key: shareDailyKey, DoneToday: s.doneDateKey == dateKey(now), LastCheckAt: s.lastCheckAt, LastClaimAt: s.lastClaimAt}
}

func (s *service) SendReportArkClick(ctx context.Context, code InviteCode) (InviteResult, error) {
	if err := checkContext(ctx); err != nil {
		return InviteResult{}, err
	}
	if code.UID <= 0 || strings.TrimSpace(code.OpenID) == "" {
		return InviteResult{}, ErrInvalidInvite
	}
	request := &pb.ReportArkClickRequest{SharerId: code.UID, SharerOpenId: strings.TrimSpace(code.OpenID)}
	unknown := make([]byte, 0, len(code.ShareKey)+16)
	unknown = appendBytesField(unknown, 3, []byte(s.shareConfigID))
	unknown = appendVarintField(unknown, 4, s.shareSceneID)
	if strings.TrimSpace(code.ShareKey) != "" {
		unknown = appendBytesField(unknown, 5, []byte(strings.TrimSpace(code.ShareKey)))
	}
	request.ProtoReflect().SetUnknown(unknown)
	reply := new(pb.ReportArkClickReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: userService, MethodName: "ReportArkClick", Response: reply,
	}, request)
	if err != nil {
		return InviteResult{}, fmt.Errorf("send invite for uid %d: %w", code.UID, err)
	}
	if response != nil {
		if _, ok := response.(*pb.ReportArkClickReply); !ok {
			return InviteResult{}, fmt.Errorf("report ark click returned %T, want *pb.ReportArkClickReply", response)
		}
	}
	return InviteResult{OK: true}, nil
}

func (s *service) ProcessInviteCodes(ctx context.Context, codes []InviteCode) (InviteProcessResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result := InviteProcessResult{Errors: make([]error, 0)}
	for i, code := range codes {
		if err := checkContext(ctx); err != nil {
			return result, err
		}
		result.Attempts++
		if _, err := s.SendReportArkClick(ctx, code); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
		} else {
			result.Success++
		}
		if i < len(codes)-1 && s.inviteDelay > 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(s.inviteDelay):
			}
		}
	}
	return result, nil
}

func (s *service) GetInteractRecords(ctx context.Context) ([]InteractRecord, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	request := &pb.InteractRecordsRequest{}
	candidates := [][2]string{
		{"gamepb.interactpb.InteractService", "InteractRecords"},
		{"gamepb.interactpb.InteractService", "GetInteractRecords"},
		{"gamepb.interactpb.VisitorService", "InteractRecords"},
		{"gamepb.interactpb.VisitorService", "GetInteractRecords"},
	}
	var lastErr error
	for _, candidate := range candidates {
		reply := new(pb.InteractRecordsReply)
		response, err := s.transport.SendMsg(ctx, transport.Command{
			ServiceName: candidate[0], MethodName: candidate[1], Response: reply,
		}, request)
		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "code=") {
				break
			}
			continue
		}
		if response != nil {
			var ok bool
			reply, ok = response.(*pb.InteractRecordsReply)
			if !ok {
				return nil, fmt.Errorf("interact records returned %T, want *pb.InteractRecordsReply", response)
			}
		}
		return s.normalizeInteract(reply.GetRecords()), nil
	}
	if lastErr == nil {
		lastErr = errors.New("no interact records route succeeded")
	}
	return nil, fmt.Errorf("get interact records: %w", lastErr)
}

func (s *service) normalizeInteract(records []*pb.InteractRecord) []InteractRecord {
	result := make([]InteractRecord, 0, len(records))
	for index, raw := range records {
		if raw == nil {
			continue
		}
		seconds := toTimeSec(raw.GetServerTime())
		cropID := int64(raw.GetCropId())
		nick := strings.TrimSpace(raw.GetNick())
		if nick == "" {
			nick = fmt.Sprintf("GID:%d", raw.GetVisitorGid())
		}
		var landID, flag1, flag2 int32
		if extra := raw.GetExtra(); extra != nil {
			landID, flag1, flag2 = extra.GetLandId(), extra.GetFlag1(), extra.GetFlag2()
		}
		cropName := ""
		if s.cropName != nil && cropID > 0 {
			cropName = strings.TrimSpace(s.cropName(cropID))
		}
		record := InteractRecord{
			Key:           fmt.Sprintf("%d-%d-%d-%d", seconds, raw.GetVisitorGid(), raw.GetActionType(), index),
			ServerTimeSec: seconds, ServerTimeMS: seconds * 1000,
			ActionType: raw.GetActionType(), ActionLabel: actionLabel(raw.GetActionType()),
			VisitorGID: raw.GetVisitorGid(), Nick: nick, AvatarURL: strings.TrimSpace(raw.GetAvatarUrl()),
			CropID: cropID, CropName: cropName, CropCount: raw.GetCropCount(), Times: raw.GetTimes(),
			FromType: raw.GetFromType(), Level: raw.GetLevel(), LandID: landID, Flag1: flag1, Flag2: flag2,
		}
		record.ActionDetail = actionDetail(record)
		result = append(result, record)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ServerTimeSec != result[j].ServerTimeSec {
			return result[i].ServerTimeSec > result[j].ServerTimeSec
		}
		if result[i].VisitorGID != result[j].VisitorGID {
			return result[i].VisitorGID > result[j].VisitorGID
		}
		return result[i].ActionType > result[j].ActionType
	})
	return result
}

func (s *service) GetDogGiftStatus(ctx context.Context) (DogGiftStatus, error) {
	if err := checkContext(ctx); err != nil {
		return DogGiftStatus{}, err
	}
	reply := new(pb.GetDogInfoReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: dogService, MethodName: "GetDogInfo", Response: reply,
	}, &pb.GetDogInfoRequest{HostGid: s.hostGID})
	if err != nil {
		return DogGiftStatus{}, fmt.Errorf("get dog gift status: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetDogInfoReply)
		if !ok {
			return DogGiftStatus{}, fmt.Errorf("dog info returned %T, want *pb.GetDogInfoReply", response)
		}
	}
	claimable, _ := extractUnknownVarint(reply, 7)
	return DogGiftStatus{OK: true, Claimable: claimable, Raw: reply}, nil
}

func (s *service) ClaimDogGifts(ctx context.Context) (DogGiftResult, error) {
	if err := checkContext(ctx); err != nil {
		return DogGiftResult{}, err
	}
	reply := new(pb.GetDogInfoReply)
	response, err := s.transport.SendMsg(ctx, transport.Command{
		ServiceName: dogService, MethodName: "ClaimSkillGifts", Response: reply,
	}, &emptypb.Empty{})
	if err != nil {
		return DogGiftResult{}, fmt.Errorf("claim dog gifts: %w", err)
	}
	if response != nil {
		var ok bool
		reply, ok = response.(*pb.GetDogInfoReply)
		if !ok {
			return DogGiftResult{}, fmt.Errorf("dog gift response returned %T, want *pb.GetDogInfoReply", response)
		}
	}
	claimed := reply.GetCoin()
	if claimed == 0 {
		claimed, _ = extractUnknownVarint(reply, 3)
	}
	return DogGiftResult{OK: true, Claimed: claimed, Raw: reply}, nil
}

func ParseShareLink(raw string) InviteCode {
	value := strings.TrimSpace(raw)
	query := value
	if parsed, err := url.Parse(value); err == nil && parsed.RawQuery != "" {
		query = parsed.RawQuery
	} else {
		query = strings.TrimPrefix(query, "?")
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return InviteCode{}
	}
	uid, _ := strconv.ParseInt(strings.TrimSpace(values.Get("uid")), 10, 64)
	return InviteCode{UID: uid, OpenID: values.Get("openid"), ShareKey: values.Get("share_key"), ShareSource: values.Get("share_source"), DocID: values.Get("doc_id")}
}

func ParseInviteLink(raw string) InviteCode { return ParseShareLink(raw) }

func ReadShareFile(path string) ([]InviteCode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := make([]InviteCode, 0)
	seen := make(map[int64]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "openid=") {
			continue
		}
		code := ParseShareLink(strings.TrimSpace(line))
		if code.UID <= 0 || strings.TrimSpace(code.OpenID) == "" {
			continue
		}
		if _, exists := seen[code.UID]; exists {
			continue
		}
		seen[code.UID] = struct{}{}
		result = append(result, code)
	}
	return result, nil
}

func ClearShareFile(path string) error { return os.WriteFile(path, nil, 0o600) }

func ExtractVarintField(body []byte, fieldNo int) (int64, bool) {
	value, ok := readVarintField(body, fieldNo)
	return int64(value), ok
}

func readVarintField(body []byte, fieldNo int) (uint64, bool) {
	for offset := 0; offset < len(body); {
		key, next, ok := readUvarint(body, offset)
		if !ok {
			return 0, false
		}
		offset = next
		field := int(key >> 3)
		wire := int(key & 7)
		switch wire {
		case 0:
			value, next, ok := readUvarint(body, offset)
			if !ok {
				return 0, false
			}
			if field == fieldNo {
				return value, true
			}
			offset = next
		case 1:
			offset += 8
		case 2:
			length, next, ok := readUvarint(body, offset)
			if !ok || length > uint64(len(body)-next) {
				return 0, false
			}
			offset = next + int(length)
		case 5:
			offset += 4
		default:
			return 0, false
		}
		if offset > len(body) {
			return 0, false
		}
	}
	return 0, false
}

func extractUnknownVarint(message proto.Message, fieldNo int) (int64, bool) {
	if message == nil {
		return 0, false
	}
	value, ok := readVarintField(message.ProtoReflect().GetUnknown(), fieldNo)
	return int64(value), ok
}

func readUvarint(data []byte, offset int) (uint64, int, bool) {
	if offset < 0 || offset >= len(data) {
		return 0, offset, false
	}
	value, n := binary.Uvarint(data[offset:])
	if n <= 0 {
		return 0, offset, false
	}
	return value, offset + n, true
}

func appendVarintField(dst []byte, field int, value uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(field<<3))
	dst = append(dst, buf[:n]...)
	n = binary.PutUvarint(buf[:], value)
	return append(dst, buf[:n]...)
}

func appendBytesField(dst []byte, field int, value []byte) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(field<<3|2))
	dst = append(dst, buf[:n]...)
	n = binary.PutUvarint(buf[:], uint64(len(value)))
	dst = append(dst, buf[:n]...)
	return append(dst, value...)
}

func actionLabel(action int32) string {
	switch action {
	case 1:
		return "偷取作物"
	case 2:
		return "帮忙"
	case 3:
		return "捣乱"
	default:
		return "互动"
	}
}

func actionDetail(record InteractRecord) string {
	var detail string
	switch record.ActionType {
	case 1:
		switch {
		case record.CropName != "" && record.CropCount > 0:
			detail = fmt.Sprintf("偷取 %s x %d", record.CropName, record.CropCount)
		case record.CropName != "":
			detail = "偷取 " + record.CropName
		case record.CropCount > 0:
			detail = fmt.Sprintf("偷取作物 x %d", record.CropCount)
		default:
			detail = "偷取作物"
		}
	case 2:
		if record.Times > 0 {
			detail = fmt.Sprintf("帮忙 %d 次", record.Times)
		} else {
			detail = "帮忙"
		}
	case 3:
		if record.Times > 0 {
			detail = fmt.Sprintf("捣乱 %d 次", record.Times)
		} else {
			detail = "捣乱"
		}
	default:
		if record.Times > 0 {
			detail = fmt.Sprintf("互动 %d 次", record.Times)
		} else {
			detail = "互动"
		}
	}
	if record.LandID > 0 {
		detail += fmt.Sprintf(" · 地块 %d", record.LandID)
	}
	return detail
}

func toTimeSec(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value > 1_000_000_000_000 {
		return value / 1000
	}
	return value
}

func dateKey(now time.Time) string { return now.Local().Format("2006-01-02") }

func (s *service) markShareDone(key string) {
	s.stateMu.Lock()
	s.doneDateKey = key
	s.stateMu.Unlock()
}

func isAlreadyClaimed(err error) bool {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return strings.Contains(message, "code=1009001") || strings.Contains(message, "已经领取")
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
