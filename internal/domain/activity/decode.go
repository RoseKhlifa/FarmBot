package activity

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// ProtoField is a small, loss-tolerant wire representation used only for
// opaque activity payloads. Known activitypb messages are decoded with the
// generated types before this fallback is attempted.
type ProtoField struct {
	Number   protowire.Number
	WireType protowire.Type
	Varint   uint64
	Bytes    []byte
	Fixed32  uint32
	Fixed64  uint64
}

// ReadProtoFields scans a protobuf body without assuming a schema. Malformed
// trailing bytes are ignored so one opaque activity cannot break the others.
func ReadProtoFields(raw []byte) []ProtoField {
	fields := make([]ProtoField, 0)
	for len(raw) > 0 {
		number, wireType, tagBytes := protowire.ConsumeTag(raw)
		if tagBytes < 0 {
			break
		}
		raw = raw[tagBytes:]
		field := ProtoField{Number: number, WireType: wireType}
		var consumed int
		switch wireType {
		case protowire.VarintType:
			field.Varint, consumed = protowire.ConsumeVarint(raw)
		case protowire.Fixed32Type:
			field.Fixed32, consumed = protowire.ConsumeFixed32(raw)
		case protowire.Fixed64Type:
			field.Fixed64, consumed = protowire.ConsumeFixed64(raw)
		case protowire.BytesType:
			field.Bytes, consumed = protowire.ConsumeBytes(raw)
		default:
			consumed = protowire.ConsumeFieldValue(number, wireType, raw)
		}
		if consumed < 0 || consumed > len(raw) {
			break
		}
		if wireType == protowire.BytesType && field.Bytes != nil {
			field.Bytes = append([]byte(nil), field.Bytes...)
		}
		fields = append(fields, field)
		raw = raw[consumed:]
	}
	return fields
}

func GetProtoNumber(fields []ProtoField, number protowire.Number, fallback uint64) uint64 {
	for _, field := range fields {
		if field.Number == number && field.WireType == protowire.VarintType {
			return field.Varint
		}
	}
	return fallback
}

func GetProtoBytes(fields []ProtoField, number protowire.Number) []byte {
	for _, field := range fields {
		if field.Number == number && field.WireType == protowire.BytesType {
			return append([]byte(nil), field.Bytes...)
		}
	}
	return nil
}

func GetProtoBytesAll(fields []ProtoField, number protowire.Number) [][]byte {
	result := make([][]byte, 0)
	for _, field := range fields {
		if field.Number == number && field.WireType == protowire.BytesType {
			result = append(result, append([]byte(nil), field.Bytes...))
		}
	}
	return result
}

func GetProtoString(fields []ProtoField, number protowire.Number, fallback string) string {
	value := GetProtoBytes(fields, number)
	if len(value) == 0 {
		return fallback
	}
	return string(value)
}

// ScanLengthDelimitedFields recursively finds fields with target number.
func ScanLengthDelimitedFields(raw []byte, target protowire.Number, maxDepth int) [][]byte {
	if maxDepth <= 0 || len(raw) == 0 {
		return nil
	}
	result := make([][]byte, 0)
	for _, field := range ReadProtoFields(raw) {
		if field.WireType != protowire.BytesType {
			continue
		}
		if field.Number == target {
			result = append(result, append([]byte(nil), field.Bytes...))
		}
		result = append(result, ScanLengthDelimitedFields(field.Bytes, target, maxDepth-1)...)
	}
	return result
}

func ParseActivityItemMessage(raw []byte, itemName func(int64) string) (ActivityItem, bool) {
	fields := ReadProtoFields(raw)
	itemID := int64(GetProtoNumber(fields, 1, 0))
	if itemID <= 0 {
		return ActivityItem{}, false
	}
	count := int64(GetProtoNumber(fields, 2, 1))
	if count < 0 {
		count = 0
	}
	name := ""
	if itemName != nil {
		name = itemName(itemID)
	}
	return ActivityItem{ItemID: itemID, ItemCount: count, Count: count, Name: name}, true
}

