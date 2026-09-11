package service

import (
	"context"
	"time"

	"schedDto "github.com/Yogdunana/StarByte/backend/internal/schedule/dto"
	"schedModel "github.com/Yogdunana/StarByte/backend/internal/schedule/model"
	"schedRepo "github.com/Yogdunana/StarByte/backend/internal/schedule/repo"
	rbacModel "github.com/Yogdunana/StarByte/backend/internal/rbac/model"
	"github.com/Yogdunana/StarByte/backend/pkg/response"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================
// Schedule 模块错误码（28000-28999）
// 注册到 pkg/response/error_codes.go 的 ModuleRanges 映射
// ============================================================

const (
	// 通用 28001-28009
	ErrCodeEventParamInvalid  = 28001 // 参数非法
	ErrCodeEventNoAccess      = 28002 // 无权限操作
	ErrCodeEventNotFound      = 28003 // 日程不存在
	ErrCodeEventConcurrent    = 28004 // 并发冲突（乐观锁）
	ErrCodeEventStatusInvalid = 28005 // 状态转换非法
	ErrCodeEventEndBeforeStart = 28006 // 结束时间早于开始时间
	ErrCodeEventAlreadyLinked = 28007 // 日程已关联会议

	// 参与人 28010-28019
	ErrCodeAttendeeNotFound  = 28010 // 参与人不存在
	ErrCodeAttendeeDuplicate = 28011 // 重复添加
	ErrCodeAttendeeOrganizer = 28012 // 组织者不可移除

	// 提醒 28020-28029
	ErrCodeReminderNotFound  = 28020 // 提醒不存在
	ErrCodeReminderAlreadyFired = 28021 // 已触发不可重复触发
	ErrCodeReminderSnoozeExceed = 28022 // 推迟超限
)

// ============================================================
// ReminderDispatcher 提醒发送接口（依赖倒置，生产可替换为 MQ）
// ============================================================

type ReminderDispatcher interface {
	Dispatch(ctx context.Context, userIDs []uuid.UUID, eventTitle string, remindMethod string, eventStart time.Time) error
}

// ============================================================
// ScheduleEventService 日程事件服务接口
// ============================================================

type ScheduleEventService interface {
	Create(ctx context.Context, operator uuid.UUID, req *schedDto.CreateEventRequest) (*schedDto.EventResponse, error)
	GetByID(ctx context.Context, id uuid.UUID, scope *rbacModel.DataScopeCondition) (*schedDto.EventResponse, error)
	List(ctx context.Context, viewer uuid.UUID, req *schedDto.EventListRequest, scope *rbacModel.DataScopeCondition) ([]*schedDto.EventResponse, int64, error)
	ListByOwner(ctx context.Context, ownerID, viewer uuid.UUID, req *schedDto.EventListRequest, scope *rbacModel.DataScopeCondition) ([]*schedDto.EventResponse, int64, error)
	ListByAttendee(ctx context.Context, userID uuid.UUID, req *schedDto.EventListRequest) ([]*schedDto.EventResponse, int64, error)
	ListByTimeRange(ctx context.Context, userID uuid.UUID, req *schedDto.EventTimeRangeRequest) ([]*schedDto.EventResponse, error)
	Update(ctx context.Context, operator uuid.UUID, id uuid.UUID, req *schedDto.UpdateEventRequest) (*schedDto.EventResponse, error)
	Delete(ctx context.Context, operator uuid.UUID, id uuid.UUID) error
	UpdateStatus(ctx context.Context, operator uuid.UUID, id uuid.UUID, req *schedDto.UpdateEventStatusRequest) (*schedDto.EventResponse, error)
	LinkMeeting(ctx context.Context, operator uuid.UUID, eventID, meetingID uuid.UUID, prevVersion int) (*schedDto.EventResponse, error)
	UnlinkMeeting(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, prevVersion int) (*schedDto.EventResponse, error)
}

// ============================================================
// ScheduleAttendeeService 日程参与人服务接口
// ============================================================

type ScheduleAttendeeService interface {
	Add(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, userID uuid.UUID, role int16) (*schedDto.AttendeeResponse, error)
	BatchAdd(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, req *schedDto.AddAttendeesRequest) ([]*schedDto.AttendeeResponse, error)
	Remove(ctx context.Context, operator uuid.UUID, eventID, attendeeID uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*schedDto.AttendeeResponse, error)
	ListByEvent(ctx context.Context, viewer uuid.UUID, eventID uuid.UUID) ([]*schedDto.AttendeeResponse, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*schedDto.AttendeeResponse, int64, error)
	Respond(ctx context.Context, userID, attendeeID uuid.UUID, status int16) (*schedDto.AttendeeResponse, error)
}

// ============================================================
// ScheduleReminderService 日程提醒服务接口
// ============================================================

type ScheduleReminderService interface {
	Create(ctx context.Context, userID uuid.UUID, req *schedDto.CreateReminderRequest) (*schedDto.ReminderResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*schedDto.ReminderResponse, error)
	ListByEvent(ctx context.Context, viewer uuid.UUID, eventID uuid.UUID) ([]*schedDto.ReminderResponse, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*schedDto.ReminderResponse, int64, error)
	Cancel(ctx context.Context, userID, reminderID uuid.UUID) error
	Snooze(ctx context.Context, userID, reminderID uuid.UUID, req *schedDto.SnoozeReminderRequest) (*schedDto.ReminderResponse, error)
	FirePending(ctx context.Context, limit int) (int, error)
}

// ============================================================
// 实现：eventService
// ============================================================

type eventService struct {
	db       *gorm.DB
	events   schedRepo.EventRepo
	attendees schedRepo.AttendeeRepo
	reminders schedRepo.ReminderRepo
	dispatch ReminderDispatcher
}

// NewScheduleEventService 构造函数
func NewScheduleEventService(
	db *gorm.DB,
	events schedRepo.EventRepo,
	attendees schedRepo.AttendeeRepo,
	reminders schedRepo.ReminderRepo,
	dispatch ReminderDispatcher,
) ScheduleEventService {
	return &eventService{
		db: db, events: events, attendees: attendees, reminders: reminders, dispatch: dispatch,
	}
}

func (s *eventService) Create(ctx context.Context, operator uuid.UUID, req *schedDto.CreateEventRequest) (*schedDto.EventResponse, error) {
	// 参数校验
	if req.EndTime.Before(req.StartTime) {
		return nil, response.NewAppError(ErrCodeEventEndBeforeStart, "结束时间必须晚于开始时间")
	}

	e := &schedModel.Event{
		ID:           uuid.New(),
		Title:        req.Title,
		Description:  req.Description,
		CalendarType: defaultString(req.CalendarType, "personal"),
		OwnerID:      operator,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		Location:     req.Location,
		OnlineLink:   req.OnlineLink,
		Visibility:   defaultString(req.Visibility, "private"),
		RepeatRule:   req.RepeatRule,
		Status:       schedModel.EventStatusPublished,
	}
	e.SetShareTargetsIDs(req.ShareTargets)

	// 事务：写 event → attendees → reminders
	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(e).Error; err != nil {
			return err
		}
		// 批量写参与人
		for _, in := range req.Attendees {
			a := &schedModel.Attendee{
				ID:      uuid.New(),
				EventID: e.ID,
				UserID:  in.UserID,
				Role:    schedModel.AttendeeRoleRequired,
			}
			if in.Role > 0 {
				a.Role = in.Role
			}
			if err := tx.WithContext(ctx).Create(a).Error; err != nil {
				return err
			}
		}
		// 批量写提醒
		for _, rm := range req.Reminders {
			rem := &schedModel.Reminder{
				ID:           uuid.New(),
				EventID:      e.ID,
				UserID:       operator,
				RemindOffset: rm.RemindOffset,
				RemindMethod: defaultString(rm.RemindMethod, "app"),
				RemindTime:   req.StartTime.Add(-time.Duration(rm.RemindOffset) * time.Minute),
				Status:       schedModel.ReminderStatusPending,
			}
			if err := tx.WithContext(ctx).Create(rem).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, response.NewAppError(response.CodeInternalError, "创建日程失败")
	}

	// 返回详情
	det, err := s.events.GetByIDWithDetails(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	return eventToResponse(det), nil
}

func (s *eventService) GetByID(ctx context.Context, id uuid.UUID, scope *rbacModel.DataScopeCondition) (*schedDto.EventResponse, error) {
	det, err := s.events.GetByIDWithDetails(ctx, id)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if det == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	return eventToResponse(det), nil
}

func (s *eventService) List(ctx context.Context, viewer uuid.UUID, req *schedDto.EventListRequest, scope *rbacModel.DataScopeCondition) ([]*schedDto.EventResponse, int64, error) {
	q := &schedRepo.EventListQuery{
		Keyword:      req.Keyword,
		Status:       req.Status,
		CalendarType: req.CalendarType,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Page:         req.Page,
		PageSize:     req.PageSize,
	}
	rows, total, err := s.events.List(ctx, q, scope)
	if err != nil {
		return nil, 0, response.NewAppError(response.CodeInternalError, "查询日程列表失败")
	}
	return eventsToResponses(rows), total, nil
}

func (s *eventService) ListByOwner(ctx context.Context, ownerID, viewer uuid.UUID, req *schedDto.EventListRequest, scope *rbacModel.DataScopeCondition) ([]*schedDto.EventResponse, int64, error) {
	q := &schedRepo.EventListQuery{
		Keyword:      req.Keyword,
		Status:       req.Status,
		CalendarType: req.CalendarType,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Page:         req.Page,
		PageSize:     req.PageSize,
	}
	rows, total, err := s.events.ListByOwner(ctx, ownerID, q, scope)
	if err != nil {
		return nil, 0, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	return eventsToResponses(rows), total, nil
}

func (s *eventService) ListByAttendee(ctx context.Context, userID uuid.UUID, req *schedDto.EventListRequest) ([]*schedDto.EventResponse, int64, error) {
	q := &schedRepo.EventListQuery{
		Keyword:   req.Keyword,
		Status:    req.Status,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}
	rows, total, err := s.events.ListByAttendee(ctx, userID, q)
	if err != nil {
		return nil, 0, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	return eventsToResponses(rows), total, nil
}

func (s *eventService) ListByTimeRange(ctx context.Context, userID uuid.UUID, req *schedDto.EventTimeRangeRequest) ([]*schedDto.EventResponse, error) {
	if req.To.Before(req.From) {
		return nil, response.NewAppError(ErrCodeEventParamInvalid, "时间区间非法")
	}
	q := &schedRepo.EventListQuery{}
	rows, err := s.events.ListByTimeRange(ctx, userID, req.From, req.To, q)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	var resp []*schedDto.EventResponse
	for _, r := range rows {
		e := r
		resp = append(resp, eventBasicToResponse(&e))
	}
	return resp, nil
}

func (s *eventService) Update(ctx context.Context, operator uuid.UUID, id uuid.UUID, req *schedDto.UpdateEventRequest) (*schedDto.EventResponse, error) {
	e, err := s.events.GetByID(ctx, id)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权修改此日程")
	}
	if req.EndTime.Before(req.StartTime) {
		return nil, response.NewAppError(ErrCodeEventEndBeforeStart, "结束时间必须晚于开始时间")
	}
	if e.Version != req.Version {
		return nil, response.NewAppError(ErrCodeEventConcurrent, "日程已被他人修改，请刷新后重试")
	}
	e.Title = req.Title
	e.Description = req.Description
	e.CalendarType = defaultString(req.CalendarType, e.CalendarType)
	e.StartTime = req.StartTime
	e.EndTime = req.EndTime
	e.Location = req.Location
	e.OnlineLink = req.OnlineLink
	e.Visibility = defaultString(req.Visibility, e.Visibility)
	e.RepeatRule = req.RepeatRule
	e.SetShareTargetsIDs(req.ShareTargets)
	if err := s.events.Update(ctx, e); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, response.NewAppError(ErrCodeEventConcurrent, "日程已被他人修改，请刷新后重试")
		}
		return nil, response.NewAppError(response.CodeInternalError, "更新日程失败")
	}
	det, err := s.events.GetByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	return eventToResponse(det), nil
}

func (s *eventService) Delete(ctx context.Context, operator uuid.UUID, id uuid.UUID) error {
	e, err := s.events.GetByID(ctx, id)
	if err != nil {
		return response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return response.NewAppError(ErrCodeEventNoAccess, "无权删除此日程")
	}
	// 事务：删 event + 级联取消 reminders + 级联删 attendees
	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.reminders.CancelByEvent(ctx, id); err != nil {
			return err
		}
		return s.events.Delete(ctx, id)
	})
	if txErr != nil {
		return response.NewAppError(response.CodeInternalError, "删除日程失败")
	}
	return nil
}

