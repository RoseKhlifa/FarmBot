package handlers

import (
	"context"

	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterCard(r gin.IRouter) {
	r.GET("/api/admin/cards", h.cards)
	r.POST("/api/admin/cards", h.createCard)
	r.POST("/api/admin/cards/batch-delete", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/admin/cards/batch-renew", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/admin/cards/:code", h.updateCard)
	r.DELETE("/api/admin/cards/:code", h.deleteCard)
	r.GET("/api/card-claim/status", h.cardStatus)
	r.POST("/api/card-claim/claim", h.claimCard)
	r.POST("/api/admin/card-claim/status", h.updateClaimStatus)
	r.GET("/api/admin/card-claim/status", h.cardStatus)
	r.GET("/api/admin/card-claim/records", func(c *gin.Context) { writeNotConfigured(c) })
}
func (h *Handler) cards(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	data, err := h.app().Cards.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) createCard(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	var body store.CardSpec
	if !bindJSON(c, &body) {
		return
	}
	data, err := h.app().Cards.Create(c.Request.Context(), body)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) deleteCard(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	if err := h.app().Cards.Delete(c.Request.Context(), c.Param("code")); err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, nil)
}
func (h *Handler) cardStatus(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	enabled := true
	if repo, ok := h.app().Cards.(interface{ GetClaimStatus() bool }); ok {
		enabled = repo.GetClaimStatus()
	}
	available := 0
	if repo, ok := h.app().Cards.(interface {
		GetAvailableTimeCardCount(context.Context) (int, error)
	}); ok {
		value, err := repo.GetAvailableTimeCardCount(c.Request.Context())
		if err == nil {
			available = value
		}
	}
	c.JSON(200, gin.H{"ok": true, "enabled": enabled, "availableTimeCards": available})
}

func (h *Handler) updateCard(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	var update store.CardUpdate
	if !bindJSON(c, &update) {
		return
	}
	card, err := h.app().Cards.Update(c.Request.Context(), c.Param("code"), update)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, card)
}

func (h *Handler) updateClaimStatus(c *gin.Context) {
	setter, ok := h.app().Cards.(interface{ SetClaimEnabled(bool) })
	if !ok {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !bindJSON(c, &body) {
		return
	}
	setter.SetClaimEnabled(body.Enabled)
	writeOK(c, gin.H{"enabled": body.Enabled})
}

func (h *Handler) claimCard(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	card, err := h.app().Cards.ClaimByUA(c.Request.Context(), c.GetHeader("User-Agent"), body.Username)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{
		"ok":          true,
		"cardCode":    card.Code,
		"days":        card.Days,
		"description": card.Description,
	})
}
