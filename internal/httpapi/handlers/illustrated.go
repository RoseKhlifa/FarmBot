package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/illustrated"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterIllustrated(r gin.IRouter) {
	r.GET("/api/illustrated", h.illustratedList)
	r.POST("/api/illustrated/buy", h.illustratedBuy)
	r.POST("/api/illustrated/buy-all", h.illustratedBuyAll)
}
func (h *Handler) illustratedService(c *gin.Context) (illustrated.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	service, ok := resolve(c, h.app().Domains.Illustrated, id)
	return service, ok
}
func (h *Handler) illustratedList(c *gin.Context) {
	service, ok := h.illustratedService(c)
	if !ok {
		return
	}
	data, err := service.GetIllustratedList(c.Request.Context(), c.Query("refresh") == "true", 1)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) illustratedBuy(c *gin.Context) {
	service, ok := h.illustratedService(c)
	if !ok {
		return
	}
	var body struct {
		OnlyClaimable bool `json:"onlyClaimable"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	data, err := service.ClaimAllRewards(c.Request.Context(), body.OnlyClaimable)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) illustratedBuyAll(c *gin.Context) { h.illustratedBuy(c) }
