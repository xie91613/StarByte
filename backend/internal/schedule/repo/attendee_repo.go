package repo

import (
	"context"

	"github.com/Yogdunana/StarByte/backend/internal/schedule/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AttendeeRepo 日程参与人仓储接口
type AttendeeRepo interface {
	Create(ctx context.Context, a *model.Attendee) error
	BatchCreate(ctx context.Context, attendees []model.Attendee) error
	Update(ctx context.Context, a *model.Attendee) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Attendee, error)
	GetByEventAndUser(ctx context.Context, eventID, userID uuid.UUID) (*model.Attendee, error)
	ListByEvent(ctx context.Context, eventID uuid.UUID) ([]model.Attendee, error)
	ListByUser(ctx context.Context, userID uuid.UUID, req *AttendeeListQuery) ([]model.Attendee, int64, error)
	UpdateResponse(ctx context.Context, id uuid.UUID, status int16) error
	RemoveByEvent(ctx context.Context, eventID uuid.UUID, excludeRoles ...int16) error
	CountByEvent(ctx context.Context, eventID uuid.UUID) (int64, error)
}

// AttendeeListQuery 参与人列表查询
type AttendeeListQuery struct {
	ResponseStatus *int16
	Role           *int16
	Page           int
	PageSize       int
}

type attendeeRepo struct{ db *gorm.DB }

// NewAttendeeRepo 构造函数
func NewAttendeeRepo(db *gorm.DB) AttendeeRepo { return &attendeeRepo{db: db} }

func (r *attendeeRepo) Create(ctx context.Context, a *model.Attendee) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *attendeeRepo) BatchCreate(ctx context.Context, attendees []model.Attendee) error {
	if len(attendees) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(&attendees, 100).Error
}

func (r *attendeeRepo) Update(ctx context.Context, a *model.Attendee) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *attendeeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Attendee{}, "id = ?", id).Error
}

func (r *attendeeRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Attendee, error) {
	var a model.Attendee
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *attendeeRepo) GetByEventAndUser(ctx context.Context, eventID, userID uuid.UUID) (*model.Attendee, error) {
	var a model.Attendee
	err := r.db.WithContext(ctx).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		First(&a).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *attendeeRepo) ListByEvent(ctx context.Context, eventID uuid.UUID) ([]model.Attendee, error) {
	var rows []model.Attendee
	err := r.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("role ASC, created_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *attendeeRepo) ListByUser(ctx context.Context, userID uuid.UUID, req *AttendeeListQuery) ([]model.Attendee, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Attendee{}).Where("user_id = ?", userID)
	if req != nil {
		if req.ResponseStatus != nil {
			q = q.Where("response_status = ?", *req.ResponseStatus)
		}
		if req.Role != nil {
			q = q.Where("role = ?", *req.Role)
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := normalizePage(req.Page, req.PageSize)
	var rows []model.Attendee
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (r *attendeeRepo) UpdateResponse(ctx context.Context, id uuid.UUID, status int16) error {
	return r.db.WithContext(ctx).
		Model(&model.Attendee{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"response_status": status,
			"responded_at":    gorm.Expr("CURRENT_TIMESTAMP"),
		}).Error
}

func (r *attendeeRepo) RemoveByEvent(ctx context.Context, eventID uuid.UUID, excludeRoles ...int16) error {
	q := r.db.WithContext(ctx).Where("event_id = ?", eventID)
	if len(excludeRoles) > 0 {
		q = q.Where("role NOT IN ?", excludeRoles)
	}
	return q.Delete(&model.Attendee{}).Error
}

func (r *attendeeRepo) CountByEvent(ctx context.Context, eventID uuid.UUID) (int64, error) {
	var count int64
	return count, r.db.WithContext(ctx).
		Model(&model.Attendee{}).
		Where("event_id = ?", eventID).
		Count(&count).Error
}
