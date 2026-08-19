package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/activity"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterActivity(r gin.IRouter) {
	r.GET("/api/activity/list", h.activityList)
	r.GET("/api/activity/group/:id", h.activityGroup)
	r.GET("/api/server-version", h.serverVersion)
	r.GET("/api/activity/helu", h.helu)
	r.POST("/api/activity/helu/draw", h.heluDraw)
	r.POST("/api/activity/helu/passport/claim", h.heluPassport)
	r.POST("/api/activity/helu/solar/claim", h.heluSolar)
	r.POST("/api/activity/helu/exchange", h.heluExchange)
	r.POST("/api/activity/qingmei/claim", h.qingmeiClaim)
	r.POST("/api/activity/qingmei/wine/sell", h.qingmeiWine)
	r.GET("/api/activity/guanxing", h.guanxing)
	r.POST("/api/activity/guanxing/claim", h.guanxingClaim)
	r.GET("/api/activity/shop", h.activityShop)
	r.POST("/api/activity/shop/buy", h.activityShopBuy)
	r.POST("/api/activity/shop/refresh", h.activityShopRefresh)
}
func (h *Handler) activityService(c *gin.Context) (activity.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	service, ok := resolve(c, h.app().Domains.Activity, id)
	return service, ok
}
func (h *Handler) activityList(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.ListActivities(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) activityGroup(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	id, ok := int64Param(c, "id")
	if !ok {
		return
	}
	data, err := service.GetActivityGroup(c.Request.Context(), id, c.Query("uid"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) serverVersion(c *gin.Context) { writeOK(c, gin.H{"version": "go"}) }
func (h *Handler) helu(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.GetHeluActivity(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) heluDraw(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.DrawHeluGiftLotus(c.Request.Context(), activity.HeluDrawOptions{})
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) heluPassport(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.ClaimSeasonPassportRewards(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) heluSolar(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	var body struct {
		ID int64 `json:"id"`
	}
	if !bindJSON(c, &body) {
		return
	}
	data, err := service.ClaimSolarTermsReward(c.Request.Context(), body.ID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) heluExchange(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	var body struct{ GoodsID, Count int64 }
	if !bindJSON(c, &body) {
		return
	}
	data, err := service.ExchangeHeluShopItem(c.Request.Context(), body.GoodsID, body.Count)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) qingmeiClaim(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.ClaimQingmeiSeeds(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) qingmeiWine(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.BrewAndSellQingmeiWine(c.Request.Context(), activity.QingmeiWineOptions{})
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) guanxing(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.GetGuanxingActivity(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) guanxingClaim(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.ClaimGuanxingRewards(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) activityShop(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.GetNanguaShop(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) activityShopBuy(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	var body struct{ SlotID, Count int64 }
	if !bindJSON(c, &body) {
		return
	}
	data, err := service.BuyNanguaShopItem(c.Request.Context(), body.SlotID, body.Count)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) activityShopRefresh(c *gin.Context) {
	service, ok := h.activityService(c)
	if !ok {
		return
	}
	data, err := service.RefreshNanguaShop(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
