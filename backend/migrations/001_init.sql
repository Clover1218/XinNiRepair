-- =============================================================================
-- 电脑维修店报修系统 —— 数据库初始化脚本
-- 数据库: PostgreSQL 13+
-- 版本: V1.0
-- =============================================================================

-- 启用 UUID 扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- 1. 企业表 (enterprises)
-- =============================================================================
CREATE TABLE enterprises (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(50)  NOT NULL,
    invite_code     VARCHAR(8)   NOT NULL UNIQUE,          -- 6-8位字母数字邀请码
    auto_approve    BOOLEAN      NOT NULL DEFAULT FALSE,   -- 是否开启免审核加入
    status          SMALLINT     NOT NULL DEFAULT 1,       -- 1=正常 0=已注销
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_enterprises_invite_code ON enterprises(invite_code);
CREATE INDEX idx_enterprises_status     ON enterprises(status);

COMMENT ON TABLE  enterprises               IS '企业表';
COMMENT ON COLUMN enterprises.name          IS '企业名称, 2-50字符';
COMMENT ON COLUMN enterprises.invite_code   IS '唯一邀请码, 系统生成6位字母数字组合';
COMMENT ON COLUMN enterprises.auto_approve  IS '免审核加入开关';

-- =============================================================================
-- 2. 用户表 (users)
-- =============================================================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    openid          VARCHAR(64)  NOT NULL UNIQUE,          -- 微信公众号 openid
    unionid         VARCHAR(64),                           -- 微信开放平台 unionid (跨应用打通)
    nickname        VARCHAR(32)  NOT NULL,                  -- 微信昵称
    avatar_url      VARCHAR(512),                          -- 微信头像 URL
    phone           VARCHAR(20),                           -- 手机号 (选填)
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_openid  ON users(openid);
CREATE INDEX idx_users_phone   ON users(phone);

COMMENT ON TABLE users          IS '用户表 (所有注册用户)';
COMMENT ON COLUMN users.openid  IS '微信公众平台 OpenID';
COMMENT ON COLUMN users.unionid IS '微信开放平台 UnionID, 用于跨应用识别用户';

-- =============================================================================
-- 3. 企业成员关系表 (memberships)
-- =============================================================================
CREATE TYPE member_role   AS ENUM ('admin', 'member');
CREATE TYPE member_status AS ENUM ('pending', 'approved', 'rejected', 'removed');

CREATE TABLE memberships (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    enterprise_id   UUID         NOT NULL REFERENCES enterprises(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    role            member_role  NOT NULL DEFAULT 'member',       -- admin | member
    status          member_status NOT NULL DEFAULT 'pending',     -- pending | approved | rejected | removed
    joined_at       TIMESTAMPTZ,                                  -- 审批通过时间
    removed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE(enterprise_id, user_id)  -- 同一用户在同一企业内唯一
);

CREATE INDEX idx_memberships_enterprise ON memberships(enterprise_id);
CREATE INDEX idx_memberships_user       ON memberships(user_id);
CREATE INDEX idx_memberships_status     ON memberships(enterprise_id, status);
CREATE INDEX idx_memberships_role       ON memberships(enterprise_id, role);

COMMENT ON TABLE memberships                IS '企业成员关系表';
COMMENT ON COLUMN memberships.status        IS 'pending=待审核 approved=已通过 rejected=已拒绝 removed=已移除';

-- =============================================================================
-- 4. 报修工单表 (repair_orders)
-- =============================================================================
CREATE TYPE order_status AS ENUM (
    'draft',        -- 草稿
    'reported',     -- 已上报
    'reviewed',     -- 已阅
    'processing',   -- 处理中
    'completed',    -- 已处理 (终态)
    'cancelled'     -- 已取消 (终态, 暂保留)
);
CREATE TYPE urgency_level AS ENUM ('normal', 'urgent', 'very_urgent');

CREATE TABLE repair_orders (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_no        VARCHAR(20)  NOT NULL UNIQUE,         -- 工单号: WO202607310001
    enterprise_id   UUID         NOT NULL REFERENCES enterprises(id),
    reporter_id     UUID         NOT NULL REFERENCES users(id),  -- 报修人
    project_name    VARCHAR(20)  NOT NULL,                 -- 报修项目名称 (≤20字)
    description     VARCHAR(500) NOT NULL,                 -- 报修描述 (≤500字)
    urgency         urgency_level NOT NULL DEFAULT 'normal', -- 紧急程度
    status          order_status NOT NULL DEFAULT 'draft',  -- 当前状态
    reject_reason   VARCHAR(200),                          -- 退回原因 (≥10字时才填)
    submitted_at    TIMESTAMPTZ,                           -- 提交上报时间
    reviewed_at     TIMESTAMPTZ,                           -- 查阅时间
    accepted_at     TIMESTAMPTZ,                           -- 接单时间
    completed_at    TIMESTAMPTZ,                           -- 完工时间
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_orders_enterprise  ON repair_orders(enterprise_id);
CREATE INDEX idx_orders_reporter    ON repair_orders(reporter_id);
CREATE INDEX idx_orders_status      ON repair_orders(enterprise_id, status);
CREATE INDEX idx_orders_urgency     ON repair_orders(enterprise_id, urgency);
CREATE INDEX idx_orders_order_no    ON repair_orders(order_no);
CREATE INDEX idx_orders_submitted   ON repair_orders(enterprise_id, submitted_at DESC);
CREATE INDEX idx_orders_created     ON repair_orders(created_at DESC);

COMMENT ON TABLE repair_orders                  IS '报修工单主表';
COMMENT ON COLUMN repair_orders.order_no        IS '工单号: WO + 日期 + 4位序号';
COMMENT ON COLUMN repair_orders.project_name    IS '报修项目名称,不超过20字';
COMMENT ON COLUMN repair_orders.reject_reason   IS '退回原因,管理员退回时必填(≥10字)';
COMMENT ON COLUMN repair_orders.status          IS '工单状态: draft→reported→reviewed→processing→completed';

-- =============================================================================
-- 5. 工单图片表 (order_images)
-- =============================================================================
CREATE TYPE image_type AS ENUM ('fault', 'receipt');

CREATE TABLE order_images (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID        NOT NULL REFERENCES repair_orders(id) ON DELETE CASCADE,
    image_url       VARCHAR(512) NOT NULL,                  -- 图片存储 URL
    image_type      image_type  NOT NULL DEFAULT 'fault',  -- fault=故障图 receipt=收据图
    sort_order      SMALLINT    NOT NULL DEFAULT 0,        -- 排序序号
    file_size       INTEGER,                               -- 文件大小 (bytes)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_images_order ON order_images(order_id);

COMMENT ON TABLE order_images              IS '工单关联图片表';
COMMENT ON COLUMN order_images.image_url   IS '图片存储URL (OSS/CDN)';
COMMENT ON COLUMN order_images.image_type  IS 'fault=故障现场图 receipt=维修收据图';
COMMENT ON COLUMN order_images.sort_order  IS '图片展示顺序';

-- =============================================================================
-- 6. 工单操作日志表 (order_timeline) — 时间轴
-- =============================================================================
CREATE TYPE action_type AS ENUM (
    'create_draft',     -- 创建草稿
    'submit',           -- 提交上报
    'review',           -- 查阅
    'accept',           -- 接单维修
    'complete',         -- 完工
    'reject',           -- 退回
    'upload_receipt',   -- 上传收据
    'cancel'            -- 取消
);

CREATE TABLE order_timeline (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID         NOT NULL REFERENCES repair_orders(id) ON DELETE CASCADE,
    operator_id     UUID         NOT NULL REFERENCES users(id),    -- 操作人
    action          action_type  NOT NULL,                         -- 操作类型
    from_status     order_status,                                  -- 变更前状态
    to_status       order_status,                                  -- 变更后状态
    remark          VARCHAR(500),                                  -- 备注 (退回原因/收据链接等)
    ip_address      VARCHAR(45),                                   -- 操作IP
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_timeline_order    ON order_timeline(order_id, created_at);
CREATE INDEX idx_timeline_operator ON order_timeline(operator_id);

COMMENT ON TABLE order_timeline               IS '工单操作时间轴';
COMMENT ON COLUMN order_timeline.action       IS '操作类型枚举';
COMMENT ON COLUMN order_timeline.from_status  IS '变更前状态, create_draft时为空';
COMMENT ON COLUMN order_timeline.to_status    IS '变更后状态';

-- =============================================================================
-- 7. 通知记录表 (notifications)
-- =============================================================================
CREATE TYPE notify_channel AS ENUM ('wechat_template', 'websocket');

CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID            NOT NULL REFERENCES users(id),
    order_id        UUID            NOT NULL REFERENCES repair_orders(id) ON DELETE CASCADE,
    channel         notify_channel  NOT NULL,                     -- 通知渠道
    title           VARCHAR(100)    NOT NULL,
    content         VARCHAR(500)    NOT NULL,
    is_read         BOOLEAN         NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMPTZ,
    send_status     SMALLINT        NOT NULL DEFAULT 1,          -- 1=待发送 2=已发送 3=发送失败
    fail_reason     VARCHAR(200),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user       ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_order      ON notifications(order_id);
CREATE INDEX idx_notifications_send_status ON notifications(send_status, created_at);

COMMENT ON TABLE notifications              IS '通知记录表';
COMMENT ON COLUMN notifications.channel     IS '通知渠道: wechat_template | websocket';
COMMENT ON COLUMN notifications.send_status IS '1=待发送 2=已发送 3=发送失败';

-- =============================================================================
-- 8. 管理员操作审计日志 (audit_logs)
-- =============================================================================
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    enterprise_id   UUID         NOT NULL REFERENCES enterprises(id),
    operator_id     UUID         NOT NULL REFERENCES users(id),   -- 操作人
    action          VARCHAR(50)  NOT NULL,                        -- 操作: remove_member / export_orders 等
    target_type     VARCHAR(50)  NOT NULL,                        -- 目标类型: user / order / enterprise
    target_id       UUID,                                         -- 目标ID
    detail          JSONB,                                        -- 操作详情 (JSON)
    ip_address      VARCHAR(45),                                  -- 操作IP
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_enterprise ON audit_logs(enterprise_id, created_at DESC);
CREATE INDEX idx_audit_operator   ON audit_logs(operator_id);

COMMENT ON TABLE audit_logs               IS '管理员操作审计日志';
COMMENT ON COLUMN audit_logs.detail       IS '操作详情, JSON格式存储变更前后差异';

-- =============================================================================
-- 初始数据 (可选)
-- =============================================================================
-- 暂无