func FlattenActivityNode(node *pb.ActivityNode, result ...[]*pb.ActivityInfo) []*pb.ActivityInfo {
	var output []*pb.ActivityInfo
	if len(result) > 0 {
		output = result[0]
	}
	if node == nil {
		return output
	}
	if info := node.GetActivity(); info != nil {
		clone := protoCloneActivityInfo(info)
		if clone.GetRandomShop() == nil && node.GetRandomShop() != nil {
			clone.RandomShop = node.GetRandomShop()
		}
		if clone.GetExchangeShop() == nil && node.GetExchangeShop() != nil {
			clone.ExchangeShop = node.GetExchangeShop()
		}
		if clone.GetDrawInfo() == nil && node.GetDrawInfo() != nil {
			clone.DrawInfo = node.GetDrawInfo()
		}
		output = append(output, clone)
	}
	for _, child := range node.GetChildren() {
		output = FlattenActivityNode(child, output)
	}
	return output
}

func FlattenActivityChildren(reply *pb.GetGroupReply) []*pb.ActivityInfo {
	if reply == nil {
		return nil
	}
	activities := FlattenActivityNode(reply.GetGroup())
	return activities
}

func NormalizeRandomShopInfo(raw *pb.RandomShopInfo, itemName func(int64) string) RandomShop {
	result := RandomShop{MaxManualRefreshCount: 6}
	if raw == nil {
		return result
	}
	result.NextRefreshTime = raw.GetNextRefreshTime()
	result.ManualRefreshCost = int64(raw.GetManualRefreshCost())
	result.ManualRefreshCurrencyID = int64(raw.GetManualRefreshCurrencyId())
	result.ManualRefreshExtraValue = int64(raw.GetManualRefreshExtraValue())
	result.MaxManualRefreshCount = int64(raw.GetMaxManualRefreshCount())
	if result.MaxManualRefreshCount == 0 {
		result.MaxManualRefreshCount = 6
	}
	result.ManualRefreshUsedCount = int64(raw.GetManualRefreshUsedCount())
	for _, rawItem := range raw.GetItems() {
		if rawItem == nil {
			continue
		}
		item := itemFromProto(rawItem.GetItem(), itemName)
		if item.ItemID <= 0 {
			continue
		}
		cost := itemFromProto(rawItem.GetCost(), itemName)
		stock, bought := int64(rawItem.GetStockCount()), int64(rawItem.GetBoughtCount())
		special := rawItem.GetSpecial()
		hasStock := stock > 0
		soldOut := special && hasStock && bought >= stock
		purchasable := special && hasStock && !soldOut
		remaining := int64(0)
		if purchasable {
			remaining = maxInt64(0, stock-bought)
		}
		label := "不可购买"
		if !special {
			label = "无库存"
		} else if soldOut {
			label = "售罄"
		} else if purchasable {
			label = "可购买"
		}
		name := rawItem.GetName()
		if name == "" {
			name = item.Name
		}
		result.Items = append(result.Items, RandomShopItem{
			ID: int64(rawItem.GetId()), Name: name, Item: item, Cost: cost,
			StockCount: stock, BoughtCount: bought, RemainingCount: remaining,
			Special: special, SoldOut: soldOut, Purchasable: purchasable, StatusLabel: label,
		})
	}
	return result
}

func NormalizeExchangeShopInfo(raw *pb.ExchangeShopInfo, itemName func(int64) string) []ExchangeShopItem {
	if raw == nil {
		return nil
	}
	result := make([]ExchangeShopItem, 0, len(raw.GetItems()))
	for _, rawItem := range raw.GetItems() {
		if rawItem == nil {
			continue
		}
		item := itemFromProto(rawItem.GetItem(), itemName)
		if item.ItemID <= 0 {
			continue
		}
		cost := itemFromProto(rawItem.GetCost(), itemName)
		status := int64(rawItem.GetStatus())
		repeatable := status > 1
		owned := rawItem.GetOwned()
		name := rawItem.GetName()
		if name == "" {
			name = item.Name
		}
		result = append(result, ExchangeShopItem{
			ID: int64(rawItem.GetId()), Sort: int64(rawItem.GetSort()), Status: status,
			Owned: owned, Name: name, Item: item, Cost: cost,
			Extra: rawItem.GetExtra(), IsRepeatable: repeatable,
			ExchangeLimit: maxInt64(0, status), OwnedBlocksExchange: owned && !repeatable,
		})
	}
	return result
}

