-- ============================================================
-- 000025_schedule_management.up.sql
-- Issue #78：日程管理模块
-- 三层表：schedule_events 日程主体 / schedule_event_attendees 参与人 / schedule_reminders 提醒
-- 外键：schedule_events.meeting_id → meetings.id（可空，关联会议模块）
-- UUID 主键 + golang-migrate 编号迁移
-- 权限码：schedule:read / create / update / delete / manage
-- ============================================================

-- ---- schedule_events：日程事件主体表 ----
CREATE TABLE IF NOT EXISTS schedule_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title           VARCHAR(200) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    calendar_type   VARCHAR(20)  NOT NULL DEFAULT 'personal',  -- personal / shared
    owner_id        UUID         NOT NULL,                        -- 创建人（用户表）
    start_time      TIMESTAMPTZ  NOT NULL,
    end_time        TIMESTAMPTZ  NOT NULL,
    location        VARCHAR(300) NOT NULL DEFAULT '',
    online_link     VARCHAR(500) NOT NULL DEFAULT '',
    visibility      VARCHAR(20)  NOT NULL DEFAULT 'private',     -- private / shared / public
    share_targets   JSONB        NOT NULL DEFAULT '[]',          -- 被共享的用户 ID 列表（JSON 数组）
    repeat_rule     VARCHAR(500) NOT NULL DEFAULT '',            -- RRULE，空表示不重复
    repeat_id       UUID,                                        -- 例外事件指向母事件
    status          SMALLINT     NOT NULL DEFAULT 0,             -- 0 草稿 1 已发布 2 已取消 3 已完成
    meeting_id      UUID,                                        -- 软引用 meetings.id，可空
    version         INT          NOT NULL DEFAULT 0,             -- 乐观锁
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_schedule_events_owner      ON schedule_events (owner_id);
CREATE INDEX IF NOT EXISTS idx_schedule_events_start_time ON schedule_events (start_time);
CREATE INDEX IF NOT EXISTS idx_schedule_events_end_time   ON schedule_events (end_time);
CREATE INDEX IF NOT EXISTS idx_schedule_events_status     ON schedule_events (status);
CREATE INDEX IF NOT EXISTS idx_schedule_events_meeting_id ON schedule_events (meeting_id);
CREATE INDEX IF NOT EXISTS idx_schedule_events_repeat_id  ON schedule_events (repeat_id);
CREATE INDEX IF NOT EXISTS idx_schedule_events_calendar   ON schedule_events (calendar_type, owner_id);

-- ---- schedule_event_attendees：日程参与人关联表 ----
CREATE TABLE IF NOT EXISTS schedule_event_attendees (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id        UUID NOT NULL REFERENCES schedule_events(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,                                 -- 参与人（用户表）
    role            SMALLINT NOT NULL DEFAULT 1,                   -- 1 组织者 2 必须参与 3 可选参与
    response_status SMALLINT NOT NULL DEFAULT 0,                   -- 0 待回复 1 接受 2 拒绝 3 待定
    responded_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_sched_attendee_event ON schedule_event_attendees (event_id);
CREATE INDEX IF NOT EXISTS idx_sched_attendee_user  ON schedule_event_attendees (user_id);

-- ---- schedule_reminders：日程提醒表 ----
CREATE TABLE IF NOT EXISTS schedule_reminders (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id         UUID NOT NULL REFERENCES schedule_events(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL,                                    -- 被提醒用户
    remind_offset    INT  NOT NULL DEFAULT 15,                        -- 提前分钟数
    remind_time      TIMESTAMPTZ NOT NULL,                             -- 实际触发时间
    remind_method    VARCHAR(20) NOT NULL DEFAULT 'app',               -- app / email / sms
    status           SMALLINT NOT NULL DEFAULT 0,                      -- 0 待触发 1 已触发 2 已取消 3 已推迟
    fired_at         TIMESTAMPTZ,
    snooze_count     INT  NOT NULL DEFAULT 0,                          -- 已推迟次数
    snooze_minutes   INT  NOT NULL DEFAULT 0,                          -- 本次推迟分钟数
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_schedule_reminders_event   ON schedule_reminders (event_id);
CREATE INDEX IF NOT EXISTS idx_schedule_reminders_user    ON schedule_reminders (user_id);
CREATE INDEX IF NOT EXISTS idx_schedule_reminders_status  ON schedule_reminders (status);
CREATE INDEX IF NOT EXISTS idx_schedule_reminders_time    ON schedule_reminders (remind_time);

-- ---- 权限种子（5 个细粒度权限） ----
INSERT INTO permissions (id, name, code, resource, action, description, type, is_system, status)
VALUES
    (uuid_generate_v4(), '日程读取', 'schedule:read',   'schedule', 'read',   '查看日程列表与详情',              2, true, 0),
    (uuid_generate_v4(), '日程创建', 'schedule:create', 'schedule', 'create', '创建新日程',                     2, true, 0),
    (uuid_generate_v4(), '日程修改', 'schedule:update', 'schedule', 'update', '修改已有日程',                   2, true, 0),
    (uuid_generate_v4(), '日程删除', 'schedule:delete', 'schedule', 'delete', '删除日程',                       2, true, 0),
    (uuid_generate_v4(), '日程管理', 'schedule:manage', 'schedule', 'manage', '日程管理：状态变更、共享、关联', 2, true, 0)
ON CONFLICT (code) DO NOTHING;
