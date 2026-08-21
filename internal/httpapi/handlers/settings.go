package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/platform/mailer"
	"github.com/RoseKhlifa/FarmBot/internal/platform/pusher"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

const userDefaultPlansKey = "legacy:user_default_account_plans"

type accountConfigAccess interface {
	GetConfig(context.Context, string) (*store.AccountConfig, error)
	ApplyConfigSnapshot(context.Context, string, store.AccountConfig) error
}

func (h *Handler) RegisterSettings(r gin.IRouter) {
	r.GET("/api/settings/default-plan", h.defaultPlan)
	r.PUT("/api/settings/default-plan", h.saveDefaultPlan)
	r.POST("/api/settings/default-plan/import", h.importDefaultPlan)
	r.POST("/api/settings/default-plan/apply", h.applyDefaultPlan)
	r.POST("/api/settings/default-plan/reset", h.resetDefaultPlan)
	r.POST("/api/settings/save", h.saveSettings)
	r.POST("/api/settings/theme", h.theme)
	r.POST("/api/settings/auto-code-refresh", h.saveAutoCodeRefresh)
	r.POST("/api/settings/auto-code-refresh/run", h.runAutoCodeRefresh)
	r.POST("/api/settings/offline-reminder", h.saveOfflineReminder)
	r.POST("/api/settings/offline-reminder/test", h.testOfflineReminder)
	r.GET("/api/settings", h.settings)
	r.GET("/api/settings/default", h.defaultSettings)
}

func accountConfigRepository(h *Handler) (accountConfigAccess, bool) {
	if h.app().Accounts == nil {
		return nil, false
	}
	repo, ok := h.app().Accounts.(accountConfigAccess)
	return repo, ok
}

func (h *Handler) loadAccountConfig(c *gin.Context, accountID string) (*store.AccountConfig, bool) {
	repo, ok := accountConfigRepository(h)
	if !ok {
		writeNotConfigured(c)
		return nil, false
	}
	config, err := repo.GetConfig(c.Request.Context(), accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return &store.AccountConfig{AccountID: accountID}, true
	}
	if err != nil {
		writeError(c, err)
		return nil, false
	}
	if config == nil {
		config = &store.AccountConfig{AccountID: accountID}
	}
	config.AccountID = accountID
	return config, true
}

func objectFromJSON(raw json.RawMessage) map[string]any {
	result := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &result)
	}
	return result
}

func rawObject(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, errors.New("expected JSON object")
	}
	return value, nil
}

func marshalJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return raw, nil
}

func rawValue(body map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	raw, ok := body[key]
	return raw, ok && len(raw) > 0 && string(raw) != "null"
}

func decodeString(raw json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return value, nil
}

func decodeInt64(raw json.RawMessage, key string) (int64, error) {
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return value, nil
}

func decodeFloat64(raw json.RawMessage, key string) (float64, error) {
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return value, nil
}

func decodeBool(raw json.RawMessage, key string) (bool, error) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return value, nil
}