func NormalizeDrawInfo(raw *pb.DrawInfo, itemName func(int64) string) DrawInfo {
	result := DrawInfo{FreeMax: 4, PaidMax: 4, PaidCurrencyID: 1002, PaidPrice: 30, FallbackPrice: 30}
	if raw == nil {
		result.Actions = ComputeHeluDrawActions(result)
		return result
	}
	if raw.GetMaxFreeCount() > 0 {
		result.FreeMax = int64(raw.GetMaxFreeCount())
	}
	if raw.GetMaxPaidCount() > 0 {
		result.PaidMax = int64(raw.GetMaxPaidCount())
	}
	result.FreeRemaining = clampInt64(int64(raw.GetFreeRemainingCount()), 0, result.FreeMax)
	result.PaidRemaining = clampInt64(int64(raw.GetPaidRemainingCount()), 0, result.PaidMax)
	if raw.GetPaidCurrencyId() > 0 {
		result.PaidCurrencyID = int64(raw.GetPaidCurrencyId())
	}
	if raw.GetPaidPrice() > 0 {
		result.PaidPrice = int64(raw.GetPaidPrice())
	}
	if raw.GetFallbackPrice() > 0 {
		result.FallbackPrice = int64(raw.GetFallbackPrice())
	} else {
		result.FallbackPrice = result.PaidPrice
	}
	result.FreeUsed = maxInt64(0, result.FreeMax-result.FreeRemaining)
	result.PaidUsed = maxInt64(0, result.PaidMax-result.PaidRemaining)
	for _, reward := range raw.GetRewards() {
		if reward == nil {
			continue
		}
		item := itemFromProto(reward.GetItem(), itemName)
		if item.ItemID <= 0 {
			continue
		}
		result.RewardPool = append(result.RewardPool, DrawPoolItem{
			ID: int64(reward.GetId()), Rarity: int64(reward.GetRarity()), Item: item, Probability: reward.GetProbability(),
		})
	}
	result.Actions = ComputeHeluDrawActions(result)
	return result
}

func ComputeHeluDrawActions(draw DrawInfo) DrawActions {
	one := DrawAction{Type: "none", Label: "已抽完"}
	if draw.FreeRemaining > 0 {
		one = DrawAction{Count: 1, Available: true, Type: "free", Label: "免费1次"}
	} else if draw.PaidRemaining > 0 {
		one = DrawAction{Count: 1, Available: true, Cost: draw.PaidPrice, CurrencyID: draw.PaidCurrencyID, Type: "paid", Label: "点券1次"}
	}
	batch := DrawAction{Type: "none", Label: "已抽完"}
	if draw.FreeRemaining > 0 {
		count := minInt64(4, draw.FreeRemaining)
		batch = DrawAction{Count: count, Available: true, Type: "free"}
		batch.Label = "免费批量"
	} else if draw.PaidRemaining > 0 {
		count := minInt64(4, draw.PaidRemaining)
		batch = DrawAction{Count: count, Available: true, Cost: count * draw.PaidPrice, CurrencyID: draw.PaidCurrencyID, Type: "paid"}
		batch.Label = "点券批量"
	}
	return DrawActions{One: one, Batch: batch}
}

