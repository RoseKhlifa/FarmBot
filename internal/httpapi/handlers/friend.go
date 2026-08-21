package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/RoseKhlifa/FarmBot/internal/domain/friend"
	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterFriend(r gin.IRouter) {
	r.GET("/api/friends", h.friends)
	r.POST("/api/friends/clear-cache", h.clearFriendCache)
	r.POST("/api/friends/fetch-dog-info", h.fetchDogInfo)
	r.GET("/api/interact-records", h.interactRecords)
	r.GET("/api/friend/:gid/lands", h.friendLands)
	r.POST("/api/friend/:gid/op", h.friendOperation)
	r.GET("/api/friend/:gid/dog", h.friendDog)
	r.POST("/api/friend/batch-delete", h.batchDeleteFriend)
	r.POST("/api/friend/:gid/delete", h.deleteFriend)
	r.POST("/api/friend/apply", h.applyFriend)
	r.GET("/api/friend-blacklist", h.friendBlacklist)
	r.POST("/api/friend-blacklist/toggle", h.toggleBlacklist)
	r.POST("/api/friend-blacklist/update", h.updateBlacklist)
	r.GET("/api/friend-known-gids", h.knownGIDs)
	r.POST("/api/friend-known-gids", h.addKnownGID)
	r.POST("/api/friend-known-gids/remove", h.removeKnownGID)
	r.POST("/api/friend-known-gids/batch-add", h.batchAddKnownGIDs)
	r.POST("/api/friend-known-gids/batch-remove", h.batchRemoveKnownGIDs)
	r.GET("/api/dog/gifts", h.dogGifts)
	r.POST("/api/dog/gifts/claim", h.claimDogGifts)
	r.GET("/api/plant-blacklist", h.plantBlacklist)
	r.POST("/api/plant-blacklist", h.updatePlantBlacklist)
	r.DELETE("/api/plant-blacklist/:seedId", h.deletePlantBlacklist)
	r.POST("/api/plant-blacklist/batch", h.updatePlantBlacklist)
	r.DELETE("/api/plant-blacklist", h.clearPlantBlacklist)
}

