-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 002
-- 目标: 对齐《数据库字段设计文档_V1.2》与《后端接口设计文档v1.0》
--  1. enterprises 新增 invite_code_expires_at (邀请码有效期, null=永不过期)
--  2. users 新增 role (0-普通用户 1-平台管理员)
--  3. memberships.role 由 ENUM 改为 SMALLINT (0-普通成员 1-企业管理员)
--  4. memberships.status 由 ENUM 改为 VARCHAR(20)
-- 版本: V1.2
-- =============================================================================

-- 1. enterprises: 邀请码有效期
ALTER TABLE enterprises
    ADD COLUMN IF NOT EXISTS invite_code_expires_at TIMESTAMPTZ;

COMMENT ON COLUMN enterprises.invite_code_expires_at IS '邀请码有效期, NULL 表示永不过期';

-- 2. users: 平台角色
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

COMMENT ON COLUMN users.role IS '平台角色: 0-普通用户 1-平台管理员';

-- 3. memberships.role: ENUM → SMALLINT (admin=1, member=0)
ALTER TABLE memberships
    ALTER COLUMN role TYPE SMALLINT USING (CASE role WHEN 'admin' THEN 1 ELSE 0 END),
    ALTER COLUMN role SET DEFAULT 0,
    ALTER COLUMN role SET NOT NULL;

-- 4. memberships.status: ENUM → VARCHAR(20)
ALTER TABLE memberships
    ALTER COLUMN status TYPE VARCHAR(20) USING status::text,
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN status SET NOT NULL;

-- 清理废弃的 ENUM 类型
DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS member_status;
