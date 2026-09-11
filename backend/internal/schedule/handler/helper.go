package handler

import (
	"errors"

	authmiddleware "github.com/Yogdunana/StarByte/backend/pkg/middleware/auth"
	"github.com/Yogdunana/StarByte/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getUserID 从 context 取出 uuid.UUID 类型的当前用户 ID
func getUserID(c *gin.Context) (uuid.UUID, error) {
	uid := authmiddleware.GetUserID(c)
	if uid == "" {
		return uuid.Nil, errors.New("未登录")
	}
	return uuid.Parse(uid)
}

// getEventID 从路径参数取日程 ID
func getEventID(c *gin.Context) (uuid.UUID, error) {
	idStr := c.Param("id")
	return uuid.Parse(idStr)
}

// getAttendeeID 从路径参数取参与人 ID
func getAttendeeID(c *gin.Context) (uuid.UUID, error) {
	idStr := c.Param("attendee_id")
	return uuid.Parse(idStr)
}

// getReminderID 从路径参数取提醒 ID
func getReminderID(c *gin.Context) (uuid.UUID, error) {
	idStr := c.Param("reminder_id")
	return uuid.Parse(idStr)
}

// bindJSON 绑定 JSON 请求体
func bindJSON(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}

// bindQuery 绑定查询参数
func bindQuery(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}