func applySettingsPatch(config *store.AccountConfig, body map[string]json.RawMessage) error {
	if config == nil {
		return errors.New("account config is nil")
	}
	configMap := objectFromJSON(config.ConfigJSON)
	if raw, ok := rawValue(body, "automation"); ok {
		current := objectFromJSON(config.AutomationJSON)
		incoming, err := rawObject(raw)
		if err != nil {
			return fmt.Errorf("automation: %w", err)
		}
		for key, value := range incoming {
			current[key] = value
		}
		config.AutomationJSON, _ = marshalJSON(current)
		configMap["automation"] = current
	}
	if raw, ok := rawValue(body, "autoCodeRefresh"); ok {
		current := objectFromJSON(config.AutoCodeRefreshJSON)
		if len(current) == 0 {
			current = map[string]any{"enabled": false, "intervalMinutes": 60}
		}
		incoming, err := rawObject(raw)
		if err != nil {
			return fmt.Errorf("autoCodeRefresh: %w", err)
		}
		for key, value := range incoming {
			current[key] = value
		}
		config.AutoCodeRefreshJSON, _ = marshalJSON(current)
		configMap["autoCodeRefresh"] = current
	}
	if raw, ok := rawValue(body, "intervals"); ok {
		value, err := rawObject(raw)
		if err != nil {
			return fmt.Errorf("intervals: %w", err)
		}
		config.IntervalsJSON, _ = marshalJSON(value)
		configMap["intervals"] = value
	}
	if raw, ok := rawValue(body, "friendQuietHours"); ok {
		value, err := rawObject(raw)
		if err != nil {
			return fmt.Errorf("friendQuietHours: %w", err)
		}
		config.FriendQuietHoursJSON, _ = marshalJSON(value)
		configMap["friendQuietHours"] = value
	}
	if raw, ok := rawValue(body, "plantingStrategy"); ok {
		value, err := decodeString(raw, "plantingStrategy")
		if err != nil {
			return err
		}
		config.PlantingStrategy = strings.TrimSpace(value)
		configMap["plantingStrategy"] = config.PlantingStrategy
	}
	if raw, ok := rawValue(body, "preferredSeedId"); ok {
		value, err := decodeInt64(raw, "preferredSeedId")
		if err != nil {
			return err
		}
		config.PreferredSeedID = value
		configMap["preferredSeedId"] = value
	}
	if raw, ok := rawValue(body, "prioritize2x2Crops"); ok {
		value, err := decodeBool(raw, "prioritize2x2Crops")
		if err != nil {
			return err
		}
		config.Prioritize2x2Crops = value
		configMap["prioritize2x2Crops"] = value
	}
	if raw, ok := rawValue(body, "friendBadRetryDate"); ok {
		value, err := decodeString(raw, "friendBadRetryDate")
		if err != nil {
			return err
		}
		config.FriendBadRetryDate = strings.TrimSpace(value)
		configMap["friendBadRetryDate"] = config.FriendBadRetryDate
	}
	if raw, ok := rawValue(body, "stealDelaySeconds"); ok {
		value, err := decodeFloat64(raw, "stealDelaySeconds")
		if err != nil {
			return err
		}
		config.StealDelaySeconds = value
		configMap["stealDelaySeconds"] = value
	}
	if raw, ok := rawValue(body, "plantOrderRandom"); ok {
		value, err := decodeBool(raw, "plantOrderRandom")
		if err != nil {
			return err
		}
		config.PlantOrderRandom = value
		configMap["plantOrderRandom"] = value
	}
	if raw, ok := rawValue(body, "plantDelaySeconds"); ok {
		value, err := decodeFloat64(raw, "plantDelaySeconds")
		if err != nil {
			return err
		}
		config.PlantDelaySeconds = value
		configMap["plantDelaySeconds"] = value
	}
	intFields := []struct {
		key    string
		target *int64
	}{
		{"fertilizerBuyOrganicCount", &config.FertilizerBuyOrganicCount}, {"fertilizerBuyOrganicThresholdHours", &config.FertilizerBuyOrganicThresholdHours},
		{"fertilizerBuyNormalCount", &config.FertilizerBuyNormalCount}, {"fertilizerBuyNormalThresholdHours", &config.FertilizerBuyNormalThresholdHours},
		{"fertilizerBuyCheckIntervalMinutes", &config.FertilizerBuyCheckIntervalMinutes}, {"autoAcceptFriendMinLevel", &config.AutoAcceptFriendMinLevel},
		{"goldenBugKeepCount", &config.GoldenBugKeepCount}, {"goldenBugRoundLimit", &config.GoldenBugRoundLimit},
	}
	for _, field := range intFields {
		if raw, ok := rawValue(body, field.key); ok {
			value, err := decodeInt64(raw, field.key)
			if err != nil {
				return err
			}
			*field.target = value
			configMap[field.key] = value
		}
	}
	if raw, ok := rawValue(body, "friendHelpExpExhausted"); ok {
		value, err := decodeBool(raw, "friendHelpExpExhausted")
		if err != nil {
			return err
		}
		config.FriendHelpExpExhausted = value
		configMap["friendHelpExpExhausted"] = value
	}
	if raw, ok := rawValue(body, "bagSeedFallbackStrategy"); ok {
		value, err := decodeString(raw, "bagSeedFallbackStrategy")
		if err != nil {
			return err
		}
		config.BagSeedFallbackStrategy = strings.TrimSpace(value)
		configMap["bagSeedFallbackStrategy"] = config.BagSeedFallbackStrategy
	}
	jsonFields := []struct {
		key    string
		target *json.RawMessage
	}{
		{"knownFriendGids", &config.KnownFriendGIDsJSON}, {"friendBlacklist", &config.FriendBlacklistJSON}, {"plantBlacklist", &config.PlantBlacklistJSON},
		{"mysteryAutoBuyCurrencies", &config.MysteryAutoBuyCurrenciesJSON}, {"bagSeedPriority", &config.BagSeedPriorityJSON}, {"bagPriorityLandTypes", &config.BagPriorityLandTypesJSON},
	}
	for _, field := range jsonFields {
		if raw, ok := rawValue(body, field.key); ok {
			if field.key == "plantBlacklist" {
				var ids []int64
				if err := json.Unmarshal(raw, &ids); err != nil {
					return fmt.Errorf("plantBlacklist must be an integer array")
				}
				normalized, err := json.Marshal(normalizePlantBlacklistIDs(ids))
				if err != nil {
					return fmt.Errorf("plantBlacklist: %w", err)
				}
				raw = normalized
			}
			if !json.Valid(raw) {
				return fmt.Errorf("%s contains invalid JSON", field.key)
			}
			*field.target = append(json.RawMessage(nil), raw...)
			var value any
			_ = json.Unmarshal(raw, &value)
			configMap[field.key] = value
		}
	}
	for key, raw := range body {
		if _, exists := configMap[key]; exists {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("%s contains invalid JSON: %w", key, err)
		}
		configMap[key] = value
	}
	config.ConfigJSON, _ = marshalJSON(configMap)
	return nil
}

