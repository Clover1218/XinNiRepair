-- =============================================================================
-- 迁移脚本 013
-- 目标: 引入「维修业务员经营范围」数据权限基座 (《经营范围与状态机改造设计文档 V1.5》第二章)
--  1. 新增 repairer_enterprises: 业务员 ↔ 可服务客户企业的指派关系
--  2. 索引: (repairer_id, enterprise_id) 唯一; enterprise_id 供反向查询
--  3. 存量兜底: 现有维修业务员(role=1)灌入全部现存企业, 保证上线后可见范围不突变
--
-- 对应问题: #10 设置可接单企业, 不能所有企业都能查看
--           #28 按权限范围内企业查询统计, 防止企业名单泄露
--
-- 说明: 本文档语句为逐条执行, 不含 DO/多语句块 (由 Go 迁移工具逐条执行)
-- =============================================================================

-- ── 1. 维修业务员经营范围 ──
-- id 不设 DEFAULT: 与既有表(users/memberships/enterprises)一致, 由应用层 uuid.New() 生成;
-- 且本机未安装 uuid-ossp 扩展, 使用 uuid_generate_v4() 会导致建表失败。
CREATE TABLE IF NOT EXISTS repairer_enterprises (
    id            UUID PRIMARY KEY,
    repairer_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    status        VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
);

COMMENT ON TABLE repairer_enterprises IS '维修业务员经营范围: 业务员可服务的客户企业指派';
COMMENT ON COLUMN repairer_enterprises.status IS 'active=生效 / revoked=已撤销(软删除, 保留指派证据用于对账)';
COMMENT ON COLUMN repairer_enterprises.created_by IS '指派人(超级管理员)';

CREATE UNIQUE INDEX IF NOT EXISTS uq_repairer_ent
    ON repairer_enterprises(repairer_id, enterprise_id);
CREATE INDEX IF NOT EXISTS idx_repairer_ent_ent
    ON repairer_enterprises(enterprise_id);
CREATE INDEX IF NOT EXISTS idx_repairer_ent_repairer_active
    ON repairer_enterprises(repairer_id) WHERE status = 'active';

-- ── 2. 存量兜底 ──
-- 采用「无指派 = 看不见任何企业」策略后, 若不做兜底, 现有业务员会在上线瞬间集体失明。
-- 此处为全部现存维修业务员授予全部现存企业, 使升级后行为与现状一致;
-- 之后由超级管理员在后台逐步收窄。
-- gen_random_uuid() 为 PostgreSQL 13+ 内置, 无需额外扩展
INSERT INTO repairer_enterprises (id, repairer_id, enterprise_id, status, created_by, created_at)
SELECT gen_random_uuid(), u.id, e.id, 'active', NULL, NOW()
FROM users u
CROSS JOIN enterprises e
WHERE u.role = 1
  AND e.status = 1
ON CONFLICT (repairer_id, enterprise_id) DO NOTHING;
