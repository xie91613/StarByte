package handler

import (
	"errors"

	schedDto "github.com/Yogdunana/StarByte/backend/internal/schedule/dto"
	"schedService" "github.com/Yogdunana/StarByte/backend/internal/schedule/service"
	"github.com/Yogdunana/StarByte/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AttendeeHandler 日程参与人处理器
type AttendeeHandler struct {
	svc schedService.ScheduleAttendeeService
}

// NewAttendeeHandler 构造函数
func NewAttendeeHandler(svc schedService.ScheduleAttendeeService) *AttendeeHandler {
	return &AttendeeHandler{svc: svc}
}

// Add 添加单个参与人
// POST /api/v1/schedule/events/:event_id/attendees
func (h *AttendeeHandler) Add(c *gin.Context) {
	eventIDStr := c.Param("event_id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req struct {
		UserID uuid.UUID `json:"user_id" binding:"required"`
		Role   int16     `json:"role"`
	}
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.Add(c.Request.Context(), uid, eventID, req.UserID, req.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// BatchAdd 批量添加参与人
// POST /api/v1/schedule/events/:event_id/attendees/batch
func (h *AttendeeHandler) BatchAdd(c *gin.Context) {
	eventIDStr := c.Param("event_id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req schedDto.AddAttendeesRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, err := h.svc.BatchAdd(c.Request.Context(), uid, eventID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// List 列出日程参与人
// GET /api/v1/schedule/events/:event_id/attendees
func (h *AttendeeHandler) List(c *gin.Context) {
	eventIDStr := c.Param("event_id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, err := h.svc.ListByEvent(c.Request.Context(), uid, eventID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// Get 获取参与人详情
// GET /api/v1/schedule/events/:event_id/attendees/:attendee_id
func (h *AttendeeHandler) Get(c *gin.Context) {
	id, err := getAttendeeID(c)
	if err != nil {
		response.BadRequest(c, "无效的参与人 ID")
		return
	}
	resp, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Remove 移除参与人
// DELETE /api/v1/schedule/events/:event_id/attendees/:attendee_id
func (h *AttendeeHandler) Remove(c *gin.Context) {
	eventIDStr := c.Param("event_id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	attendeeID, err := getAttendeeID(c)
	if err != nil {
		response.BadRequest(c, "无效的参与人 ID")
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	if err := h.svc.Remove(c.Request.Context(), uid, eventID, attendeeID); err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithoutData(c)
}

// Respond 响应邀请
// PATCH /api/v1/schedule/events/:event_id/attendees/:attendee_id/respond
func (h *AttendeeHandler) Respond(c *gin.Context) {
	attendeeID, err := getAttendeeID(c)
	if err != nil {
		response.BadRequest(c, "无效的参与人 ID")
		return
	}
	var req schedDto.RespondAttendeeRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.Respond(c.Request.Context(), uid, attendeeID, req.ResponseStatus)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// ListMy 我作为参与人的日程列表（个人视角）
// GET /api/v1/schedule/attendees/me
func (h *AttendeeHandler) ListMy(c *gin.Context) {
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

// getPagination 从 query 提取分页参数
func getPagination(c *gin.Context) (int, int) {
	page := c.DefaultQuery("page", "1")
	size := c.DefaultQuery("page_size", "20")
	p, _ := parseInt(page)
	s, _ := parseInt(size)
	if p <= 0 {
		p = 1
	}
	if s <= 0 {
		s = 20
	}
	if s > 200 {
		s = 200
	}
	return p, s
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
