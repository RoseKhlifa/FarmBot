package handlers

import (
	"strconv"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterSeedShop(r gin.IRouter) {
	r.GET("/api/seeds", h.seedList)
	r.GET("/api/shop/seed", h.seedList)
	r.POST("/api/shop/buy", h.seedBuy)
}

func (h *Handler) seedService(c *gin.Context) (farm.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	return resolve(c, h.app().Domains.Farm, id)
}

func (h *Handler) seedList(c *gin.Context) {
	service, ok := h.seedService(c)
	if !ok {
		return
	}
	shopID := farm.SeedShopID
	if value := c.Query("shopId"); value != "" {
		if parsed, valid := int64ParamValue(value); valid {
			shopID = parsed
		}
	}
	reply, err := service.API().GetShopInfo(c.Request.Context(), shopID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply.GetGoodsList())
}

func (h *Handler) seedBuy(c *gin.Context) {
	service, ok := h.seedService(c)
	if !ok {
		return
	}
	var body struct {
		GoodsID int64 `json:"goodsId"`
		Count   int64 `json:"count"`
		Price   int64 `json:"price"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.Count <= 0 {
		body.Count = 1
	}
	reply, err := service.API().BuyGoods(c.Request.Context(), body.GoodsID, body.Count, body.Price)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, reply)
}

func int64ParamValue(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil && parsed > 0
}
