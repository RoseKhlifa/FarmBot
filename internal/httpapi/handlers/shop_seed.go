package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterSeedShop(r gin.IRouter) {
	r.GET("/api/seeds", func(c *gin.Context) { writeNotConfigured(c) })
	r.GET("/api/shop/seed", func(c *gin.Context) { writeNotConfigured(c) })
	r.POST("/api/shop/buy", func(c *gin.Context) { writeNotConfigured(c) })
}
