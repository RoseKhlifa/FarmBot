package handlers

import (
	"github.com/RoseKhlifa/FarmBot/internal/domain/task"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterTask(r gin.IRouter) {
	r.GET("/api/tasks", h.taskInfo)
	r.POST("/api/tasks/claim", h.taskClaim)
	r.POST("/api/tasks/claim-all", h.taskClaimAll)
}
func (h *Handler) taskService(c *gin.Context) (task.Service, bool) {
	id, ok := accountID(c, true)
	if !ok {
		return nil, false
	}
	service, ok := resolve(c, h.app().Domains.Task, id)
	return service, ok
}
func (h *Handler) taskInfo(c *gin.Context) {
	service, ok := h.taskService(c)
	if !ok {
		return
	}
	data, err := service.GetTaskInfo(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) taskClaim(c *gin.Context) {
	service, ok := h.taskService(c)
	if !ok {
		return
	}
	var body struct {
		ID     int64 `json:"id"`
		Shared bool  `json:"shared"`
	}
	if !bindJSON(c, &body) {
		return
	}
	data, err := service.ClaimTaskReward(c.Request.Context(), body.ID, body.Shared)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
func (h *Handler) taskClaimAll(c *gin.Context) {
	service, ok := h.taskService(c)
	if !ok {
		return
	}
	data, err := service.CheckAndClaimTasks(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, data)
}
