package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterMysteryShop(r gin.IRouter) {
	r.GET("/api/shop/mystery", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/shop/mystery/buy", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/shop/mystery/abandon", func(c *gin.Context) { writeNotConfigured(c) })
}
