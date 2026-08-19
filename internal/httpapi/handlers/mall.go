package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/mall"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterMall(r gin.IRouter) {
	r.GET("/api/shop/mall", h.mallList)
	r.POST("/api/shop/mall/buy", h.mallBuy)
}
func (h *Handler) mallService(c *gin.Context) (*mall.Domains, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	service, ok := resolve(c, h.app().Domains.Mall, id)
	return service, ok
}
func (h *Handler) mallList(c *gin.Context) {
	service, ok := h.mallService(c)
	if !ok {
		return
	}
	data, err := service.GetMallGoodsList(c.Request.Context(), mall.MallSlotType)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) mallBuy(c *gin.Context) {
	service, ok := h.mallService(c)
	if !ok {
		return
	}
	var body struct {
		GoodsID int64 `json:"goodsId"`
		Count   int64 `json:"count"`
	}
	if !bindJSON(c, &body) {
		return
	}
	data, err := service.PurchaseMallGoods(c.Request.Context(), body.GoodsID, body.Count)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