func (s *eventService) UpdateStatus(ctx context.Context, operator uuid.UUID, id uuid.UUID, req *schedDto.UpdateEventStatusRequest) (*schedDto.EventResponse, error) {
	e, err := s.events.GetByID(ctx, id)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	// 状态机校验
	if !isStatusTransitionAllowed(e.Status, req.Status) {
		return nil, response.NewAppError(ErrCodeEventStatusInvalid, "状态转换非法")
	}
	if err := s.events.UpdateStatus(ctx, id, req.Status, e.Version); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, response.NewAppError(ErrCodeEventConcurrent, "日程已被他人修改")
		}
		return nil, response.NewAppError(response.CodeInternalError, "更新状态失败")
	}
	det, err := s.events.GetByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	return eventToResponse(det), nil
}

func (s *eventService) LinkMeeting(ctx context.Context, operator uuid.UUID, eventID, meetingID uuid.UUID, prevVersion int) (*schedDto.EventResponse, error) {
	e, err := s.events.GetByID(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	if e.MeetingID != nil {
		return nil, response.NewAppError(ErrCodeEventAlreadyLinked, "日程已关联其他会议")
	}
	if err := s.events.UpdateMeetingID(ctx, eventID, &meetingID, prevVersion); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, response.NewAppError(ErrCodeEventConcurrent, "日程已被他人修改")
		}
		return nil, response.NewAppError(response.CodeInternalError, "关联会议失败")
	}
	det, err := s.events.GetByIDWithDetails(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return eventToResponse(det), nil
}

