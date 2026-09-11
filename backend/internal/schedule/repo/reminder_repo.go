package repo

import (
	"context"
	"time"

	"github.com/Yogdunana/StarByte/backend/internal/schedule/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReminderRepo 日程提醒仓储接口
type ReminderRepo interface {
	Create(ctx context.Context, r *model.Reminder) error
	BatchCreate(ctx context.Context, reminders []model.Reminder) error
	Update(ctx context.Context, r *model.Reminder) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Reminder, error)
	ListByEvent(ctx context.Context, eventID uuid.UUID) ([]model.Reminder, error)
	ListByUser(ctx context.Context, userID uuid.UUID, req *ReminderListQuery) ([]model.Reminder, int64, error)
	ListPendingToFire(ctx context.Context, before time.Time, limit int) ([]model.Reminder, error)
	MarkTriggered(ctx context.Context, id uuid.UUID) error
	Snooze(ctx context.Context, id uuid.UUID, snoozeMinutes int, newFireTime time.Time) (*model.Reminder, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	CancelByEvent(ctx context.Context, eventID uuid.UUID) error
}

// ReminderListQuery 提醒列表查询
type ReminderListQuery struct {
	Status     *int16
	Method     string
	EventID    *uuid.UUID
	Page       int
	PageSize   int
}

type reminderRepo struct{ db *gorm.DB }

// NewReminderRepo 构造函数
func NewReminderRepo(db *gorm.DB) ReminderRepo { return &reminderRepo{db: db} }

func (r *reminderRepo) Create(ctx context.Context, rm *model.Reminder) error {
	return r.db.WithContext(ctx).Create(rm).Error
}

func (r *reminderRepo) BatchCreate(ctx context.Context, reminders []model.Reminder) error {
	if len(reminders) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(&reminders, 100).Error
}

func (r *reminderRepo) Update(ctx context.Context, rm *model.Reminder) error {
	return r.db.WithContext(ctx).Save(rm).Error
}

func (r *reminderRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Reminder{}, "id = ?", id).Error
}

func (r *reminderRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Reminder, error) {
	var rm model.Reminder
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&rm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *reminderRepo) ListByEvent(ctx context.Context, eventID uuid.UUID) ([]model.Reminder, error) {
	var rows []model.Reminder
	err := r.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("remind_time ASC").
		Find(&rows).Error
	return rows, err
}

func (r *reminderRepo) ListByUser(ctx context.Context, userID uuid.UUID, req *ReminderListQuery) ([]model.Reminder, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Reminder{}).Where("user_id = ?", userID)
	if req != nil {
		if req.Status != nil {
			q = q.Where("status = ?", *req.Status)
		}
		if req.Method != "" {
			q = q.Where("remind_method = ?", req.Method)
		}
		if req.EventID != nil {
			q = q.Where("event_id = ?", *req.EventID)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(req.Page, req.PageSize)
	var rows []model.Reminder
	err := q.Order("remind_time DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

// ListPendingToFire 调度器扫描：查询触发时间 ≤ before 且状态为 pending/snoozed 的提醒
func (r *reminderRepo) ListPendingToFire(ctx context.Context, before time.Time, limit int) ([]model.Reminder, error) {
	var rows []model.Reminder
	err := r.db.WithContext(ctx).
		Where("remind_time <= ? AND status IN ?", before, []int16{model.ReminderStatusPending, model.ReminderStatusSnoozed}).
		Order("remind_time ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// MarkTriggered 标记提醒为已触发，仅在 pending/snoozed 状态时生效（乐观防重复触发）
func (r *reminderRepo) MarkTriggered(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&model.Reminder{}).
		Where("id = ? AND status IN ?", id, []int16{model.ReminderStatusPending, model.ReminderStatusSnoozed}).
		Updates(map[string]interface{}{
			"status":    model.ReminderStatusFired,
			"fired_at":  gorm.Expr("CURRENT_TIMESTAMP"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // 已被其他 worker 处理过
	}
	return nil
}

// Snooze 推迟提醒：重算触发时间，更新 snooze_count
func (r *reminderRepo) Snooze(ctx context.Context, id uuid.UUID, snoozeMinutes int, newFireTime time.Time) (*model.Reminder, error) {
	res := r.db.WithContext(ctx).
		Model(&model.Reminder{}).
		Where("id = ? AND status IN ?", id, []int16{model.ReminderStatusPending, model.ReminderStatusSnoozed}).
		Updates(map[string]interface{}{
			"status":          model.ReminderStatusSnoozed,
			"remind_time":     newFireTime,
			"snooze_count":    gorm.Expr("snooze_count + 1"),
			"snooze_minutes":  snoozeMinutes,
			"updated_at":      gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.GetByID(ctx, id)
}

// Cancel 取消单个提醒
func (r *reminderRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Reminder{}).
		Where("id = ? AND status IN ?", id, []int16{model.ReminderStatusPending, model.ReminderStatusSnoozed}).
		Update("status", model.ReminderStatusCanceled).Error
}

// CancelByEvent 事件被取消时，批量取消其所有 pending/snoozed 提醒
func (r *reminderRepo) CancelByEvent(ctx context.Context, eventID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Reminder{}).
		Where("event_id = ? AND status IN ?", eventID, []int16{model.ReminderStatusPending, model.ReminderStatusSnoozed}).
		Update("status", model.ReminderStatusCanceled).Error
}