// NormalizeSeasonInfo decodes the known season response layout from an
// opaque body. It deliberately only reads documented field numbers.
func NormalizeSeasonInfo(raw []byte, itemName func(int64) string) SeasonPassport {
	result := SeasonPassport{UID: HeluPassportUID, Title: "荷风游记", ActivityID: HeluActivityID}
	replyFields := ReadProtoFields(raw)
	seasonBytes := GetProtoBytes(replyFields, 1)
	if len(seasonBytes) == 0 {
		return result
	}
	seasonFields := ReadProtoFields(seasonBytes)
	passportBytes := GetProtoBytes(seasonFields, 10)
	if len(passportBytes) == 0 {
		return result
	}
	passportFields := ReadProtoFields(passportBytes)
	result.ActivityID = int64(GetProtoNumber(passportFields, 1, uint64(result.ActivityID)))
	result.CurrentLevel = int64(GetProtoNumber(passportFields, 2, 0))
	result.Score = int64(GetProtoNumber(passportFields, 3, 0))
	result.CurrentProgress = int64(GetProtoNumber(passportFields, 4, 0))
	result.NextLevelNeed = int64(GetProtoNumber(passportFields, 5, 0))
	result.MaxLevel = int64(GetProtoNumber(passportFields, 6, 0))
	result.FreeClaimedLevel = int64(GetProtoNumber(passportFields, 9, 0))
	result.PremiumClaimedLevel = int64(GetProtoNumber(passportFields, 11, 0))
	result.ClaimableLevels = maxInt64(0, result.CurrentLevel-result.FreeClaimedLevel)
	result.Title = GetProtoString(passportFields, 16, result.Title)
	result.ConfigText = GetProtoString(passportFields, 17, "")
	result.StartTime = int64(GetProtoNumber(seasonFields, 5, 0))
	result.EndTime = int64(GetProtoNumber(seasonFields, 6, 0))
	for _, tierBytes := range GetProtoBytesAll(passportFields, 8) {
		tierFields := ReadProtoFields(tierBytes)
		tier := SeasonRewardTier{Level: int64(GetProtoNumber(tierFields, 1, 0))}
		for _, itemBytes := range GetProtoBytesAll(tierFields, 2) {
			if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
				tier.FreeRewards = append(tier.FreeRewards, item)
			}
		}
		for _, itemBytes := range GetProtoBytesAll(tierFields, 3) {
			if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
				tier.PremiumRewards = append(tier.PremiumRewards, item)
			}
		}
		if tier.Level > 0 {
			result.RewardTiers = append(result.RewardTiers, tier)
		}
	}
	return result
}

func NormalizeSeasonClaimResult(raw []byte, itemName func(int64) string) ([]ActivityItem, *SeasonPassport) {
	fields := ReadProtoFields(raw)
	rewards := make([]ActivityItem, 0)
	for _, itemBytes := range GetProtoBytesAll(fields, 1) {
		if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
			rewards = append(rewards, item)
		}
	}
	passportBytes := GetProtoBytes(fields, 3)
	if len(passportBytes) == 0 {
		return rewards, nil
	}
	passportFields := ReadProtoFields(passportBytes)
	passport := &SeasonPassport{UID: HeluPassportUID, Title: "荷风游记", ActivityID: HeluActivityID}
	passport.CurrentLevel = int64(GetProtoNumber(passportFields, 2, 0))
	passport.FreeClaimedLevel = int64(GetProtoNumber(passportFields, 9, 0))
	passport.PremiumClaimedLevel = int64(GetProtoNumber(passportFields, 11, 0))
	passport.ClaimableLevels = maxInt64(0, passport.CurrentLevel-passport.FreeClaimedLevel)
	return rewards, passport
}

func solarStatusLabel(status int64) string {
	switch status {
	case 2:
		return "可领取"
	case 3:
		return "已领取"
	case 1:
		return "未开启"
	case 5:
		return "已结束"
	default:
		return "状态" + intToString(status)
	}
}

