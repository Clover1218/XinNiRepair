-- 009_v1_4_problem_to_property.sql
-- 目标：常见问题由「隶属项目大类」改为「直接隶属项目属性」(V1.4 就地修订)。
--
--  1. project_problems.category_id → property_id (FK 指向 project_properties)
--  2. 唯一约束/索引由 (category_id, …) 改为 (property_id, …)
--  3. 清理遗留的按大类维度问题数据（问题不落工单表，可直接删除）
--  4. 以开发库当前自定义字典「电脑(维修/保修/更换)」为基线重建属性级问题种子
--
-- 说明：project_problems 中的记录仅作报修端快捷填充描述，不落入任何工单字段，
--       因此 3/4 步直接 DELETE + INSERT 不会影响历史工单。

-- 1) 删除指向大类的外键与旧索引
ALTER TABLE project_problems DROP CONSTRAINT IF EXISTS project_problems_category_id_fkey;
DROP INDEX IF EXISTS uq_project_problems_name;
DROP INDEX IF EXISTS idx_project_problems_category;

-- 2) category_id → property_id
ALTER TABLE project_problems RENAME COLUMN category_id TO property_id;

-- 3) 清理遗留数据：旧记录 property_id 指向的是大类 id，已不再合法
DELETE FROM project_problems
WHERE NOT EXISTS (SELECT 1 FROM project_properties p WHERE p.id = project_problems.property_id);

-- 4) 重建索引/外键
ALTER TABLE project_problems
    ADD CONSTRAINT project_problems_property_id_fkey FOREIGN KEY (property_id) REFERENCES project_properties(id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_problems_name ON project_problems(property_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_project_problems_property ON project_problems(property_id, sort_order) WHERE deleted_at IS NULL;

-- ─────────────────────────────────────────────
-- 5) 属性级问题种子（基线：大类「电脑」→ 属性 维修/保修/更换）
--    通过 类别名+属性名 子查询定位 property_id，重复执行由 NOT EXISTS 去重
-- ─────────────────────────────────────────────

-- 属性：维修
INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000001', p.id, '无法开机', '按电源键无响应、黑屏无法开机等', '["检查电源与排线","更换电源模块","主板检修"]'::jsonb, 1
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '维修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '无法开机');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000002', p.id, '蓝屏/死机', '使用中出现蓝屏、频繁死机重启', '["驱动更新","系统修复","内存/硬盘检测"]'::jsonb, 2
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '维修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '蓝屏/死机');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000003', p.id, '运行缓慢', '开机慢、运行卡顿', '["清理启动项","加装内存或换固态","系统重装"]'::jsonb, 3
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '维修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '运行缓慢');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000004', p.id, '网络故障', '无法上网、频繁断网等网络问题', '["重启光猫/路由","检查线路与接口","更换网络设备"]'::jsonb, 4
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '维修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '网络故障');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000005', p.id, '主板损坏', '无法点亮、供电异常、疑似主板故障', '["主板检修","更换电容/供电模块","更换主板"]'::jsonb, 5
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '维修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '主板损坏');

-- 属性：保修
INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000101', p.id, '保修期内故障确认', '设备保修期内出现故障，需确认是否免费保修', '["核实保修期","确认故障现象与范围","按保修流程送修"]'::jsonb, 1
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '保修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '保修期内故障确认');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000102', p.id, '保修状态查询', '查询设备是否在保、剩余保修期', '["核对购买凭证","查询保修状态","办理延保或自费报价"]'::jsonb, 2
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '保修' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '保修状态查询');

-- 属性：更换
INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000201', p.id, '更换屏幕', '屏幕碎裂、显示异常需更换', '["屏幕检测","更换屏幕总成","更换后测试"]'::jsonb, 1
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '更换' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '更换屏幕');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000202', p.id, '更换电池', '电池老化、续航骤降需更换', '["电池健康检测","更换电池","充电校准"]'::jsonb, 2
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '更换' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '更换电池');

INSERT INTO project_problems (id, property_id, name, description, common_solutions, sort_order)
SELECT '40000000-0000-4000-8000-000000000203', p.id, '更换其他配件', '适配器、风扇、键盘等配件更换', '["部件检测","更换适配器/风扇/键盘等","更换后测试"]'::jsonb, 3
FROM project_properties p JOIN project_categories c ON c.id = p.category_id
WHERE c.name = '电脑' AND p.name = '更换' AND c.deleted_at IS NULL AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM project_problems x WHERE x.property_id = p.id AND x.name = '更换其他配件');
