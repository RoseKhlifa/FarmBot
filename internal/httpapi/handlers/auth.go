package handlers

import (
	"net/http"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/RoseKhlifa/FarmBot/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterAuth(r gin.IRouter) {
	r.POST("/api/login", h.login)
	r.POST("/api/register", h.register)
	r.GET("/api/card/info/:code", h.cardInfo)
	r.POST("/api/user/renew", h.renew)
	r.POST("/api/public/renew", h.publicRenew)
	r.POST("/api/public/reset-password/verify", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/public/reset-password/confirm", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/user/change-password", h.changePassword)
}
func (h *Handler) login(c *gin.Context) {
	var body struct{ Username, Password string }
	if !bindJSON(c, &body) {
		return
	}
	var user *store.User
	var err error
	if h.app().Auth != nil {
		value, e := h.app().Auth.Login(c.Request.Context(), body.Username, body.Password)
		user = &value
		err = e
	} else if h.app().Users != nil {
		user, err = h.app().Users.Authenticate(c.Request.Context(), body.Username, body.Password, c.ClientIP())
	} else {
		writeNotConfigured(c)
		return
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": err.Error()})
		return
	}
	response := gin.H{"user": user}
	if h.app().Sessions != nil {
		token, e := h.app().Sessions.Create(c.Request.Context(), *user)
		if e != nil {
			writeError(c, e)
			return
		}
		response["token"] = token
		response["adminToken"] = token
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": response})
}
func (h *Handler) register(c *gin.Context) {
	var body struct{ Username, Password, CardCode string }
	if !bindJSON(c, &body) {
		return
	}
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	user, err := h.app().Cards.RegisterWithCard(c.Request.Context(), body.Username, body.Password, body.CardCode)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, user)
}
func (h *Handler) cardInfo(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	card, err := h.app().Cards.Get(c.Request.Context(), strings.TrimSpace(c.Param("code")))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, card)
}
func (h *Handler) renew(c *gin.Context) {
	if h.app().Cards == nil {
		writeNotConfigured(c)
		return
	}
	user, _ := middleware.CurrentUser(c)
	var body struct {
		Code string `json:"code"`
	}
	if !bindJSON(c, &body) {
		return
	}
	updated, err := h.app().Cards.Renew(c.Request.Context(), user.Username, body.Code)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, updated)
}
func (h *Handler) publicRenew(c *gin.Context)    { h.renew(c) }
func (h *Handler) changePassword(c *gin.Context) { writeNotConfigured(c) }
