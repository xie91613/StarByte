package repo

import (
	"context"
	"strings"
	"time"

	"github.com/Yogdunana/StarByte/backend/internal/schedule/model"
	rbacModel "github.com/Yogdunana/StarByte/backend/internal/rbac/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EventRepo 日程事件仓储接口
type EventRepo interface {
	Create(ctx context.Context, e *model.Event) error
	Update(ctx context.Context, e *model.Event) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Event, error)
	GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.EventWithDetails, error)
	List(ctx context.Context, req *EventListQuery, scope *rbacModel.DataScopeCondition) ([]model.EventWithDetails, int64, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, req *EventListQuery, scope *rbacModel.DataScopeCondition) ([]model.EventWithDetails, int64, error)
	ListByAttendee(ctx context.Context, userID uuid.UUID, req *EventListQuery) ([]model.EventWithDetails, int64, error)
	ListByTimeRange(ctx context.Context, userID uuid.UUID, from, to time.Time, req *EventListQuery) ([]model.Event, error)
	GetByMeetingID(ctx context.Context, meetingID uuid.UUID) (*model.Event, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status int16, prevVersion int) error
	UpdateMeetingID(ctx context.Context, id uuid.UUID, meetingID *uuid.UUID, prevVersion int) error
}

// EventListQuery 日程列表查询条件
type EventListQuery struct {
	Keyword      string
	Status       *int16
	CalendarType string
	OwnerID      *uuid.UUID
	StartDate    string
	EndDate      string
	Page         int
	PageSize     int
}

type eventRepo struct{ db *gorm.DB }

// NewEventRepo 构造函数
func NewEventRepo(db *gorm.DB) EventRepo { return &eventRepo{db: db} }

func (r *eventRepo) Create(ctx context.Context, e *model.Event) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *eventRepo) Update(ctx context.Context, e *model.Event) error {
	res := r.db.WithContext(ctx).
		Model(&model.Event{}).
		Where("id = ? AND version = ?", e.ID, e.Version).
		Updates(map[string]interface{}{
			"title":         e.Title,
			"description":   e.Description,
			"calendar_type": e.CalendarType,
			"start_time":    e.StartTime,
			"end_time":      e.EndTime,
			"location":      e.Location,
			"online_link":   e.OnlineLink,
			"visibility":    e.Visibility,
			"share_targets": e.ShareTargets,
			"repeat_rule":   e.RepeatRule,
			"repeat_id":     e.RepeatID,
			"status":        e.Status,
			"meeting_id":    e.MeetingID,
			"version":       gorm.Expr("version + 1"),
			"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // 乐观锁冲突
	}
	return nil
}

func (r *eventRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Event{}, "id = ?", id).Error
}

func (r *eventRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Event, error) {
	var e model.Event
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// eventDetailQuery 构建带详情（参与人数、提醒数）的查询
func (r *eventRepo) eventDetailQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("schedule_events AS e").
		Select(`e.*,
			(SELECT COUNT(*) FROM schedule_event_attendees a WHERE a.event_id = e.id) AS attendee_count,
			(SELECT COUNT(*) FROM schedule_reminders r WHERE r.event_id = e.id) AS reminder_count`)
}

func (r *eventRepo) GetByIDWithDetails(ctx context.Context, id uuid.UUID) (*model.EventWithDetails, error) {
	var row model.EventWithDetails
	err := r.eventDetailQuery(ctx).Where("e.id = ?", id).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *eventRepo) List(ctx context.Context, req *EventListQuery, scope *rbacModel.DataScopeCondition) ([]model.EventWithDetails, int64, error) {
	q := r.eventDetailQuery(ctx)
	q = applyScope(q, scope)
	q = applyEventFilters(q, req)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(req.Page, req.PageSize)
	var rows []model.EventWithDetails
	err := q.Order("e.start_time DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (r *eventRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID, req *EventListQuery, scope *rbacModel.DataScopeCondition) ([]model.EventWithDetails, int64, error) {
	q := r.eventDetailQuery(ctx).Where("e.owner_id = ?", ownerID)
	q = applyScope(q, scope)
	q = applyEventFilters(q, req)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(req.Page, req.PageSize)
	var rows []model.EventWithDetails
	err := q.Order("e.start_time DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (r *eventRepo) ListByAttendee(ctx context.Context, userID uuid.UUID, req *EventListQuery) ([]model.EventWithDetails, int64, error) {
	q := r.eventDetailQuery(ctx).
		Joins("INNER JOIN schedule_event_attendees a ON a.event_id = e.id").
		Where("a.user_id = ?", userID)
	q = applyEventFilters(q, req)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(req.Page, req.PageSize)
	var rows []model.EventWithDetails
	err := q.Order("e.start_time DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (r *eventRepo) ListByTimeRange(ctx context.Context, userID uuid.UUID, from, to time.Time, req *EventListQuery) ([]model.Event, error) {
	q := r.db.WithContext(ctx).Model(&model.Event{}).
		Where("owner_id = ? AND status <> ? AND start_time <= ? AND end_time >= ?",
			userID, model.EventStatusCanceled, to, from)
	q = applyEventFilters(q, req)

	var rows []model.Event
	return rows, q.Order("start_time ASC").Find(&rows).Error
}

func (r *eventRepo) GetByMeetingID(ctx context.Context, meetingID uuid.UUID) (*model.Event, error) {
	var e model.Event
	err := r.db.WithContext(ctx).Where("meeting_id = ?", meetingID).First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *eventRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status int16, prevVersion int) error {
	res := r.db.WithContext(ctx).
		Model(&model.Event{}).
		Where("id = ? AND version = ?", id, prevVersion).
		Updates(map[string]interface{}{
			"status":     status,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *eventRepo) UpdateMeetingID(ctx context.Context, id uuid.UUID, meetingID *uuid.UUID, prevVersion int) error {
	res := r.db.WithContext(ctx).
		Model(&model.Event{}).
		Where("id = ? AND version = ?", id, prevVersion).
		Updates(map[string]interface{}{
			"meeting_id": meetingID,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// applyEventFilters 公共过滤
func applyEventFilters(q *gorm.DB, req *EventListQuery) *gorm.DB {
	if req == nil {
		return q
	}
	if req.Status != nil {
		q = q.Where("e.status = ?", *req.Status)
	}
	if req.CalendarType != "" {
		q = q.Where("e.calendar_type = ?", req.CalendarType)
	}
	if req.OwnerID != nil {
		q = q.Where("e.owner_id = ?", *req.OwnerID)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("e.title ILIKE ? OR e.description ILIKE ? OR e.location ILIKE ?", like, like, like)
	}
	if req.StartDate != "" {
		q = q.Where("e.start_time >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		q = q.Where("e.end_time <= ?", req.EndDate+" 23:59:59")
	}
	return q
}

// applyScope 应用数据权限过滤（与 meeting repo 共享的 DataScopeCondition 逻辑）
func applyScope(q *gorm.DB, scope *rbacModel.DataScopeCondition) *gorm.DB {
	if scope == nil || scope.Scope == "" || scope.Scope == "all" {
		return q
	}
	switch scope.Scope {
	case "self":
		q = q.Where("e.owner_id IN (?) OR e.owner_id = ?", scope.UserIDs, scope.UserIDs[0])
	case "dept":
		// 部门权限：owner_id 在部门用户列表中
		q = q.Where("e.owner_id IN (?)", scope.UserIDs)
	}
	return q
}

// normalizePage 分页公共函数
func normalizePage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}
