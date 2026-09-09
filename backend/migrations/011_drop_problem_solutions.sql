-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 011
-- 目标: 项目字典「常见问题」(project_problems) 移除多余的 common_solutions 字段
--   需求：常见问题表仅保留 name + description（及系统字段 id/property_id/
--         sort_order/deleted_at/时间戳），不再存储"常见解决建议"列表。
--   影响：
--     - 删除列 common_solutions（jsonb，仅作展示用，未落入工单表，可安全丢弃）
--     - 同步后端 service/order.go 的 4.1 选项接口、service/project.go 的字典 CRUD
--       不再读写该列；Web 后台字典页移除"常见解决建议"多标签编辑器。
-- 版本: V1.3（依据需求变更：project_problems 仅 name + description）
-- 说明: 本文档语句为逐条执行, 不含 DO/多语句块 (由 Go 迁移工具逐条执行)
-- =============================================================================

-- 移除常见问题表的 common_solutions 列（IF EXISTS 保证幂等，重复执行不报错）
ALTER TABLE project_problems
DROP COLUMN IF EXISTS common_solutions;