func NormalizeSolarTermsInfo(raw []byte, itemName func(int64) string) SolarTermsInfo {
	fields := ReadProtoFields(raw)
	result := SolarTermsInfo{NowTime: int64(GetProtoNumber(fields, 2, 0))}
	for _, termBytes := range GetProtoBytesAll(fields, 1) {
		termFields := ReadProtoFields(termBytes)
		term := SolarTerm{
			ID:        int64(GetProtoNumber(termFields, 1, 0)),
			Status:    int64(GetProtoNumber(termFields, 2, 0)),
			StartTime: int64(GetProtoNumber(termFields, 3, 0)),
			EndTime:   int64(GetProtoNumber(termFields, 4, 0)),
			Title:     GetProtoString(termFields, 6, ""),
		}
		term.StatusLabel = solarStatusLabel(term.Status)
		term.Claimable = term.Status == 2
		for _, itemBytes := range GetProtoBytesAll(termFields, 5) {
			if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
				term.Rewards = append(term.Rewards, item)
			}
		}
		if term.ID > 0 {
			result.Terms = append(result.Terms, term)
			if term.Claimable {
				result.ClaimableCount++
			}
		}
	}
	config := ReadProtoFields(GetProtoBytes(fields, 3))
	result.TipsText = GetProtoString(config, 3, "")
	for index := range result.Terms {
		term := result.Terms[index]
		if term.Claimable || term.Status == 3 {
			result.CurrentTerm = &term
			break
		}
	}
	if result.CurrentTerm == nil && len(result.Terms) > 0 {
		result.CurrentTerm = &result.Terms[0]
	}
	return result
}

func NormalizeSolarTermsClaimResult(raw []byte, itemName func(int64) string) ([]ActivityItem, *SolarTerm) {
	fields := ReadProtoFields(raw)
	rewards := make([]ActivityItem, 0)
	for _, itemBytes := range GetProtoBytesAll(fields, 1) {
		if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
			rewards = append(rewards, item)
		}
	}
	termBytes := GetProtoBytes(fields, 2)
	if len(termBytes) == 0 {
		return rewards, nil
	}
	wrapped := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapped = protowire.AppendBytes(wrapped, termBytes)
	terms := NormalizeSolarTermsInfo(wrapped, itemName)
	if len(terms.Terms) > 0 {
		return rewards, &terms.Terms[0]
	}
	return rewards, nil
}

