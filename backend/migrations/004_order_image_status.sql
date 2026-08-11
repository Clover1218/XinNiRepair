-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 004
-- 目标: 对齐《后端接口设计文档v1.0》工单 4.2/4.3/4.4/4.9 新逻辑
--  1. order_images: is_deleted(BOOL) → status(VARCHAR): temporary/active/deleted
--  2. repair_orders.order_no 允许为空 (草稿阶段不生成, 接单时生成)
--  3. order_timeline.order_no 允许为空
-- 版本: V1.4
-- =============================================================================

-- 1. order_images: status 替换 is_deleted
ALTER TABLE order_images
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'temporary';

COMMENT ON COLUMN order_images.status IS '图片状态: temporary=刚上传未确认 active=草稿确认 deleted=软删除';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'order_images' AND column_name = 'is_deleted') THEN
        UPDATE order_images SET status = CASE WHEN is_deleted THEN 'deleted' ELSE 'active' END;
        ALTER TABLE order_images DROP COLUMN is_deleted;
    END IF;
END $$;

-- 2. repair_orders.order_no 允许为空 (空草稿不生成工单号)
ALTER TABLE repair_orders ALTER COLUMN order_no DROP NOT NULL;

-- 3. order_timeline.order_no 允许为空
ALTER TABLE order_timeline ALTER COLUMN order_no DROP NOT NULL;
