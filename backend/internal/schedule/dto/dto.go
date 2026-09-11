package dto

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Event DTO
// ============================================================

// CreateEventRequest 创建日程请求
type CreateEventRequest struct {
	Title        string      `json:"title" binding:"required,max=200"`
	Description  string      `json:"description"`
	CalendarType string      `json:"calendar_type" binding:"omitempty,oneof=personal shared"`
	StartTime    time.Time   `json:"start_time" binding:"required"`
	EndTime      time.Time   `json:"end_time" binding:"required"`
	Location     string      `json:"location"`
	OnlineLink   string      `json:"online_link"`
	Visibility   string      `json:"visibility" binding:"omitempty,oneof=private shared public"`
	ShareTargets []uuid.UUID `json:"share_targets"`
	RepeatRule   string      `json:"repeat_rule"`
	Attendees    []AttendeeInput `json:"attendees"`
	Reminders    []ReminderInput  `json:"reminders"`
}

// AttendeeInput 参与人输入
type AttendeeInput struct {
	UserID           uuid.UUID `json:"user_id" binding:"required"`
	Role             int16     `json:"role" binding:"omitempty,oneof=1 2 3"`
}

// ReminderInput 提醒输入
type ReminderInput struct {
	RemindOffset int    `json:"remind_offset" binding:"required,gte=0"`
	RemindMethod string `json:"remind_method" binding:"omitempty,oneof=app email sms"`
}

// UpdateEventRequest 更新日程请求（乐观锁）
type UpdateEventRequest struct {
	Title        string      `json:"title" binding:"required,max=200"`
	Description  string      `json:"description"`
	CalendarType string      `json:"calendar_type" binding:"omitempty,oneof=personal shared"`
	StartTime    time.Time   `json:"start_time" binding:"required"`
	EndTime      time.Time   `json:"end_time" binding:"required"`
	Location     string      `json:"location"`
	OnlineLink   string      `json:"online_link"`
	Visibility   string      `json:"visibility" binding:"omitempty,oneof=private shared public"`
	ShareTargets []uuid.UUID `json:"share_targets"`
	RepeatRule   string      `json:"repeat_rule"`
	Version      int         `json:"version" binding:"required"`
}

// UpdateEventStatusRequest 更新状态请求
type UpdateEventStatusRequest struct {
	Status int16 `json:"status" binding:"required,oneof=1 2 3"`
}

// EventResponse 日程响应
type EventResponse struct {
	ID            uuid.UUID    `json:"id"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	CalendarType  string       `json:"calendar_type"`
	OwnerID       uuid.UUID    `json:"owner_id"`
	StartTime     time.Time    `json:"start_time"`
	EndTime       time.Time    `json:"end_time"`
	Location      string       `json:"location"`
	OnlineLink    string       `json:"online_link"`
	Visibility    string       `json:"visibility"`
	ShareTargets  []uuid.UUID  `json:"share_targets"`
	RepeatRule    string       `json:"repeat_rule"`
	RepeatID      *uuid.UUID   `json:"repeat_id,omitempty"`
	Status        int16        `json:"status"`
	MeetingID     *uuid.UUID   `json:"meeting_id,omitempty"`
	Version       int          `json:"version"`
	AttendeeCount int64        `json:"attendee_count,omitempty"`
	ReminderCount int64        `json:"reminder_count,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// EventListRequest 日程列表查询请求
type EventListRequest struct {
	Keyword      string    `form:"keyword"`
	Status       *int16    `form:"status"`
	CalendarType string    `form:"calendar_type"`
	StartDate    string    `form:"start_date"`
	EndDate      string    `form:"end_date"`
	Page         int       `form:"page"`
	PageSize     int       `form:"page_size"`
}

// EventTimeRangeRequest 时间区间查询
type EventTimeRangeRequest struct {
	From  time.Time `form:"from" binding:"required"`
	To    time.Time `form:"to" binding:"required"`
	Scope string    `form:"scope"` // day / week / month
}

// ============================================================
// Attendee DTO
// ============================================================

// AttendeeResponse 参与人响应
type AttendeeResponse struct {
	ID             uuid.UUID  `json:"id"`
	EventID        uuid.UUID  `json:"event_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Role           int16      `json:"role"`
	ResponseStatus int16      `json:"response_status"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// RespondAttendeeRequest 响应邀请
type RespondAttendeeRequest struct {
	ResponseStatus int16 `json:"response_status" binding:"required,oneof=1 2 3"` // 1接受 2拒绝 3待定
}

// AddAttendeesRequest 批量添加参与人
type AddAttendeesRequest struct {
	UserIDs []uuid.UUID `json:"user_ids" binding:"required,min=1"`
	Role    int16       `json:"role" binding:"omitempty,oneof=1 2 3"`
}

// ============================================================
// Reminder DTO
// ============================================================

// CreateReminderRequest 创建提醒
type CreateReminderRequest struct {
	EventID      uuid.UUID `json:"event_id" binding:"required"`
	RemindOffset int       `json:"remind_offset" binding:"required,gte=0"`
	RemindMethod string    `json:"remind_method" binding:"omitempty,oneof=app email sms"`
}

// ReminderResponse 提醒响应
type ReminderResponse struct {
	ID            uuid.UUID  `json:"id"`
	EventID       uuid.UUID  `json:"event_id"`
	UserID        uuid.UUID  `json:"user_id"`
	RemindOffset  int        `json:"remind_offset"`
	RemindTime    time.Time  `json:"remind_time"`
	RemindMethod  string     `json:"remind_method"`
	Status        int16      `json:"status"`
	FiredAt       *time.Time `json:"fired_at,omitempty"`
	SnoozeCount   int        `json:"snooze_count"`
	SnoozeMinutes int        `json:"snooze_minutes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// SnoozeReminderRequest 推迟提醒
type SnoozeReminderRequest struct {
	SnoozeMinutes int `json:"snooze_minutes" binding:"required,gte=1,lte=1440"`
}
