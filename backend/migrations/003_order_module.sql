-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 003
-- 目标: 对齐《数据库字段设计文档_V1.3》工单模块
--  1. repair_orders: status/urgency ENUM → VARCHAR, 新增 category/property/room/contact
--  2. order_images: image_type ENUM → VARCHAR, 新增 is_deleted 软删除标记
--  3. order_timeline: 新增 order_no 冗余字段, action/from_status/to_status ENUM → VARCHAR
--  4. notifications: channel ENUM → VARCHAR
-- 版本: V1.3
-- =============================================================================

-- 1. repair_orders: 枚举转字符串 + 新增字段
ALTER TABLE repair_orders
    ALTER COLUMN status  TYPE VARCHAR(20) USING status::text,
    ALTER COLUMN urgency TYPE VARCHAR(20) USING urgency::text,
    ADD COLUMN IF NOT EXISTS category VARCHAR(20) NOT NULL DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS property VARCHAR(20) NOT NULL DEFAULT 'repair',
    ADD COLUMN IF NOT EXISTS room     VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS contact  VARCHAR(40) NOT NULL DEFAULT '';

COMMENT ON COLUMN repair_orders.category IS '报修类别: computer/network/printer/other';
COMMENT ON COLUMN repair_orders.property IS '工单性质: repair/purchase/replace/warranty';
COMMENT ON COLUMN repair_orders.room     IS '报修房间号/位置';
COMMENT ON COLUMN repair_orders.contact  IS '联系人及电话';

-- 2. order_images: 枚举转字符串 + 软删除标记
ALTER TABLE order_images
    ALTER COLUMN image_type TYPE VARCHAR(20) USING image_type::text,
    ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN order_images.image_type IS '图片类型: fault=故障图 receipt=收据图';
COMMENT ON COLUMN order_images.is_deleted IS '软删除标记, true=已删除(待图床清理)';

-- 3. order_timeline: 冗余工单号 + 枚举转字符串
ALTER TABLE order_timeline
    ADD COLUMN IF NOT EXISTS order_no VARCHAR(20) NOT NULL DEFAULT '',
    ALTER COLUMN action      TYPE VARCHAR(30) USING action::text,
    ALTER COLUMN from_status TYPE VARCHAR(20) USING from_status::text,
    ALTER COLUMN to_status   TYPE VARCHAR(20) USING to_status::text;

-- 回填 order_timeline.order_no
UPDATE order_timeline tl
SET order_no = o.order_no
FROM repair_orders o
WHERE tl.order_id = o.id;

COMMENT ON COLUMN order_timeline.order_no IS '冗余工单号, 避免联表查询';

-- 4. notifications: 枚举转字符串
ALTER TABLE notifications
    ALTER COLUMN channel TYPE VARCHAR(20) USING channel::text;

-- 清理废弃的 ENUM 类型
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS urgency_level;
DROP TYPE IF EXISTS image_type;
DROP TYPE IF EXISTS action_type;
DROP TYPE IF EXISTS notify_channel;
