-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 006
-- 目标: repair_orders 新增维修员字段 (业务员导出功能)
--  1. repairer_id: 维修员(业务员)ID, 接单(accept)时绑定
-- 版本: V1.3
-- =============================================================================

ALTER TABLE repair_orders
    ADD COLUMN IF NOT EXISTS repairer_id UUID REFERENCES users(id);

COMMENT ON COLUMN repair_orders.repairer_id IS '维修员(业务员)ID, accept 接单时写入';

-- 索引: 按维修员查询工单 (导出/筛选)
CREATE INDEX IF NOT EXISTS idx_orders_repairer ON repair_orders (repairer_id);
