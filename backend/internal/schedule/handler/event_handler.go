package handler

import (
	schedDto "github.com/Yogdunana/StarByte/backend/internal/schedule/dto"
	"schedService" "github.com/Yogdunana/StarByte/backend/internal/schedule/service"
	"github.com/Yogdunana/StarByte/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

// EventHandler 日程事件处理器
type EventHandler struct {
	svc schedService.ScheduleEventService
}

// NewEventHandler 构造函数
func NewEventHandler(svc schedService.ScheduleEventService) *EventHandler {
	return &EventHandler{svc: svc}
}

// Create 创建日程
// POST /api/v1/schedule/events
func (h *EventHandler) Create(c *gin.Context) {
	var req schedDto.CreateEventRequest
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

// Get 获取日程详情
// GET /api/v1/schedule/events/:id
func (h *EventHandler) Get(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	resp, err := h.svc.GetByID(c.Request.Context(), id, nil)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// List 日程列表（全局可见）
// GET /api/v1/schedule/events
func (h *EventHandler) List(c *gin.Context) {
	var req schedDto.EventListRequest
	if !bindQuery(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, total, err := h.svc.List(c.Request.Context(), uid, &req, nil)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, req.Page, req.PageSize)
}

// ListMyOwner 我创建的日程
// GET /api/v1/schedule/events/owner
func (h *EventHandler) ListMyOwner(c *gin.Context) {
	var req schedDto.EventListRequest
	if !bindQuery(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, total, err := h.svc.ListByOwner(c.Request.Context(), uid, uid, &req, nil)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, req.Page, req.PageSize)
}

// ListMyAttended 我参与的日程
// GET /api/v1/schedule/events/attended
func (h *EventHandler) ListMyAttended(c *gin.Context) {
	var req schedDto.EventListRequest
	if !bindQuery(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, total, err := h.svc.ListByAttendee(c.Request.Context(), uid, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, req.Page, req.PageSize)
}

// ListByTimeRange 时间区间查询
// GET /api/v1/schedule/events/range
func (h *EventHandler) ListByTimeRange(c *gin.Context) {
	var req schedDto.EventTimeRangeRequest
	if !bindQuery(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	list, err := h.svc.ListByTimeRange(c.Request.Context(), uid, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// Update 更新日程
// PUT /api/v1/schedule/events/:id
func (h *EventHandler) Update(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req schedDto.UpdateEventRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), uid, id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Delete 删除日程
// DELETE /api/v1/schedule/events/:id
func (h *EventHandler) Delete(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, id); err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithoutData(c)
}

// UpdateStatus 更新日程状态
// PATCH /api/v1/schedule/events/:id/status
func (h *EventHandler) UpdateStatus(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req schedDto.UpdateEventStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.UpdateStatus(c.Request.Context(), uid, id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// LinkMeeting 关联会议
// POST /api/v1/schedule/events/:id/meeting
func (h *EventHandler) LinkMeeting(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req struct {
		MeetingID  string `json:"meeting_id" binding:"required"`
		PrevVersion int   `json:"prev_version" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	meetingID, err := uuid.Parse(req.MeetingID)
	if err != nil {
		response.BadRequest(c, "无效的会议 ID")
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.LinkMeeting(c.Request.Context(), uid, id, meetingID, req.PrevVersion)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// UnlinkMeeting 解除会议关联
// DELETE /api/v1/schedule/events/:id/meeting
func (h *EventHandler) UnlinkMeeting(c *gin.Context) {
	id, err := getEventID(c)
	if err != nil {
		response.BadRequest(c, "无效的日程 ID")
		return
	}
	var req struct {
		PrevVersion int `json:"prev_version" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	uid, err := getUserID(c)
	if err != nil {
		response.Unauthorized(c, "未登录")
		return
	}
	resp, err := h.svc.UnlinkMeeting(c.Request.Context(), uid, id, req.PrevVersion)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}
