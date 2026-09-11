package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// schedule_events：日程事件主体
// ============================================================

// 日程日历类型
const (
	CalendarTypePersonal = "personal" // 个人日历
	CalendarTypeShared   = "shared"   // 共享日历
)

// 日程可见性
const (
	VisibilityPrivate = "private" // 私有，仅自己可见
	VisibilityShared  = "shared"  // 共享给指定用户
	VisibilityPublic  = "public"  // 公开，所有人可见
)

// 日程状态
const (
	EventStatusDraft    int16 = 0 // 草稿
	EventStatusPublished int16 = 1 // 已发布
	EventStatusCanceled  int16 = 2 // 已取消
	EventStatusCompleted int16 = 3 // 已完成
)

// Event 日程事件主体表
type Event struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Title        string     `gorm:"type:varchar(200);not null" json:"title"`
	Description  string     `gorm:"type:text;not null;default:''" json:"description"`
	CalendarType string     `gorm:"type:varchar(20);not null;default:personal" json:"calendar_type"`
	OwnerID      uuid.UUID  `gorm:"type:uuid;not null" json:"owner_id"`
	StartTime    time.Time  `gorm:"not null" json:"start_time"`
	EndTime      time.Time  `gorm:"not null" json:"end_time"`
	Location     string     `gorm:"type:varchar(300);not null;default:''" json:"location"`
	OnlineLink   string     `gorm:"type:varchar(500);not null;default:''" json:"online_link"`
	Visibility   string     `gorm:"type:varchar(20);not null;default:private" json:"visibility"`
	ShareTargets string     `gorm:"column:share_targets;type:jsonb;not null;default:'[]'" json:"share_targets"`
	RepeatRule   string     `gorm:"type:varchar(500);not null;default:''" json:"repeat_rule"`
	RepeatID     *uuid.UUID `gorm:"type:uuid" json:"repeat_id,omitempty"`
	Status       int16      `gorm:"type:smallint;not null;default:0" json:"status"`
	MeetingID    *uuid.UUID `gorm:"type:uuid" json:"meeting_id,omitempty"`
	Version      int        `gorm:"not null;default:0" json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (Event) TableName() string { return "schedule_events" }

// ShareTargetsIDs 反序列化 share_targets JSON 为 UUID 切片
func (e *Event) ShareTargetsIDs() []uuid.UUID {
	var ids []uuid.UUID
	if e.ShareTargets == "" {
		return ids
	}
	_ = json.Unmarshal([]byte(e.ShareTargets), &ids)
	return ids
}

// SetShareTargetsIDs 将 UUID 切片序列化写入 share_targets
func (e *Event) SetShareTargetsIDs(ids []uuid.UUID) {
	if ids == nil {
		e.ShareTargets = "[]"
		return
	}
	b, _ := json.Marshal(ids)
	e.ShareTargets = string(b)
}

// WithShareTargetsIDs 便捷构造：返回设置好 share_targets 的副本指针
func (e *Event) WithShareTargetsIDs(ids []uuid.UUID) *Event {
	e.SetShareTargetsIDs(ids)
	return e
}

// EventWithDetails 日程详情视图，附带参与人列表和提醒数量
type EventWithDetails struct {
	Event
	AttendeeCount int64 `gorm:"column:attendee_count" json:"attendee_count"`
	ReminderCount int64 `gorm:"column:reminder_count" json:"reminder_count"`
}

// ============================================================
// schedule_event_attendees：日程参与人关联
// ============================================================

// 参与人角色
const (
	AttendeeRoleOrganizer int16 = 1 // 组织者
	AttendeeRoleRequired  int16 = 2 // 必须参与
	AttendeeRoleOptional  int16 = 3 // 可选参与
)

// 参与人响应状态
const (
	ResponsePending  int16 = 0 // 待回复
	ResponseAccepted int16 = 1 // 接受
	ResponseDeclined int16 = 2 // 拒绝
	ResponseTentative int16 = 3 // 待定
)

// Attendee 日程参与人关联表
type Attendee struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	EventID        uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_event_user" json:"event_id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_event_user" json:"user_id"`
	Role           int16      `gorm:"type:smallint;not null;default:1" json:"role"`
	ResponseStatus int16      `gorm:"type:smallint;not null;default:0" json:"response_status"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// TableName 指定表名
func (Attendee) TableName() string { return "schedule_event_attendees" }

// ============================================================
// schedule_reminders：日程提醒
// ============================================================

// 提醒状态
const (
	ReminderStatusPending  int16 = 0 // 待触发
	ReminderStatusFired    int16 = 1 // 已触发
	ReminderStatusCanceled int16 = 2 // 已取消
	ReminderStatusSnoozed  int16 = 3 // 已推迟
)

// 提醒方式
const (
	ReminderMethodApp   = "app"   // 应用内通知
	ReminderMethodEmail = "email" // 邮件
	ReminderMethodSMS   = "sms"   // 短信
)

// Reminder 日程提醒表
type Reminder struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	EventID       uuid.UUID  `gorm:"type:uuid;not null" json:"event_id"`
	UserID        uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	RemindOffset  int        `gorm:"not null;default:15" json:"remind_offset"`
	RemindTime    time.Time  `gorm:"not null" json:"remind_time"`
	RemindMethod  string     `gorm:"type:varchar(20);not null;default:app" json:"remind_method"`
	Status        int16      `gorm:"type:smallint;not null;default:0" json:"status"`
	FiredAt       *time.Time `json:"fired_at,omitempty"`
	SnoozeCount   int        `gorm:"not null;default:0" json:"snooze_count"`
	SnoozeMinutes int        `gorm:"not null;default:0" json:"snooze_minutes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (Reminder) TableName() string { return "schedule_reminders" }

// ============================================================
// DataScope 相关：供 middleware.DataScopeMiddleware 过滤查询
// 与 meeting 模块的 Viewer 模式对齐
// ============================================================

// Viewer 表示当前请求的查看者，供 service 层权限过滤使用
type Viewer struct {
	ID         uuid.UUID
	Scope      string // data scope: all / dept / self
	Manage     bool
	ManageScope   string
	CanManage     bool
	UpdateScope   string
	CanUpdate     bool
	DeleteScope   string
	CanDelete     bool
}

type contextKey string

const viewerContextKey contextKey = "schedule_viewer"

// WithViewer 将 Viewer 注入 context
func WithViewer(ctx context.Context, v Viewer) context.Context {
	return context.WithValue(ctx, viewerContextKey, v)
}

// ViewerFromContext 从 context 取出 Viewer；若无则返回零值
func ViewerFromContext(ctx context.Context) Viewer {
	if v, ok := ctx.Value(viewerContextKey).(Viewer); ok {
		return v
	}
	return Viewer{}
}
