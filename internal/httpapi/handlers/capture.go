package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterCapture(r gin.IRouter) {
	for _, route := range []struct{ method, path string }{{"GET", "/api/admin/capture-config"}, {"POST", "/api/admin/capture-config/test"}, {"POST", "/api/admin/capture-config"}, {"GET", "/api/capture/config"}, {"POST", "/api/capture/sessions"}, {"GET", "/api/capture/sessions/:flowId"}, {"GET", "/api/public/capture-certificate/:flowId/:token"}, {"POST", "/api/capture/sessions/:flowId/complete"}, {"DELETE", "/api/capture/sessions/:flowId"}} {
		path := route.path
		switch route.method {
		case "GET":
			r.GET(path, func(c *gin.Context) { h.capture(c, path) })
		case "POST":
			r.POST(path, func(c *gin.Context) { h.capture(c, path) })
		case "DELETE":
			r.DELETE(path, func(c *gin.Context) { h.capture(c, path) })
		}
	}
}
func (h *Handler) capture(c *gin.Context, path string) {
	if h.app().Capture == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	data, err := h.app().Capture.Handle(c.Request.Context(), path, body)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
