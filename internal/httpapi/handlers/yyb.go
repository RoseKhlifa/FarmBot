package handlers

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterYYB(r gin.IRouter) {
	for _, path := range []string{"/api/yyb/accounts", "/api/yyb/getcode", "/api/yyb/thirdparty-code", "/api/yyb/qr/create", "/api/yyb/qr/poll", "/api/yyb/qr/confirm"} {
		p := path
		r.POST(p, func(c *gin.Context) { h.yyb(c, p) })
	}
}
func (h *Handler) yyb(c *gin.Context, path string) {
	if h.app().Yyb == nil {
		writeNotConfigured(c)
		return
	}
	var body map[string]any
	if c.Request.ContentLength > 0 && !bindJSON(c, &body) {
		return
	}
	data, err := h.app().Yyb.Handle(c.Request.Context(), path, body)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
