package handlers

import (
	"net/http"

	"github.com/RoseKhlifa/FarmBot/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterCapture(r gin.IRouter) {
	for _, route := range []struct{ method, path string }{{"GET", "/api/admin/capture-config"}, {"POST", "/api/admin/capture-config/test"}, {"POST", "/api/admin/capture-config"}, {"GET", "/api/capture/config"}, {"POST", "/api/capture/sessions"}, {"GET", "/api/capture/sessions/:flowId"}, {"GET", "/api/public/capture-certificate/:flowId/:token"}, {"POST", "/api/capture/sessions/:flowId/complete"}, {"DELETE", "/api/capture/sessions/:flowId"}} {
		path := route.path
		switch route.method {
		case "GET":
			r.GET(path, func(c *gin.Context) { h.capture(c) })
		case "POST":
			r.POST(path, func(c *gin.Context) { h.capture(c) })
		case "DELETE":
			r.DELETE(path, func(c *gin.Context) { h.capture(c) })
		}
	}
}
func (h *Handler) capture(c *gin.Context) {
	if h.app().Capture == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	if body == nil {
		body = make(map[string]any)
	}
	if user, ok := middleware.CurrentUser(c); ok {
		body["_username"] = user.Username
		if middleware.HasAdminRole(user.Role) {
			body["_admin"] = "true"
		}
	}
	if flowID := c.Param("flowId"); flowID != "" {
		body["flowId"] = flowID
	}
	if token := c.Param("token"); token != "" {
		body["token"] = token
	}
	body["_method"] = c.Request.Method
	path := c.Request.URL.Path
	data, err := h.app().Capture.Handle(c.Request.Context(), path, body)
	if err != nil {
		writeError(c, err)
		return
	}
	if binary, ok := data.(BinaryResponse); ok {
		if binary.ContentType != "" {
			c.Header("Content-Type", binary.ContentType)
		}
		if binary.Filename != "" {
			c.Header("Content-Disposition", `inline; filename="`+binary.Filename+`"`)
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, binary.ContentType, binary.Data)
		return
	}
	writeOK(c, data)
}