func (s *eventService) UnlinkMeeting(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, prevVersion int) (*schedDto.EventResponse, error) {
	e, err := s.events.GetByID(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	nilMeeting := (*uuid.UUID)(nil)
	if err := s.events.UpdateMeetingID(ctx, eventID, nilMeeting, prevVersion); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, response.NewAppError(ErrCodeEventConcurrent, "日程已被他人修改")
		}
		return nil, response.NewAppError(response.CodeInternalError, "解除会议失败")
	}
	det, err := s.events.GetByIDWithDetails(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return eventToResponse(det), nil
}

// isStatusTransitionAllowed 状态机：draft→published→completed；published→canceled；draft→canceled
func isStatusTransitionAllowed(from, to int16) bool {
	allowed := map[int16][]int16{
		schedModel.EventStatusDraft:     {schedModel.EventStatusPublished, schedModel.EventStatusCanceled},
		schedModel.EventStatusPublished: {schedModel.EventStatusCompleted, schedModel.EventStatusCanceled},
		schedModel.EventStatusCanceled:  {},
		schedModel.EventStatusCompleted: {},
	}
	for _, allowedTo := range allowed[from] {
		if allowedTo == to {
			return true
		}
	}
	return false
}

// ============================================================
// 实现：attendeeService
// ============================================================