func settingsPayload(config *store.AccountConfig) map[string]any {
	result := objectFromJSON(config.ConfigJSON)
	result["automation"] = objectFromJSON(config.AutomationJSON)
	result["autoCodeRefresh"] = objectFromJSON(config.AutoCodeRefreshJSON)
	result["intervals"] = objectFromJSON(config.IntervalsJSON)
	result["friendQuietHours"] = objectFromJSON(config.FriendQuietHoursJSON)
	result["plantingStrategy"] = config.PlantingStrategy
	result["preferredSeedId"] = config.PreferredSeedID
	result["prioritize2x2Crops"] = config.Prioritize2x2Crops
	result["friendBadRetryDate"] = config.FriendBadRetryDate
	result["stealDelaySeconds"] = config.StealDelaySeconds
	result["plantOrderRandom"] = config.PlantOrderRandom
	result["plantDelaySeconds"] = config.PlantDelaySeconds
	result["fertilizerBuyOrganicCount"] = config.FertilizerBuyOrganicCount
	result["fertilizerBuyOrganicThresholdHours"] = config.FertilizerBuyOrganicThresholdHours
	result["fertilizerBuyNormalCount"] = config.FertilizerBuyNormalCount
	result["fertilizerBuyNormalThresholdHours"] = config.FertilizerBuyNormalThresholdHours
	result["fertilizerBuyCheckIntervalMinutes"] = config.FertilizerBuyCheckIntervalMinutes
	result["autoAcceptFriendMinLevel"] = config.AutoAcceptFriendMinLevel
	result["goldenBugKeepCount"] = config.GoldenBugKeepCount
	result["goldenBugRoundLimit"] = config.GoldenBugRoundLimit
	result["friendHelpExpExhausted"] = config.FriendHelpExpExhausted
	result["bagSeedFallbackStrategy"] = config.BagSeedFallbackStrategy
	for key, raw := range map[string]json.RawMessage{"knownFriendGids": config.KnownFriendGIDsJSON, "friendBlacklist": config.FriendBlacklistJSON, "plantBlacklist": config.PlantBlacklistJSON, "mysteryAutoBuyCurrencies": config.MysteryAutoBuyCurrenciesJSON, "bagSeedPriority": config.BagSeedPriorityJSON, "bagPriorityLandTypes": config.BagPriorityLandTypesJSON} {
		var value any
		if len(raw) > 0 && json.Unmarshal(raw, &value) == nil {
			result[key] = value
		}
	}
	return result
}

