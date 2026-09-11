package handler

import (
	"github.com/google/uuid"

	schedDto "github.com/Yogdunana/StarByte/backend/internal/schedule/dto"
	"schedService" "github.com/Yogdunana/StarByte/backend/internal/schedule/service"
	"github.com/Yogdunana/StarByte/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

// ReminderHandler 日程提醒处理器
type ReminderHandler struct {
	svc schedService.ScheduleReminderService
}

// NewReminderHandler 构造函数
func NewReminderHandler(svc schedService.ScheduleReminderService) *ReminderHandler {
	return &ReminderHandler{svc: svc}
}

// Create 创建提醒
// POST /api/v1/schedule/reminders
func (h *ReminderHandler) Create(c *gin.Context) {
	var req schedDto.CreateReminderRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), uid, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Get 获取提醒详情
// GET /api/v1/schedule/reminders/:reminder_id
func (h *ReminderHandler) Get(c *gin.Context) {
	id, err := getReminderID(c)
	if err != nil {
		response.BadRequest(c, "无效的提醒 ID")
		return
	}
	resp, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// ListByEvent 按日程查提醒
// GET /api/v1/schedule/events/:event_id/reminders
func (h *ReminderHandler) ListByEvent(c *gin.Context) {
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	eventID, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	list, err := h.svc.ListByEvent(c.Request.Context(), uid, eventID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// ListMy 我的提醒列表
// GET /api/v1/schedule/reminders
func (h *ReminderHandler) ListMy(c *gin.Context) {
	page, pageSize := getPagination(c)
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, total, err := h.svc.ListByUser(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, page, pageSize)
}

// Cancel 取消提醒
// DELETE /api/v1/schedule/reminders/:reminder_id
func (h *ReminderHandler) Cancel(c *gin.Context) {
	id, err := getReminderID(c)
	if err != nil {
		response.BadRequest(c, "无效的提醒 ID")
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), uid, id); err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithoutData(c)
}

// Snooze 推迟提醒
// PATCH /api/v1/schedule/reminders/:reminder_id/snooze
func (h *ReminderHandler) Snooze(c *gin.Context) {
	id, err := getReminderID(c)
	if err != nil {
		response.BadRequest(c, "无效的提醒 ID")
		return
	}
	var req schedDto.SnoozeReminderRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.Snooze(c.Request.Context(), uid, id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}
