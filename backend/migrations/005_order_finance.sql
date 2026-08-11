-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 005
-- 目标: 对齐《数据库字段设计文档_V1.3》repair_orders 对账字段
--  1. repair_content: 具体维修操作内容
--  2. quantity / unit_price / amount(生成列) / metadata(JSONB)
-- 版本: V1.3
-- =============================================================================

ALTER TABLE repair_orders
    ADD COLUMN IF NOT EXISTS repair_content VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quantity       INTEGER     NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS unit_price     DECIMAL(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS metadata       JSONB       NOT NULL DEFAULT '{}';

COMMENT ON COLUMN repair_orders.repair_content IS '具体维修操作内容';
COMMENT ON COLUMN repair_orders.quantity      IS '维修或更换的物料/台数数量';
COMMENT ON COLUMN repair_orders.unit_price    IS '单价';
COMMENT ON COLUMN repair_orders.metadata      IS '维修附加元数据 JSONB, 结构见 RepairMetadata';

-- amount: 生成列, 数据库自动计算 (quantity * unit_price), 只读不可写入
-- 先删除可能存在的旧列(普通列/生成列均可), 再重建为生成列, 保证幂等
ALTER TABLE repair_orders DROP COLUMN IF EXISTS amount;
ALTER TABLE repair_orders
    ADD COLUMN amount DECIMAL(10,2) GENERATED ALWAYS AS (quantity * unit_price) STORED;

COMMENT ON COLUMN repair_orders.amount IS '金额 = quantity * unit_price, 自动计算';

-- 索引: 按工单号查询 (order_no 已唯一索引, 无需重复创建)
