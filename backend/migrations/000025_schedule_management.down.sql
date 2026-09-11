-- ============================================================
-- 000025_schedule_management.down.sql
-- 回滚日程管理模块
-- ============================================================

DROP INDEX IF EXISTS idx_schedule_reminders_time;
DROP INDEX IF EXISTS idx_schedule_reminders_status;
DROP INDEX IF EXISTS idx_schedule_reminders_user;
DROP INDEX IF EXISTS idx_schedule_reminders_event;

DROP INDEX IF EXISTS idx_sched_attendee_user;
DROP INDEX IF EXISTS idx_sched_attendee_event;

DROP INDEX IF EXISTS idx_schedule_events_calendar;
DROP INDEX IF EXISTS idx_schedule_events_repeat_id;
DROP INDEX IF EXISTS idx_schedule_events_meeting_id;
DROP INDEX IF EXISTS idx_schedule_events_status;
DROP INDEX IF EXISTS idx_schedule_events_end_time;
DROP INDEX IF EXISTS idx_schedule_events_start_time;
DROP INDEX IF EXISTS idx_schedule_events_owner;

DROP TABLE IF EXISTS schedule_reminders;
DROP TABLE IF EXISTS schedule_event_attendees;
DROP TABLE IF EXISTS schedule_events;

DELETE FROM permissions
WHERE code IN ('schedule:read', 'schedule:create', 'schedule:update', 'schedule:delete', 'schedule:manage');