type attendeeService struct {
	db        *gorm.DB
	events    schedRepo.EventRepo
	attendees schedRepo.AttendeeRepo
}

// NewScheduleAttendeeService 构造函数
func NewScheduleAttendeeService(db *gorm.DB, events schedRepo.EventRepo, attendees schedRepo.AttendeeRepo) ScheduleAttendeeService {
	return &attendeeService{db: db, events: events, attendees: attendees}
}

func (s *attendeeService) Add(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, userID uuid.UUID, role int16) (*schedDto.AttendeeResponse, error) {
	e, err := s.events.GetByID(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	// 唯一性校验
	exist, err := s.attendees.GetByEventAndUser(ctx, eventID, userID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	if exist != nil {
		return nil, response.NewAppError(ErrCodeAttendeeDuplicate, "用户已是该日程参与人")
	}
	a := &schedModel.Attendee{
		ID:      uuid.New(),
		EventID: eventID,
		UserID:  userID,
		Role:    defaultInt16(role, schedModel.AttendeeRoleRequired),
	}
	if err := s.attendees.Create(ctx, a); err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "添加参与人失败")
	}
	return attendeeToResponse(a), nil
}

func (s *attendeeService) BatchAdd(ctx context.Context, operator uuid.UUID, eventID uuid.UUID, req *schedDto.AddAttendeesRequest) ([]*schedDto.AttendeeResponse, error) {
	e, err := s.events.GetByID(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	// 事务内批量添加 + 去重
	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, uid := range req.UserIDs {
			exist, _ := s.attendees.GetByEventAndUser(ctx, eventID, uid)
			if exist != nil {
				continue // 跳过已存在
			}
			a := &schedModel.Attendee{
				ID:      uuid.New(),
				EventID: eventID,
				UserID:  uid,
				Role:    defaultInt16(req.Role, schedModel.AttendeeRoleRequired),
			}
			if err := tx.WithContext(ctx).Create(a).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, response.NewAppError(response.CodeInternalError, "批量添加参与人失败")
	}
	list, _ := s.attendees.ListByEvent(ctx, eventID)
	var resp []*schedDto.AttendeeResponse
	for _, a := range list {
		resp = append(resp, attendeeToResponse(&a))
	}
	return resp, nil
}

func (s *attendeeService) Remove(ctx context.Context, operator uuid.UUID, eventID, attendeeID uuid.UUID) error {
	e, err := s.events.GetByID(ctx, eventID)
	if err != nil {
		return response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	if e.OwnerID != operator {
		return response.NewAppError(ErrCodeEventNoAccess, "无权操作此日程")
	}
	a, err := s.attendees.GetByID(ctx, attendeeID)
	if err != nil {
		return response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	if a == nil {
		return response.NewAppError(ErrCodeAttendeeNotFound, "参与人不存在")
	}
	if a.Role == schedModel.AttendeeRoleOrganizer {
		return response.NewAppError(ErrCodeAttendeeOrganizer, "组织者不可移除")
	}
	return s.attendees.Delete(ctx, attendeeID)
}

func (s *attendeeService) GetByID(ctx context.Context, id uuid.UUID) (*schedDto.AttendeeResponse, error) {
	a, err := s.attendees.GetByID(ctx, id)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	if a == nil {
		return nil, response.NewAppError(ErrCodeAttendeeNotFound, "参与人不存在")
	}
	return attendeeToResponse(a), nil
}

func (s *attendeeService) ListByEvent(ctx context.Context, viewer uuid.UUID, eventID uuid.UUID) ([]*schedDto.AttendeeResponse, error) {
	a, err := s.attendees.ListByEvent(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	var resp []*schedDto.AttendeeResponse
	for _, x := range a {
		resp = append(resp, attendeeToResponse(&x))
	}
	return resp, nil
}

func (s *attendeeService) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*schedDto.AttendeeResponse, int64, error) {
	q := &schedRepo.AttendeeListQuery{Page: page, PageSize: pageSize}
	a, total, err := s.attendees.ListByUser(ctx, userID, q)
	if err != nil {
		return nil, 0, response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	var resp []*schedDto.AttendeeResponse
	for _, x := range a {
		resp = append(resp, attendeeToResponse(&x))
	}
	return resp, total, nil
}

func (s *attendeeService) Respond(ctx context.Context, userID, attendeeID uuid.UUID, status int16) (*schedDto.AttendeeResponse, error) {
	a, err := s.attendees.GetByID(ctx, attendeeID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询参与人失败")
	}
	if a == nil {
		return nil, response.NewAppError(ErrCodeAttendeeNotFound, "参与人不存在")
	}
	if a.UserID != userID {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "只能响应自己的邀请")
	}
	if err := s.attendees.UpdateResponse(ctx, attendeeID, status); err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "更新响应失败")
	}
	a.ResponseStatus = status
	return attendeeToResponse(a), nil
}

// ============================================================
// 实现：reminderService
// ============================================================

type reminderService struct {
	db       *gorm.DB
	events   schedRepo.EventRepo
	reminders schedRepo.ReminderRepo
	dispatch ReminderDispatcher
}

// NewScheduleReminderService 构造函数
func NewScheduleReminderService(db *gorm.DB, events schedRepo.EventRepo, reminders schedRepo.ReminderRepo, dispatch ReminderDispatcher) ScheduleReminderService {
	return &reminderService{db: db, events: events, reminders: reminders, dispatch: dispatch}
}

func (s *reminderService) Create(ctx context.Context, userID uuid.UUID, req *schedDto.CreateReminderRequest) (*schedDto.ReminderResponse, error) {
	e, err := s.events.GetByID(ctx, req.EventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询日程失败")
	}
	if e == nil {
		return nil, response.NewAppError(ErrCodeEventNotFound, "日程不存在")
	}
	rm := &schedModel.Reminder{
		ID:           uuid.New(),
		EventID:      req.EventID,
		UserID:       userID,
		RemindOffset: req.RemindOffset,
		RemindTime:   e.StartTime.Add(-time.Duration(req.RemindOffset) * time.Minute),
		RemindMethod: defaultString(req.RemindMethod, "app"),
		Status:       schedModel.ReminderStatusPending,
	}
	if err := s.reminders.Create(ctx, rm); err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "创建提醒失败")
	}
	return reminderToResponse(rm), nil
}

