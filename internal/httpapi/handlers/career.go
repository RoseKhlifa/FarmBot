package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/career"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterCareer(r gin.IRouter) { r.GET("/api/career", h.careerInfo) }
func (h *Handler) careerInfo(c *gin.Context) {
	id, ok := accountID(c, true)
	if !ok {
		return
	}
	if h.app().Domains.Career == nil {
		writeNotConfigured(c)
		return
	}
	service, err := h.app().Domains.Career(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	data, err := service.GetCareerInfo(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}

var _ career.Service
