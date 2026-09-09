-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 010
-- 目标: 引入「已退回」独立状态 (rejected)，与草稿 (draft) 区分
--   C19（方案 B）：退回操作不再将工单置回 draft，而是置为独立状态 rejected，
--   语义清晰、可筛选、可统计；原靠 reject_reason 非空隐式判断「被退回草稿」的逻辑下线。
-- 说明: repair_orders.status 在 003 已由 ENUM 转为 VARCHAR(20)，此处无需 ALTER TYPE，
--        仅做存量数据回填（幂等）。
-- 版本: V1.3 (依据 docs/v2/小程序前端开发任务文档 V1.3.md C19)
-- 说明: 本文档语句为逐条执行, 不含 DO/多语句块 (由 Go 迁移工具逐条执行)
-- =============================================================================

-- 存量回填：历史「被退回草稿」（status='draft' 且 reject_reason 非空）统一置为 'rejected'，
-- 使列表/统计能正确识别「已退回」。新退回操作由后端 Reject 直接写入 'rejected'，不受影响。
UPDATE repair_orders
SET status = 'rejected'
WHERE status = 'draft'
  AND reject_reason IS NOT NULL
  AND reject_reason <> '';