func (s *reminderService) GetByID(ctx context.Context, id uuid.UUID) (*schedDto.ReminderResponse, error) {
	rm, err := s.reminders.GetByID(ctx, id)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询提醒失败")
	}
	if rm == nil {
		return nil, response.NewAppError(ErrCodeReminderNotFound, "提醒不存在")
	}
	return reminderToResponse(rm), nil
}

func (s *reminderService) ListByEvent(ctx context.Context, viewer uuid.UUID, eventID uuid.UUID) ([]*schedDto.ReminderResponse, error) {
	rms, err := s.reminders.ListByEvent(ctx, eventID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询提醒失败")
	}
	var resp []*schedDto.ReminderResponse
	for _, r := range rms {
		if r.UserID == viewer {
			resp = append(resp, reminderToResponse(&r))
		}
	}
	return resp, nil
}

func (s *reminderService) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*schedDto.ReminderResponse, int64, error) {
	q := &schedRepo.ReminderListQuery{Page: page, PageSize: pageSize}
	rms, total, err := s.reminders.ListByUser(ctx, userID, q)
	if err != nil {
		return nil, 0, response.NewAppError(response.CodeInternalError, "查询提醒失败")
	}
	var resp []*schedDto.ReminderResponse
	for _, r := range rms {
		resp = append(resp, reminderToResponse(&r))
	}
	return resp, total, nil
}

