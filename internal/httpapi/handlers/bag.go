package handlers

import (
	"errors"

	"github.com/RoseKhlifa/FarmBot/internal/domain/warehouse"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterBag(r gin.IRouter) {
	r.GET("/api/bag", h.bag)
	r.POST("/api/bag/use", h.bagUse)
	r.POST("/api/bag/sell", h.bagSell)
	r.GET("/api/bag/seeds", h.bagSeeds)
	r.GET("/api/daily-gifts", h.dailyGifts)
}
func (h *Handler) warehouseService(c *gin.Context) (warehouse.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	service, ok := resolve(c, h.app().Domains.Warehouse, id)
	return service, ok
}
func (h *Handler) bag(c *gin.Context) {
	service, ok := h.warehouseService(c)
	if !ok {
		return
	}
	data, err := service.ListBag(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) bagUse(c *gin.Context) {
	service, ok := h.warehouseService(c)
	if !ok {
		return
	}
	var body struct {
		ItemID  int64                `json:"itemId"`
		Count   int64                `json:"count"`
		UID     int64                `json:"uid"`
		LandIDs []int64              `json:"landIds"`
		Items   []warehouse.UseEntry `json:"items"`
	}
	if !bindJSON(c, &body) {
		return
	}
	var data any
	var err error
	if len(body.Items) > 0 {
		data, err = service.BatchUse(c.Request.Context(), body.Items)
	} else {
		data, err = service.UseItem(c.Request.Context(), body.ItemID, body.Count, body.LandIDs)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) bagSell(c *gin.Context) {
	service, ok := h.warehouseService(c)
	if !ok {
		return
	}
	var body struct {
		Items []warehouse.Item `json:"items"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	var data any
	var err error
	if len(body.Items) > 0 {
		data, err = service.SellItems(c.Request.Context(), body.Items)
	} else {
		data, err = service.SellAll(c.Request.Context())
	}
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) bagSeeds(c *gin.Context) {
	service, ok := h.warehouseService(c)
	if !ok {
		return
	}
	data, err := service.ListBag(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	seeds := make([]warehouse.Item, 0)
	for _, item := range data.Items {
		if item.ID > 0 && item.Count > 0 {
			seeds = append(seeds, item)
		}
	}
	writeOK(c, seeds)
}
func (h *Handler) dailyGifts(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok {
		return
	}
	domains, resolved := resolve(c, h.app().Domains.Mall, id)
	if !resolved {
		return
	}
	if domains == nil {
		writeError(c, errors.New("mall domains are not initialized"))
		return
	}
	result := gin.H{
		"fertilizer": domains.FreeGiftDailyState(),
		"monthCard":  domains.MonthCard.DailyState(),
		"qqVip":      domains.QQVIP.DailyState(),
	}
	writeOK(c, result)
}