func NormalizeGuanxingActivity(raw []byte, now time.Time, itemName func(int64) string) GuanxingActivity {
	if now.IsZero() {
		now = time.Now()
	}
	result := GuanxingActivity{ActivityID: GuanxingActivityID, Title: "观星礼录", SeasonTitle: "观星礼录", NowTime: now.Unix()}
	infoFields := findActivityInfoFields(raw, GuanxingActivityID)
	if len(infoFields) > 0 {
		result.Title = GetProtoString(infoFields, 4, result.Title)
		result.StartTime = int64(GetProtoNumber(infoFields, 6, 0))
		result.EndTime = int64(GetProtoNumber(infoFields, 7, 0))
	}
	constellation := findLargestField(raw, 110, 5)
	if len(constellation) == 0 {
		result.Warning = "未解析到星宿数据"
		return result
	}
	constellationFields := ReadProtoFields(constellation)
	groups := make(map[int64]GuanxingGroup)
	for _, groupBytes := range GetProtoBytesAll(constellationFields, 5) {
		group := normalizeGuanxingGroup(groupBytes)
		if group.ID > 0 {
			groups[group.ID] = group
		}
	}
	for _, nodeBytes := range GetProtoBytesAll(constellationFields, 4) {
		node := normalizeGuanxingNode(nodeBytes, groups, itemName)
		if node.ID > 0 {
			result.Nodes = append(result.Nodes, node)
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	result.TotalDays = len(result.Nodes)
	serverDay := int64(GetProtoNumber(constellationFields, 1, 0))
	result.CurrentDay = serverDay
	if result.CurrentDay <= 0 {
		result.CurrentDay = estimateConstellationDay(result.StartTime, result.NowTime, result.TotalDays)
	}
	for _, node := range result.Nodes {
		if node.Unlocked {
			result.UnlockedCount++
		}
		if node.Claimed {
			result.ClaimedCount++
		}
		if node.Claimable {
			result.ClaimableCount++
			result.PendingRewards = mergeActivityItems(result.PendingRewards, node.Rewards)
		}
	}
	return result
}

type GuanxingGroup struct {
	ID       int64
	Name     string
	Category string
	Explain  string
	Links    string
}

func findLargestField(raw []byte, target protowire.Number, depth int) []byte {
	chunks := ScanLengthDelimitedFields(raw, target, depth)
	var best []byte
	for _, chunk := range chunks {
		if len(chunk) > len(best) {
			best = chunk
		}
	}
	return best
}

func findActivityInfoFields(raw []byte, activityID int64) []ProtoField {
	for _, chunk := range ScanLengthDelimitedFields(raw, 1, 5) {
		fields := ReadProtoFields(chunk)
		if int64(GetProtoNumber(fields, 1, 0)) == activityID && GetProtoString(fields, 4, "") != "" {
			return fields
		}
	}
	return nil
}

func normalizeGuanxingGroup(raw []byte) GuanxingGroup {
	fields := ReadProtoFields(raw)
	group := GuanxingGroup{
		ID:    int64(GetProtoNumber(fields, 1, 0)),
		Name:  GetProtoString(fields, 3, ""),
		Links: GetProtoString(fields, 4, ""),
	}
	configText := GetProtoString(fields, 5, "")
	if configText != "" {
		var config struct {
			Category string `json:"category"`
			Explain  string `json:"explain"`
		}
		if json.Unmarshal([]byte(configText), &config) == nil {
			group.Category, group.Explain = config.Category, config.Explain
		}
	}
	return group
}

func normalizeGuanxingNode(raw []byte, groups map[int64]GuanxingGroup, itemName func(int64) string) GuanxingNode {
	fields := ReadProtoFields(raw)
	id := int64(GetProtoNumber(fields, 1, 0))
	group := groups[id]
	node := GuanxingNode{
		ID: id, Day: id, Name: group.Name, Category: group.Category, Explain: group.Explain, Links: group.Links,
		Unlocked:  GetProtoNumber(fields, 2, 0) == 1,
		Claimed:   GetProtoNumber(fields, 3, 0) == 1,
		Claimable: GetProtoNumber(fields, 4, 0) == 1,
	}
	if node.Name == "" {
		node.Name = "第" + intToString(id) + "宿"
	}
	node.StatusLabel = "未解锁"
	if node.Claimed {
		node.StatusLabel = "已领取"
	} else if node.Claimable {
		node.StatusLabel = "可领取"
	} else if node.Unlocked {
		node.StatusLabel = "已解锁"
	}
	for _, itemBytes := range GetProtoBytesAll(fields, 5) {
		if item, ok := ParseActivityItemMessage(itemBytes, itemName); ok {
			node.Rewards = append(node.Rewards, item)
		}
	}
	return node
}

func estimateConstellationDay(start, now int64, total int) int64 {
	if start <= 0 || now <= start {
		return 1
	}
	maxDay := int64(total)
	if maxDay <= 0 {
		maxDay = 28
	}
	day := (now-start)/86400 + 1
	if day < 1 {
		return 1
	}
	return minInt64(day, maxDay)
}

func mergeActivityItems(items, additions []ActivityItem) []ActivityItem {
	result := append([]ActivityItem(nil), items...)
	for _, item := range additions {
		if item.ItemID <= 0 {
			continue
		}
		found := false
		for index := range result {
			if result[index].ItemID == item.ItemID {
				result[index].ItemCount += item.ItemCount
				result[index].Count = result[index].ItemCount
				found = true
				break
			}
		}
		if !found {
			result = append(result, item)
		}
	}
	return result
}

func protoCloneActivityInfo(info *pb.ActivityInfo) *pb.ActivityInfo {
	if info == nil {
		return nil
	}
	return proto.Clone(info).(*pb.ActivityInfo)
}

func clampInt64(value, low, high int64) int64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func intToString(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		digits[index] = '-'
	}
	return string(digits[index:])
}