func (s *reminderService) Cancel(ctx context.Context, userID, reminderID uuid.UUID) error {
	rm, err := s.reminders.GetByID(ctx, reminderID)
	if err != nil {
		return response.NewAppError(response.CodeInternalError, "查询提醒失败")
	}
	if rm == nil {
		return response.NewAppError(ErrCodeReminderNotFound, "提醒不存在")
	}
	if rm.UserID != userID {
		return response.NewAppError(ErrCodeEventNoAccess, "无权操作此提醒")
	}
	return s.reminders.Cancel(ctx, reminderID)
}

func (s *reminderService) Snooze(ctx context.Context, userID, reminderID uuid.UUID, req *schedDto.SnoozeReminderRequest) (*schedDto.ReminderResponse, error) {
	rm, err := s.reminders.GetByID(ctx, reminderID)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "查询提醒失败")
	}
	if rm == nil {
		return nil, response.NewAppError(ErrCodeReminderNotFound, "提醒不存在")
	}
	if rm.UserID != userID {
		return nil, response.NewAppError(ErrCodeEventNoAccess, "无权操作此提醒")
	}
	if rm.Status == schedModel.ReminderStatusFired {
		return nil, response.NewAppError(ErrCodeReminderAlreadyFired, "提醒已触发，无法推迟")
	}
	const maxSnoozeMinutes = 60
	if rm.SnoozeMinutes + req.SnoozeMinutes > maxSnoozeMinutes {
		return nil, response.NewAppError(ErrCodeReminderSnoozeExceed, "推迟总时长超过60分钟")
	}
	newFire := rm.RemindTime.Add(time.Duration(req.SnoozeMinutes) * time.Minute)
	updated, err := s.reminders.Snooze(ctx, reminderID, req.SnoozeMinutes, newFire)
	if err != nil {
		return nil, response.NewAppError(response.CodeInternalError, "推迟提醒失败")
	}
	return reminderToResponse(updated), nil
}