func (h *Handler) saveAccountSettings(c *gin.Context, body map[string]json.RawMessage) (map[string]any, bool) {
	id, ok := accountID(c, true)
	if !ok || !h.requireAccountOwner(c, id) {
		return nil, false
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return nil, false
	}
	if err := applySettingsPatch(config, body); err != nil {
		c.JSON(400, gin.H{"ok": false, "error": err.Error()})
		return nil, false
	}
	repo, _ := accountConfigRepository(h)
	if err := repo.ApplyConfigSnapshot(c.Request.Context(), id, *config); err != nil {
		writeError(c, err)
		return nil, false
	}
	if restarter, canRestart := h.app().Accounts.(interface {
		Restart(context.Context, string) error
	}); canRestart {
		if running, reportsState := h.app().Accounts.(interface{ IsRunning(string) bool }); !reportsState || running.IsRunning(id) {
			if err := restarter.Restart(c.Request.Context(), id); err != nil {
				writeError(c, err)
				return nil, false
			}
		}
	}
	return settingsPayload(config), true
}

func (h *Handler) saveSettings(c *gin.Context) {
	var body map[string]json.RawMessage
	if !bindJSON(c, &body) {
		return
	}
	data, ok := h.saveAccountSettings(c, body)
	if ok {
		writeOK(c, data)
	}
}

func (h *Handler) saveAutoCodeRefresh(c *gin.Context) {
	var body map[string]json.RawMessage
	if !bindJSON(c, &body) {
		return
	}
	data, ok := h.saveAccountSettings(c, map[string]json.RawMessage{"autoCodeRefresh": mustObjectRaw(body)})
	if ok {
		writeOK(c, gin.H{"autoCodeRefresh": data["autoCodeRefresh"]})
	}
}

func mustObjectRaw(body map[string]json.RawMessage) json.RawMessage {
	raw, _ := marshalJSON(body)
	return raw
}

func (h *Handler) runAutoCodeRefresh(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok || !h.requireAccountOwner(c, id) {
		return
	}
	restarter, ok := h.app().Accounts.(interface {
		Restart(context.Context, string) error
	})
	if !ok {
		writeNotConfigured(c)
		return
	}
	if err := restarter.Restart(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"accountId": id, "refreshed": true})
}

