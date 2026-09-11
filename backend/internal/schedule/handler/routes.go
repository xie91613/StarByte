package handler

import (
	"gorm.io/gorm"

	rbacRepo "github.com/Yogdunana/StarByte/backend/internal/rbac/repo"
	rbacService "github.com/Yogdunana/StarByte/backend/internal/rbac/service"
	"schedService" "github.com/Yogdunana/StarByte/backend/internal/schedule/service"
	"github.com/Yogdunana/StarByte/backend/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// ScheduleHandlers 聚合三个 handler
type ScheduleHandlers struct {
	Event    *EventHandler
	Attendee *AttendeeHandler
	Reminder *ReminderHandler
}

// NewScheduleHandlers 构造函数
func NewScheduleHandlers(
	eventSvc schedService.ScheduleEventService,
	attendeeSvc schedService.ScheduleAttendeeService,
	reminderSvc schedService.ScheduleReminderService,
) *ScheduleHandlers {
	return &ScheduleHandlers{
		Event:    NewEventHandler(eventSvc),
		Attendee: NewAttendeeHandler(attendeeSvc),
		Reminder: NewReminderHandler(reminderSvc),
	}
}

// ============================================================
// 路由注册辅助（对齐 meeting handler 的 withPermission / withScope 模式）
// ============================================================

// withPermission 仅校验权限码（RequirePermission + PermissionRequired）
func withPermission(
	group *gin.RouterGroup,
	permCode string,
	cacheService rbacService.PermissionCacheService,
) *gin.RouterGroup {
	g := group.Group("")
	g.Use(middleware.RequirePermission(permCode))
	g.Use(middleware.PermissionRequired(cacheService))
	return g
}

// withReadScope 读权限 + DataScope（schedule:read）
func withReadScope(
	group *gin.RouterGroup,
	cacheService rbacService.PermissionCacheService,
	db *gorm.DB,
	deptRepo rbacRepo.DepartmentRepo,
) *gin.RouterGroup {
	g := group.Group("")
	g.Use(middleware.RequirePermission("schedule:read"))
	g.Use(middleware.RequireDataScope("schedule:read"))
	g.Use(middleware.PermissionRequired(cacheService))
	g.Use(middleware.DataScopeMiddleware(db, deptRepo, cacheService))
	return g
}

// withWriteScope 写权限 + DataScope（组合模式：permission + RequireDataScope + DataScopeMiddleware）
func withWriteScope(
	group *gin.RouterGroup,
	permCode string,
	cacheService rbacService.PermissionCacheService,
	db *gorm.DB,
	deptRepo rbacRepo.DepartmentRepo,
) *gin.RouterGroup {
	g := group.Group("")
	g.Use(middleware.RequirePermission(permCode))
	g.Use(middleware.RequireDataScope(permCode))
	g.Use(middleware.PermissionRequired(cacheService))
	g.Use(middleware.DataScopeMiddleware(db, deptRepo, cacheService))
	return g
}

// ============================================================
// RegisterRoutes 注册日程模块路由
//
// 路径前缀：/api/v1/schedule
// 完整路径示例：
//   GET    /api/v1/schedule/events          日程列表（全局可见）
//   POST   /api/v1/schedule/events          创建日程
//   GET    /api/v1/schedule/events/:id      日程详情
//   PUT    /api/v1/schedule/events/:id      更新日程
//   DELETE /api/v1/schedule/events/:id      删除日程
//   ...
//
// 权限模型：
//   schedule:read   读权限（含 DataScope 过滤）
//   schedule:create 创建
//   schedule:update 修改
//   schedule:delete 删除
//   schedule:manage 管理（状态变更、共享、关联会议等）
//
// 使用方式（main.go 中）：
//   schedH := handler.NewScheduleHandlers(eventSvc, attendeeSvc, reminderSvc)
//   handler.RegisterRoutes(protected, schedH, cacheService, database.DB(), deptRepo)
// ============================================================

func RegisterRoutes(
	r *gin.RouterGroup,
	h *ScheduleHandlers,
	cacheService rbacService.PermissionCacheService,
	db *gorm.DB,
	deptRepo rbacRepo.DepartmentRepo,
) {
	// ---- events 路由组 ----
	events := r.Group("/schedule/events")
	{
		// 读权限路由（全局可见，但受 DataScope 过滤）
		read := withReadScope(events, cacheService, db, deptRepo)
		read.GET("", h.Event.List)            // 日程列表
		read.GET("/owner", h.Event.ListMyOwner) // 我创建的
		read.GET("/attended", h.Event.ListMyAttended) // 我参与的
		read.GET("/range", h.Event.ListByTimeRange)   // 时间区间查询
		read.GET("/:id", h.Event.Get)         // 日程详情

		// 创建路由
		create := withPermission(events, "schedule:create", cacheService)
		create.POST("", h.Event.Create)

		// 修改路由（需 DataScope 过滤）
		update := withWriteScope(events, "schedule:update", cacheService, db, deptRepo)
		update.PUT("/:id", h.Event.Update)

		// 删除路由
		del := withWriteScope(events, "schedule:delete", cacheService, db, deptRepo)
		del.DELETE("/:id", h.Event.Delete)

		// 管理路由
		manage := withWriteScope(events, "schedule:manage", cacheService, db, deptRepo)
		manage.PATCH("/:id/status", h.Event.UpdateStatus)
		manage.POST("/:id/meeting", h.Event.LinkMeeting)
		manage.DELETE("/:id/meeting", h.Event.UnlinkMeeting)

		// 参与人子路由（只读）
		attendeeRead := withReadScope(events, cacheService, db, deptRepo)
		attendeeRead.GET("/:event_id/attendees", h.Attendee.List)
		attendeeRead.GET("/:event_id/attendees/:attendee_id", h.Attendee.Get)

		// 参与人管理子路由
		attendeeManage := withWriteScope(events, "schedule:manage", cacheService, db, deptRepo)
		attendeeManage.POST("/:event_id/attendees", h.Attendee.Add)
		attendeeManage.POST("/:event_id/attendees/batch", h.Attendee.BatchAdd)
		attendeeManage.DELETE("/:event_id/attendees/:attendee_id", h.Attendee.Remove)

		// 提醒子路由（按日程查）
		read.GET("/:event_id/reminders", h.Reminder.ListByEvent)
	}

	// ---- reminders 路由组 ----
	reminders := r.Group("/schedule/reminders")
	{
		// 创建提醒
		create := withPermission(reminders, "schedule:create", cacheService)
		create.POST("", h.Reminder.Create)

		// 我的提醒（需要登录，不限特定权限码，因为是个人资源）
		reminders.GET("", h.Reminder.ListMy)
		reminders.GET("/:reminder_id", h.Reminder.Get)
		reminders.DELETE("/:reminder_id", h.Reminder.Cancel)
		reminders.PATCH("/:reminder_id/snooze", h.Reminder.Snooze)
	}

	// ---- attendees 路由组（个人视角）----
	attendees := r.Group("/schedule/attendees")
	{
		// 响应邀请（个人操作，不限特定权限码）
		attendees.PATCH("/me", h.Attendee.ListMy) // 我参与的所有日程（个人视角）
	}
}