// FirePending 调度器入口：扫描待触发提醒 → 发送 → 标记
func (s *reminderService) FirePending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	rms, err := s.reminders.ListPendingToFire(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	fired := 0
	for _, rm := range rms {
		// 查事件
		e, _ := s.events.GetByID(ctx, rm.EventID)
		if e == nil {
			s.reminders.Cancel(ctx, rm.ID)
			continue
		}
		// 发送提醒
		if s.dispatch != nil {
			_ = s.dispatch.Dispatch(ctx, []uuid.UUID{rm.UserID}, e.Title, rm.RemindMethod, e.StartTime)
		}
		// 标记已触发
		if err := s.reminders.MarkTriggered(ctx, rm.ID); err == nil {
			fired++
		}
	}
	return fired, nil
}

// ============================================================
// 辅助函数：DTO 转换 + 默认值
// ============================================================

func eventToResponse(e *schedModel.EventWithDetails) *schedDto.EventResponse {
	if e == nil {
		return nil
	}
	return &schedDto.EventResponse{
		ID:            e.ID,
		Title:         e.Title,
		Description:   e.Description,
		CalendarType:  e.CalendarType,
		OwnerID:       e.OwnerID,
		StartTime:     e.StartTime,
		EndTime:       e.EndTime,
		Location:      e.Location,
		OnlineLink:    e.OnlineLink,
		Visibility:    e.Visibility,
		ShareTargets:  e.ShareTargetsIDs(),
		RepeatRule:    e.RepeatRule,
		RepeatID:      e.RepeatID,
		Status:        e.Status,
		MeetingID:     e.MeetingID,
		Version:       e.Version,
		AttendeeCount: e.AttendeeCount,
		ReminderCount: e.ReminderCount,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func eventBasicToResponse(e *schedModel.Event) *schedDto.EventResponse {
	if e == nil {
		return nil
	}
	return &schedDto.EventResponse{
		ID:            e.ID,
		Title:         e.Title,
		Description:   e.Description,
		CalendarType:  e.CalendarType,
		OwnerID:       e.OwnerID,
		StartTime:     e.StartTime,
		EndTime:       e.EndTime,
		Location:      e.Location,
		OnlineLink:    e.OnlineLink,
		Visibility:    e.Visibility,
		ShareTargets:  e.ShareTargetsIDs(),
		RepeatRule:    e.RepeatRule,
		RepeatID:      e.RepeatID,
		Status:        e.Status,
		MeetingID:     e.MeetingID,
		Version:       e.Version,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func eventsToResponses(rows []schedModel.EventWithDetails) []*schedDto.EventResponse {
	var resp []*schedDto.EventResponse
	for i := range rows {
		resp = append(resp, eventToResponse(&rows[i]))
	}
	return resp
}

func attendeeToResponse(a *schedModel.Attendee) *schedDto.AttendeeResponse {
	if a == nil {
		return nil
	}
	return &schedDto.AttendeeResponse{
		ID:             a.ID,
		EventID:        a.EventID,
		UserID:         a.UserID,
		Role:           a.Role,
		ResponseStatus: a.ResponseStatus,
		RespondedAt:    a.RespondedAt,
		CreatedAt:      a.CreatedAt,
	}
}

func reminderToResponse(rm *schedModel.Reminder) *schedDto.ReminderResponse {
	if rm == nil {
		return nil
	}
	return &schedDto.ReminderResponse{
		ID:            rm.ID,
		EventID:       rm.EventID,
		UserID:        rm.UserID,
		RemindOffset:  rm.RemindOffset,
		RemindTime:    rm.RemindTime,
		RemindMethod:  rm.RemindMethod,
		Status:        rm.Status,
		FiredAt:       rm.FiredAt,
		SnoozeCount:   rm.SnoozeCount,
		SnoozeMinutes: rm.SnoozeMinutes,
		CreatedAt:     rm.CreatedAt,
		UpdatedAt:     rm.UpdatedAt,
	}
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func defaultInt16(v, def int16) int16 {
	if v <= 0 {
		return def
	}
	return v
}