func (h *Handler) settings(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok || !h.requireAccountOwner(c, id) {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	data := settingsPayload(config)
	if h.app().Config != nil {
		if theme, err := h.app().Config.GetTheme(c.Request.Context()); err == nil {
			data["ui"] = map[string]any{"theme": theme}
		}
	}
	if user, authenticated := middleware.CurrentUser(c); authenticated && h.app().Config != nil {
		if raw, err := h.app().Config.GetOfflineReminder(c.Request.Context(), user.Username); err == nil {
			var reminder any
			if json.Unmarshal(raw, &reminder) == nil {
				data["offlineReminder"] = reminder
			}
		}
	}
	writeOK(c, data)
}

func (h *Handler) theme(c *gin.Context) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Theme string `json:"theme"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if err := h.app().Config.SetTheme(c.Request.Context(), body.Theme); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"theme": body.Theme})
}

func (h *Handler) defaultSettings(c *gin.Context) { writeOK(c, defaultPlanConfig()) }

func defaultPlanConfig() map[string]any {
	return map[string]any{
		"automation": map[string]any{}, "autoCodeRefresh": map[string]any{"enabled": false, "intervalMinutes": 60}, "plantingStrategy": "max_exp", "preferredSeedId": int64(0), "prioritize2x2Crops": false,
		"intervals": map[string]any{}, "friendQuietHours": map[string]any{"enabled": false, "start": "23:00", "end": "07:00"}, "stealDelaySeconds": float64(1), "plantOrderRandom": true, "plantDelaySeconds": float64(2),
		"fertilizerBuyOrganicCount": int64(1), "fertilizerBuyOrganicThresholdHours": int64(10), "fertilizerBuyNormalCount": int64(1), "fertilizerBuyNormalThresholdHours": int64(10), "fertilizerBuyCheckIntervalMinutes": int64(60), "mysteryAutoBuyCurrencies": []int64{},
		"goldenBugKeepCount": int64(0), "goldenBugRoundLimit": int64(24), "autoAcceptFriendMinLevel": int64(0), "bagSeedPriority": []int64{}, "bagSeedFallbackStrategy": "level", "bagPriorityLandTypes": []string{"purple", "gold", "black", "red", "normal"},
	}
}

func currentUsername(c *gin.Context) (string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok || strings.TrimSpace(user.Username) == "" {
		c.JSON(401, gin.H{"ok": false, "error": "Unauthorized"})
		return "", false
	}
	return strings.TrimSpace(user.Username), true
}

func (h *Handler) userPlans(c *gin.Context) (map[string]json.RawMessage, bool) {
	if h.app().Config == nil {
		writeNotConfigured(c)
		return nil, false
	}
	raw, err := h.app().Config.GetGlobal(c.Request.Context(), userDefaultPlansKey)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]json.RawMessage{}, true
	}
	if err != nil {
		writeError(c, err)
		return nil, false
	}
	plans := map[string]json.RawMessage{}
	if len(raw) > 0 && json.Unmarshal(raw, &plans) != nil {
		return map[string]json.RawMessage{}, true
	}
	return plans, true
}

func planResponse(raw json.RawMessage, exists bool) map[string]any {
	result := map[string]any{"exists": exists, "enabled": true, "config": defaultPlanConfig(), "updatedAt": int64(0)}
	if !exists || len(raw) == 0 {
		return result
	}
	var saved map[string]json.RawMessage
	if json.Unmarshal(raw, &saved) != nil {
		return result
	}
	if value, ok := saved["enabled"]; ok {
		var enabled bool
		if json.Unmarshal(value, &enabled) == nil {
			result["enabled"] = enabled
		}
	}
	if value, ok := saved["config"]; ok {
		var config map[string]any
		if json.Unmarshal(value, &config) == nil && config != nil {
			result["config"] = config
		}
	}
	if value, ok := saved["updatedAt"]; ok {
		var updatedAt int64
		if json.Unmarshal(value, &updatedAt) == nil {
			result["updatedAt"] = updatedAt
		}
	}
	return result
}

func (h *Handler) defaultPlan(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	plans, ok := h.userPlans(c)
	if !ok {
		return
	}
	raw, exists := plans[username]
	writeOK(c, planResponse(raw, exists))
}

func (h *Handler) savePlanForUser(c *gin.Context, username string, config json.RawMessage, enabled bool) (map[string]any, error) {
	plans, ok := h.userPlans(c)
	if !ok {
		return nil, ErrApplicationDependency
	}
	var value map[string]any
	if len(config) == 0 || json.Unmarshal(config, &value) != nil || value == nil {
		value = defaultPlanConfig()
	}
	saved := map[string]any{"enabled": enabled, "config": value, "updatedAt": time.Now().UnixMilli()}
	raw, err := marshalJSON(saved)
	if err != nil {
		return nil, err
	}
	plans[username] = raw
	all, err := marshalJSON(plans)
	if err != nil {
		return nil, err
	}
	if err := h.app().Config.SetGlobal(c.Request.Context(), userDefaultPlansKey, all); err != nil {
		return nil, err
	}
	return planResponse(raw, true), nil
}

func (h *Handler) saveDefaultPlan(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	var body map[string]json.RawMessage
	if !bindJSON(c, &body) {
		return
	}
	config := body["config"]
	enabled := true
	if raw, present := body["enabled"]; present {
		value, err := decodeBool(raw, "enabled")
		if err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		enabled = value
	}
	data, err := h.savePlanForUser(c, username, config, enabled)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}

func (h *Handler) importDefaultPlan(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok || !h.requireAccountOwner(c, id) {
		return
	}
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	config, ok := h.loadAccountConfig(c, id)
	if !ok {
		return
	}
	raw, _ := marshalJSON(settingsPayload(config))
	enabled := true
	if plans, valid := h.userPlans(c); valid {
		if previous, exists := plans[username]; exists {
			enabled, _ = planResponse(previous, true)["enabled"].(bool)
		}
	}
	data, err := h.savePlanForUser(c, username, raw, enabled)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}

func (h *Handler) applyDefaultPlan(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok || !h.requireAccountOwner(c, id) {
		return
	}
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	plans, ok := h.userPlans(c)
	if !ok {
		return
	}
	raw, exists := plans[username]
	if !exists {
		c.JSON(400, gin.H{"ok": false, "error": "尚未保存默认方案"})
		return
	}
	var saved map[string]json.RawMessage
	if json.Unmarshal(raw, &saved) != nil || len(saved["config"]) == 0 {
		c.JSON(400, gin.H{"ok": false, "error": "默认方案无效"})
		return
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(saved["config"], &body) != nil {
		c.JSON(400, gin.H{"ok": false, "error": "默认方案无效"})
		return
	}
	data, ok := h.saveAccountSettings(c, body)
	if ok {
		writeOK(c, data)
	}
}

func (h *Handler) resetDefaultPlan(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	raw, _ := marshalJSON(defaultPlanConfig())
	data, err := h.savePlanForUser(c, username, raw, true)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}

func (h *Handler) saveOfflineReminder(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]json.RawMessage
	if !bindJSON(c, &body) {
		return
	}
	current := map[string]any{}
	if raw, err := h.app().Config.GetOfflineReminder(c.Request.Context(), username); err == nil {
		current = objectFromJSON(raw)
	}
	for key, raw := range body {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		current[key] = value
	}
	payload, _ := marshalJSON(current)
	if err := h.app().Config.SetOfflineReminder(c.Request.Context(), username, payload); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, current)
}

func (h *Handler) testOfflineReminder(c *gin.Context) {
	username, ok := currentUsername(c)
	if !ok {
		return
	}
	if h.app().Config == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if c.Request.ContentLength > 0 {
		if !bindJSON(c, &body) {
			return
		}
	}
	if body == nil {
		body = map[string]any{}
	}
	if len(body) == 0 {
		if raw, err := h.app().Config.GetOfflineReminder(c.Request.Context(), username); err == nil {
			_ = json.Unmarshal(raw, &body)
		}
	}
	channel := strings.ToLower(strings.TrimSpace(fmt.Sprint(body["channel"])))
	if channel == "" {
		channel = "smtp"
	}
	if channel == "smtp" {
		result, err := mailer.SendSMTPEmail(c.Request.Context(), mailer.Config{SMTPHost: fmt.Sprint(body["smtpHost"]), SMTPPort: intNumber(body["smtpPort"], 465), SMTPUser: fmt.Sprint(body["smtpUser"]), SMTPPass: fmt.Sprint(body["smtpPass"]), SenderName: fmt.Sprint(body["senderName"])}, mailer.Message{RecipientEmail: fmt.Sprint(body["recipientEmail"]), Subject: "下线提醒（测试）", Content: "这是一封测试邮件，收到它代表你配置邮箱成功了！"})
		if err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		writeOK(c, result)
		return
	}
	result, err := pusher.SendPushooMessage(c.Request.Context(), pusher.Payload{Channel: channel, Endpoint: fmt.Sprint(body["endpoint"]), Token: fmt.Sprint(body["token"]), Title: fmt.Sprint(body["title"]) + "（测试）", Content: fmt.Sprint(body["msg"])})
	if err != nil {
		c.JSON(400, gin.H{"ok": false, "error": err.Error(), "data": result})
		return
	}
	writeOK(c, result)
}

func intNumber(value any, fallback int) int {
	switch number := value.(type) {
	case float64:
		if number > 0 {
			return int(number)
		}
	case int:
		if number > 0 {
			return number
		}
	case string:
		if parsed, err := strconv.Atoi(number); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