func (h *Handler) friendService(c *gin.Context) (string, friend.Service, bool) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return "", nil, false
	}
	service, ok := resolve(c, h.app().Domains.Friend, id)
	return id, service, ok
}
func (h *Handler) friends(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	data, err := service.API().GetAllFriends(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) clearFriendCache(c *gin.Context) {
	id, _, ok := h.friendService(c)
	if !ok {
		return
	}
	if h.app().Cache == nil {
		writeNotConfigured(c)
		return
	}
	ctx := c.Request.Context()
	if err := h.app().Cache.InvalidateFriendList(ctx, id); err != nil {
		writeError(c, err)
		return
	}
	_ = h.app().Cache.InvalidateKnownFriendGIDs(ctx, id)
	_ = h.app().Cache.InvalidateFriendDogInfo(ctx, id)
	writeOK(c, nil)
}
func (h *Handler) fetchDogInfo(c *gin.Context) {
	id, service, ok := h.friendService(c)
	if !ok {
		return
	}
	friends, err := service.API().GetAllFriends(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	gids := make([]int64, 0, len(friends))
	for _, item := range friends {
		gids = append(gids, item.GID)
	}
	dogs, err := service.Analyzer().BatchGetFriendDogInfo(c.Request.Context(), gids)
	if err != nil {
		writeError(c, err)
		return
	}
	_ = id
	writeOK(c, dogs)
}
func (h *Handler) interactRecords(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return
	}
	if h.app().Domains.Social == nil {
		writeNotConfigured(c)
		return
	}
	service, err := h.app().Domains.Social(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	data, err := service.GetInteractRecords(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) friendLands(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	gid, ok := int64Param(c, "gid")
	if !ok {
		return
	}
	entered, err := service.API().EnterFriendFarm(c.Request.Context(), gid)
	if err != nil {
		writeError(c, err)
		return
	}
	defer func() { _ = service.API().LeaveFriendFarm(context.Background(), gid) }()
	writeOK(c, entered.GetLands())
}
func (h *Handler) friendOperation(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	gid, ok := int64Param(c, "gid")
	if !ok {
		return
	}
	var body struct {
		OpType  string  `json:"opType"`
		LandIDs []int64 `json:"landIds"`
	}
	if !bindJSON(c, &body) {
		return
	}
	action := friend.VisitAction(body.OpType)
	if action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "opType is required"})
		return
	}
	result, err := service.Visit().DoFriendOperation(c.Request.Context(), gid, action, body.LandIDs)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, result)
}
func (h *Handler) friendDog(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	gid, ok := int64Param(c, "gid")
	if !ok {
		return
	}
	data, err := service.Analyzer().GetFriendDogInfo(c.Request.Context(), gid)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) batchDeleteFriend(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	var body struct {
		GIDs     []int64 `json:"gids"`
		Password string  `json:"password"`
	}
	if !bindJSON(c, &body) {
		return
	}
	body.GIDs = friend.NormalizeGIDs(body.GIDs)
	if len(body.GIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请提供要删除的好友 GID 列表"})
		return
	}
	success := []int64{}
	failed := []any{}
	for _, gid := range body.GIDs {
		if err := service.API().DeleteFriend(c.Request.Context(), gid); err != nil {
			failed = append(failed, gin.H{"gid": gid, "error": err.Error()})
		} else {
			success = append(success, gid)
		}
	}
	writeOK(c, gin.H{"success": success, "failed": failed, "successCount": len(success), "failedCount": len(failed), "hasPassword": body.Password != ""})
}
func (h *Handler) deleteFriend(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	gid, ok := int64Param(c, "gid")
	if !ok {
		return
	}
	if err := service.API().DeleteFriend(c.Request.Context(), gid); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"message": "删除好友成功"})
}
func (h *Handler) applyFriend(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	var body struct {
		GID      int64  `json:"gid"`
		UID      int64  `json:"uid"`
		OpenID   string `json:"openid"`
		ShareKey string `json:"shareKey"`
		Token    string `json:"token"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.GID == 0 {
		body.GID = body.UID
	}
	if body.GID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid gid"})
		return
	}
	if err := service.API().ApplyFriend(c.Request.Context(), body.GID, body.Token); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"method": "ApplyFriend", "openid": body.OpenID, "shareKey": body.ShareKey})
}
func (h *Handler) friendBlacklist(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	data, err := service.API().Blacklist(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) toggleBlacklist(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	var body struct {
		GID     int64  `json:"gid"`
		Reason  string `json:"reason"`
		Blocked *bool  `json:"blocked"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.GID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Missing gid"})
		return
	}
	blacklist, _ := service.API().Blacklist(c.Request.Context())
	blocked := body.Blocked != nil && *body.Blocked
	if body.Blocked == nil {
		_, blocked = blacklist[body.GID]
	}
	var err error
	if blocked {
		err = service.API().RemoveBlacklist(c.Request.Context(), body.GID)
	} else {
		err = service.API().AddBlacklist(c.Request.Context(), body.GID, body.Reason)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"gid": body.GID, "blocked": !blocked})
}
func (h *Handler) updateBlacklist(c *gin.Context) {
	_, service, ok := h.friendService(c)
	if !ok {
		return
	}
	var body struct {
		GID       int64  `json:"gid"`
		Reason    string `json:"reason"`
		SkipSteal *bool  `json:"skipSteal"`
		SkipHelp  *bool  `json:"skipHelp"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.GID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Missing gid"})
		return
	}
	if err := service.API().AddBlacklist(c.Request.Context(), body.GID, body.Reason); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"gid": body.GID, "skipSteal": body.SkipSteal, "skipHelp": body.SkipHelp})
}
func (h *Handler) knownGIDs(c *gin.Context) {
	id, service, ok := h.friendService(c)
	if !ok {
		return
	}
	if h.app().Cache == nil {
		writeNotConfigured(c)
		return
	}
	value, err := h.app().Cache.GetKnownFriendGIDs(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	var gids []int64
	_ = json.Unmarshal(value.Payload, &gids)
	writeOK(c, gin.H{"knownFriendGids": friend.NormalizeGIDs(gids), "data": friend.NormalizeGIDs(gids), "accountId": id, "service": service.API() != nil})
}
func (h *Handler) addKnownGID(c *gin.Context)          { h.changeKnownGIDs(c, false, false) }
func (h *Handler) removeKnownGID(c *gin.Context)       { h.changeKnownGIDs(c, true, false) }
func (h *Handler) batchAddKnownGIDs(c *gin.Context)    { h.changeKnownGIDs(c, false, true) }
func (h *Handler) batchRemoveKnownGIDs(c *gin.Context) { h.changeKnownGIDs(c, true, true) }
func (h *Handler) changeKnownGIDs(c *gin.Context, remove, batch bool) {
	id, _, ok := h.friendService(c)
	if !ok {
		return
	}
	if h.app().Cache == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		GID  int64   `json:"gid"`
		GIDs []int64 `json:"gids"`
	}
	if !bindJSON(c, &body) {
		return
	}
	gids := body.GIDs
	if !batch && body.GID > 0 {
		gids = []int64{body.GID}
	}
	gids = friend.NormalizeGIDs(gids)
	if len(gids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "gid is required"})
		return
	}
	value, err := h.app().Cache.GetKnownFriendGIDs(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	var known []int64
	_ = json.Unmarshal(value.Payload, &known)
	set := map[int64]bool{}
	for _, gid := range friend.NormalizeGIDs(known) {
		set[gid] = true
	}
	for _, gid := range gids {
		if remove {
			delete(set, gid)
		} else {
			set[gid] = true
		}
	}
	out := make([]int64, 0, len(set))
	for gid := range set {
		out = append(out, gid)
	}
	raw, _ := json.Marshal(friend.NormalizeGIDs(out))
	if err := h.app().Cache.PutKnownFriendGIDs(c.Request.Context(), id, store.CacheValue{Payload: raw}); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, friend.NormalizeGIDs(out))
}

var _ = strconv.IntSize
var _ = middleware.AccountID

func (h *Handler) dogGifts(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok || h.app().Domains.Social == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	service, err := h.app().Domains.Social(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	data, err := service.GetDogGiftStatus(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) claimDogGifts(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok || h.app().Domains.Social == nil {
		if ok {
			writeNotConfigured(c)
		}
		return
	}
	service, err := h.app().Domains.Social(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	data, err := service.ClaimDogGifts(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) plantBlacklist(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	writeOK(c, plantBlacklistIDs(config))
}
func (h *Handler) updatePlantBlacklist(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	var body struct {
		SeedID  int64   `json:"seedId"`
		SeedIDs []int64 `json:"seedIds"`
	}
	if !bindJSON(c, &body) {
		return
	}
	ids := plantBlacklistIDs(config)
	if len(body.SeedIDs) > 0 {
		ids = append(ids, body.SeedIDs...)
	} else if body.SeedID > 0 {
		ids = append(ids, body.SeedID)
	} else {
		c.JSON(400, gin.H{"ok": false, "error": "seedId is required"})
		return
	}
	ids = normalizePlantBlacklistIDs(ids)
	if err := h.savePlantBlacklist(c, config, ids); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, ids)
}
func (h *Handler) deletePlantBlacklist(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	seedID, err := strconv.ParseInt(c.Param("seedId"), 10, 64)
	if err != nil || seedID <= 0 {
		c.JSON(400, gin.H{"ok": false, "error": "invalid seedId"})
		return
	}
	ids := make([]int64, 0)
	for _, value := range plantBlacklistIDs(config) {
		if value != seedID {
			ids = append(ids, value)
		}
	}
	if err := h.savePlantBlacklist(c, config, ids); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, ids)
}
func (h *Handler) clearPlantBlacklist(c *gin.Context) {
	id, ok := RequireAccountAccess(c)
	if !ok {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	if err := h.savePlantBlacklist(c, config, nil); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, []int64{})
}

func plantBlacklistIDs(config *store.AccountConfig) []int64 {
	if config == nil || len(config.PlantBlacklistJSON) == 0 {
		return []int64{}
	}
	var values []int64
	if json.Unmarshal(config.PlantBlacklistJSON, &values) != nil {
		return []int64{}
	}
	return normalizePlantBlacklistIDs(values)
}

func normalizePlantBlacklistIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (h *Handler) savePlantBlacklist(c *gin.Context, config *store.AccountConfig, values []int64) error {
	raw, err := json.Marshal(normalizePlantBlacklistIDs(values))
	if err != nil {
		return err
	}
	config.PlantBlacklistJSON = raw
	var current map[string]any
	if len(config.ConfigJSON) > 0 {
		_ = json.Unmarshal(config.ConfigJSON, &current)
	}
	if current == nil {
		current = map[string]any{}
	}
	current["plantBlacklist"] = normalizePlantBlacklistIDs(values)
	config.ConfigJSON, err = json.Marshal(current)
	if err != nil {
		return err
	}
	repo, ok := accountConfigRepository(h)
	if !ok {
		return ErrApplicationDependency
	}
	return repo.ApplyConfigSnapshot(c.Request.Context(), middleware.AccountID(c), *config)
}
