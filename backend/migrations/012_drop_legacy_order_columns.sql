-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 012
-- 目标: 清理 V1.4 已废弃的工单遗留列, 修复新建工单 INSERT 失败:
--   ERROR: null value in column "project_name" of relation "repair_orders"
--          violates not-null constraint (SQLSTATE 23502)
--
-- 背景: 001_init.sql 建表时 project_name 为 NOT NULL 且无 DEFAULT;
--   V1.4 起工单改存 category_id/category_name + property_id/property_name
--   (model.RepairOrder 已无 project_name 字段), GORM 生成的 INSERT 不再包含该列,
--   于是 NOT NULL 约束被触发。本地开发库曾被手工加过 DEFAULT '' 才未暴露此问题,
--   生产库未打补丁, 因此报错。
--
-- 处理: 直接删除三张已废弃的遗留列 (均已被项目字典取代, 代码无任何读写):
--   project_name  旧报修项目名称 (自由文本, 被 category/property 字典取代)
--   category      旧报修类别枚举 computer/network/printer/other (008 已回填新字典)
--   property      旧工单性质枚举 repair/purchase/replace/warranty (V1.4 已移除)
--   使用 IF EXISTS 保证幂等, 重复执行不报错。
-- 版本: V1.4
-- 说明: 本文档语句为逐条执行, 不含 DO/多语句块 (由 Go 迁移工具逐条执行)
-- =============================================================================

ALTER TABLE repair_orders DROP COLUMN IF EXISTS project_name;

ALTER TABLE repair_orders DROP COLUMN IF EXISTS category;

ALTER TABLE repair_orders DROP COLUMN IF EXISTS property;
