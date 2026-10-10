-- V1.5: 工单本质标签派生字段（不新增状态机状态）
-- 设计意图：状态机保持精简 (7 态)，"不能修 / 待补收据 / 待补签"等本质由
-- 既有事实 + 本批字段组合派生（详见 service.ComputeDisplayTag），便于一眼展示与筛选。
--
--   signature_status: 客户签字状态 none/signed/pending_sign（仅完工后有意义）
--   finish_outcome:   cancelled 终态的细分成因
--                     unrepairable=不能修 / customer_cancelled=客户原因取消
--                     旧数据 NULL 表示普通取消（无细分成因）

ALTER TABLE repair_orders
  ADD COLUMN IF NOT EXISTS signature_status varchar(20) NOT NULL DEFAULT 'none',
  ADD COLUMN IF NOT EXISTS finish_outcome   varchar(20);

COMMENT ON COLUMN repair_orders.signature_status IS '客户签字状态: none=未涉及/signed=已签字确认/pending_sign=已通知下次补签';
COMMENT ON COLUMN repair_orders.finish_outcome   IS 'cancelled 终态成因: unrepairable=不能修, customer_cancelled=客户原因取消, NULL 表示普通取消';
