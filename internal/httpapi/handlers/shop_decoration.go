package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterDecorationShop(r gin.IRouter) {
	r.GET("/api/shop/decoration", func(c *gin.Context) { writeNotConfigured(c) })
}
