package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterPetShop(r gin.IRouter) {
	r.GET("/api/shop/pet", func(c *gin.Context) { writeNotConfigured(c) })
}
