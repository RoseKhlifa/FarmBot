package handlers

import (
	"errors"

	"github.com/RoseKhlifa/FarmBot/internal/domain/mall"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterMysteryShop(r gin.IRouter) {
	r.GET("/api/shop/mystery", h.mysteryShop)
	r.POST("/api/shop/mystery/buy", h.mysteryBuy)
	r.POST("/api/shop/mystery/abandon", h.mysteryAbandon)
}

func (h *Handler) mysteryService(c *gin.Context) (*mall.Domains, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	return resolve(c, h.app().Domains.Mall, id)
}
func (h *Handler) mysteryShop(c *gin.Context) {
	domains, ok := h.mysteryService(c)
	if !ok || domains.Mystery == nil {
		if ok {
			writeError(c, errors.New("mystery shop is not initialized"))
		}
		return
	}
	data, err := domains.Mystery.GetActiveMysteryShop(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) mysteryBuy(c *gin.Context) {
	domains, ok := h.mysteryService(c)
	if !ok || domains.Mystery == nil {
		if ok {
			writeError(c, errors.New("mystery shop is not initialized"))
		}
		return
	}
	var body struct {
		NPCID int64 `json:"npcId"`
		ID    int64 `json:"id"`
	}
	if !bindJSON(c, &body) {
		return
	}
	if body.NPCID <= 0 {
		body.NPCID = body.ID
	}
	data, err := domains.Mystery.BuyMysteryShopGoods(c.Request.Context(), body.NPCID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) mysteryAbandon(c *gin.Context) {
	domains, ok := h.mysteryService(c)
	if !ok || domains.Mystery == nil {
		if ok {
			writeError(c, errors.New("mystery shop is not initialized"))
		}
		return
	}
	if err := domains.Mystery.AbandonMysteryShop(c.Request.Context()); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
