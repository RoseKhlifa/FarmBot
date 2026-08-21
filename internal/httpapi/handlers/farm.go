package handlers

import (
	"encoding/json"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/RoseKhlifa/FarmBot/internal/domain/mall"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterFarm(r gin.IRouter) {
	r.GET("/api/status", h.status)
	r.POST("/api/automation", h.automation)
	r.POST("/api/fertilizer/buy", h.buyFertilizer)
	r.POST("/api/fertilizer/check-and-buy", h.checkBuyFertilizer)
	r.GET("/api/lands", h.lands)
	r.POST("/api/farm/operate", h.farmOperate)
	r.POST("/api/land/fertilize", h.landFertilize)
	r.POST("/api/land/remove", h.landRemove)
	r.POST("/api/land/remove-all", h.landRemoveAll)
}
func (h *Handler) farmService(c *gin.Context) (string, farm.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return "", nil, false
	}
	service, ok := resolve(c, h.app().Domains.Farm, id)
	return id, service, ok
}
func (h *Handler) status(c *gin.Context) {
	id, ok := accountID(c, false)
	if !ok {
		return
	}
	if h.app().Runtime == nil {
		writeOK(c, gin.H{"running": false, "accountId": id})
		return
	}
	data, err := h.app().Runtime.Status(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) automation(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok {
		return
	}
	var body jsonMap
	if !bindJSON(c, &body) {
		return
	}
	if h.app().Runtime == nil {
		writeNotConfigured(c)
		return
	}
	data, err := h.app().Runtime.Automation(c.Request.Context(), id, body.raw())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) lands(c *gin.Context) {
	_, service, ok := h.farmService(c)
	if !ok {
		return
	}
	reply, err := service.API().GetAllLands(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply.GetLands())
}
func (h *Handler) farmOperate(c *gin.Context) {
	_, service, ok := h.farmService(c)
	if !ok {
		return
	}
	var body struct {
		Operation string `json:"operation"`
		Op        string `json:"op"`
	}
	if !bindJSON(c, &body) {
		return
	}
	operation := body.Operation
	if operation == "" {
		operation = body.Op
	}
	result, err := service.RunFarmOperation(c.Request.Context(), operation)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, result)
}
func (h *Handler) landFertilize(c *gin.Context) {
	_, service, ok := h.farmService(c)
	if !ok {
		return
	}
	var body struct {
		LandIDs      []int64 `json:"landIds"`
		FertilizerID int64   `json:"fertilizerId"`
	}
	if !bindJSON(c, &body) {
		return
	}
	reply, err := service.API().Fertilize(c.Request.Context(), body.LandIDs, body.FertilizerID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply)
}
func (h *Handler) landRemove(c *gin.Context) {
	_, service, ok := h.farmService(c)
	if !ok {
		return
	}
	var body struct {
		LandIDs []int64 `json:"landIds"`
	}
	if !bindJSON(c, &body) {
		return
	}
	reply, err := service.API().RemovePlant(c.Request.Context(), body.LandIDs)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply)
}
func (h *Handler) landRemoveAll(c *gin.Context) {
	_, service, ok := h.farmService(c)
	if !ok {
		return
	}
	reply, err := service.RunFarmOperation(c.Request.Context(), string(farm.OperationClear))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply)
}
func (h *Handler) buyFertilizer(c *gin.Context) {
	id, _, ok := h.farmService(c)
	if !ok {
		return
	}
	domains, ok := resolve(c, h.app().Domains.Mall, id)
	if !ok || domains == nil || domains.Mall == nil {
		return
	}
	var body struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
		Force bool   `json:"force"`
	}
	if !bindJSON(c, &body) {
		return
	}
	fertilizer := mall.FertilizerOrganic
	if strings.EqualFold(strings.TrimSpace(body.Type), "normal") || strings.EqualFold(strings.TrimSpace(body.Type), "inorganic") {
		fertilizer = mall.FertilizerNormal
	}
	if body.Count <= 0 {
		body.Count = 1
	}
	bought, err := domains.Mall.BuyFertilizer(c.Request.Context(), fertilizer, body.Count, body.Force)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, gin.H{"type": fertilizer, "bought": bought})
}
func (h *Handler) checkBuyFertilizer(c *gin.Context) {
	id, _, ok := h.farmService(c)
	if !ok {
		return
	}
	domains, ok := resolve(c, h.app().Domains.Mall, id)
	if !ok || domains == nil || domains.Mall == nil {
		return
	}
	var body struct {
		BuyOrganic          bool  `json:"buyOrganic"`
		OrganicCount        int64 `json:"organicCount"`
		OrganicThresholdHrs int64 `json:"organicThresholdHours"`
		BuyNormal           bool  `json:"buyNormal"`
		NormalCount         int64 `json:"normalCount"`
		NormalThresholdHrs  int64 `json:"normalThresholdHours"`
		Force               bool  `json:"force"`
	}
	if !bindJSON(c, &body) {
		return
	}
	result, err := domains.Mall.CheckAndBuyFertilizerBoth(c.Request.Context(), mall.FertilizerCheckOptions{
		BuyOrganic: body.BuyOrganic, OrganicCount: body.OrganicCount, OrganicThresholdHrs: float64(body.OrganicThresholdHrs),
		BuyNormal: body.BuyNormal, NormalCount: body.NormalCount, NormalThresholdHrs: float64(body.NormalThresholdHrs), Force: body.Force,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, result)
}

type jsonMap map[string]any

func (m jsonMap) raw() []byte { data, _ := json.Marshal(m); return data }
