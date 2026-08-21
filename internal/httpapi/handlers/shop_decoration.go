package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterDecorationShop(r gin.IRouter) {
	r.GET("/api/shop/decoration", h.decorationShop)
}

func (h *Handler) decorationShop(c *gin.Context) { h.profileShop(c, "decoration") }
