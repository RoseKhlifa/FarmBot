package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/farm"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterPetShop(r gin.IRouter) {
	r.GET("/api/shop/pet", h.petShop)
}

func (h *Handler) petShop(c *gin.Context) { h.profileShop(c, "pet") }

func (h *Handler) profileShop(c *gin.Context, kind string) {
	service, ok, err := h.seedService(c)
	if !ok {
		if err != nil {
			writeError(c, err)
		}
		return
	}
	profiles, err := service.API().GetShopProfiles(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	// The protocol exposes the same profile list for legacy pet/decoration
	// tabs. Preserve the server's complete profile metadata and let the UI
	// select the matching shop type/name when a deployment has one.
	writeOK(c, gin.H{"kind": kind, "shops": profiles.GetShopProfiles()})
}

var _ farm.Service
