-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 008
-- 目标: 对齐《数据库字段设计文档 V1.4》状态机 / 双层角色 / 项目字典库表化 / 工单字段重构
--  1. 新增项目字典三表 project_categories / project_properties / project_problems (软删除 + sort_order)
--     + 业务种子数据 (电脑维修/手机维修/网络故障/打印机及办公设备/其他设备)
--  2. repair_orders: reviewed_at → audited_at; 新增 audited_by;
--     新增 category_id/category_name/property_id/property_name (项目大类/属性 + 名称快照)
--  3. 存量回填: status='reviewed' → 'pending_accept'; 旧单按旧枚举 category 映射新字典大类与首个属性
--  4. 清理: order_no 空串置 NULL (唯一索引允许重复 NULL)
-- 版本: V1.4 (依据 docs/v2/数据库字段设计文档_V1.4.md)
-- 说明: 本文档语句为逐条执行, 不含 DO/多语句块 (由 Go 迁移工具逐条执行)
-- =============================================================================

-- ── 1. 项目大类 ──
CREATE TABLE IF NOT EXISTS project_categories (
    id UUID PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    description VARCHAR(200) NOT NULL DEFAULT '',
    sort_order SMALLINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_categories_name ON project_categories(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_project_categories_active ON project_categories(sort_order) WHERE deleted_at IS NULL;

-- ── 2. 项目属性 (category ↔ property 一对多) ──
CREATE TABLE IF NOT EXISTS project_properties (
    id UUID PRIMARY KEY,
    category_id UUID NOT NULL REFERENCES project_categories(id),
    name VARCHAR(50) NOT NULL,
    description VARCHAR(200) NOT NULL DEFAULT '',
    sort_order SMALLINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_properties_name ON project_properties(category_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_project_properties_category ON project_properties(category_id, sort_order) WHERE deleted_at IS NULL;

-- ── 3. 常见问题 (category ↔ problem 一对多; 仅作描述快捷填充, 不落工单) ──
CREATE TABLE IF NOT EXISTS project_problems (
    id UUID PRIMARY KEY,
    category_id UUID NOT NULL REFERENCES project_categories(id),
    name VARCHAR(100) NOT NULL,
    description VARCHAR(200) NOT NULL DEFAULT '',
    common_solutions JSONB NOT NULL DEFAULT '[]',
    sort_order SMALLINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_problems_name ON project_problems(category_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_project_problems_category ON project_problems(category_id, sort_order) WHERE deleted_at IS NULL;

-- ── 4. repair_orders 重构 ──
ALTER TABLE repair_orders ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES project_categories(id);
ALTER TABLE repair_orders ADD COLUMN IF NOT EXISTS category_name VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE repair_orders ADD COLUMN IF NOT EXISTS property_id UUID REFERENCES project_properties(id);
ALTER TABLE repair_orders ADD COLUMN IF NOT EXISTS property_name VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE repair_orders ADD COLUMN IF NOT EXISTS audited_by UUID REFERENCES users(id);

-- reviewed_at → audited_at (当前库必有 reviewed_at 列, 仅执行一次)
ALTER TABLE repair_orders RENAME COLUMN reviewed_at TO audited_at;

-- 存量状态回填: reviewed(已阅) → pending_accept(待接单)
UPDATE repair_orders SET status = 'pending_accept' WHERE status = 'reviewed';

-- order_no 空串 → NULL (唯一索引下 NULL 不冲突, 与新代码草稿无单号一致)
UPDATE repair_orders SET order_no = NULL WHERE order_no = '';

-- 辅助索引 (与代码模型 GORM tag 对齐)
CREATE INDEX IF NOT EXISTS idx_orders_category ON repair_orders(category_id);
CREATE INDEX IF NOT EXISTS idx_orders_audited ON repair_orders(audited_by);
CREATE INDEX IF NOT EXISTS idx_orders_status ON repair_orders(enterprise_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_urgency ON repair_orders(enterprise_id, urgency);
CREATE INDEX IF NOT EXISTS idx_orders_submitted ON repair_orders(enterprise_id, submitted_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_completed ON repair_orders(enterprise_id, completed_at DESC);

-- =============================================================================
-- 5. 种子数据 (幂等: 仅当名称不存在时插入)
--    大类 UUID 段: 10000000-...; 属性 UUID 段: 20000000-...; 问题 UUID 段: 30000000-...
-- =============================================================================

-- 5.1 大类
INSERT INTO project_categories (id, name, description, sort_order)
SELECT '10000000-0000-4000-8000-000000000001', '电脑维修', '各类电脑硬件/软件故障维修', 1
WHERE NOT EXISTS (SELECT 1 FROM project_categories WHERE name = '电脑维修');
INSERT INTO project_categories (id, name, description, sort_order)
SELECT '10000000-0000-4000-8000-000000000002', '手机维修', '各类手机故障维修', 2
WHERE NOT EXISTS (SELECT 1 FROM project_categories WHERE name = '手机维修');
INSERT INTO project_categories (id, name, description, sort_order)
SELECT '10000000-0000-4000-8000-000000000003', '网络故障', '网络与无线覆盖问题', 3
WHERE NOT EXISTS (SELECT 1 FROM project_categories WHERE name = '网络故障');
INSERT INTO project_categories (id, name, description, sort_order)
SELECT '10000000-0000-4000-8000-000000000004', '打印机及办公设备', '打印机/复印机等办公外设', 4
WHERE NOT EXISTS (SELECT 1 FROM project_categories WHERE name = '打印机及办公设备');
INSERT INTO project_categories (id, name, description, sort_order)
SELECT '10000000-0000-4000-8000-000000000005', '其他设备', '其他电子设备', 5
WHERE NOT EXISTS (SELECT 1 FROM project_categories WHERE name = '其他设备');

-- 5.2 项目属性
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', '台式机', 1
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '台式机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', '笔记本', 2
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '笔记本');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000001', '一体机', 3
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '一体机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000011', '10000000-0000-4000-8000-000000000002', '安卓机', 1
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '安卓机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000012', '10000000-0000-4000-8000-000000000002', '苹果机', 2
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '苹果机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000013', '10000000-0000-4000-8000-000000000002', '平板', 3
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '平板');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000021', '10000000-0000-4000-8000-000000000003', '宽带路由器', 1
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = '宽带路由器');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000022', '10000000-0000-4000-8000-000000000003', '交换机', 2
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = '交换机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000023', '10000000-0000-4000-8000-000000000003', '无线AP', 3
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = '无线AP');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000031', '10000000-0000-4000-8000-000000000004', '打印机', 1
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '打印机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000032', '10000000-0000-4000-8000-000000000004', '复印机', 2
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '复印机');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000033', '10000000-0000-4000-8000-000000000004', '扫描仪', 3
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '扫描仪');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000041', '10000000-0000-4000-8000-000000000005', '显示器', 1
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '显示器');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000042', '10000000-0000-4000-8000-000000000005', '外设', 2
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '外设');
INSERT INTO project_properties (id, category_id, name, sort_order)
SELECT '20000000-0000-4000-8000-000000000043', '10000000-0000-4000-8000-000000000005', '其他', 3
WHERE NOT EXISTS (SELECT 1 FROM project_properties WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '其他');

-- 5.3 常见问题
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', '无法开机', '按电源键无反应或无显示', '["检查电源与排线","更换电源模块","主板检修"]', 1
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '无法开机');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', '蓝屏/死机', '运行中蓝屏或卡死', '["驱动更新","系统修复","内存/硬盘检测"]', 2
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '蓝屏/死机');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000001', '运行缓慢', '开机慢/运行卡顿', '["清理启动项","加装内存/换固态","系统重装"]', 3
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '运行缓慢');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000004', '10000000-0000-4000-8000-000000000001', '系统崩溃', '系统文件损坏无法启动', '["系统重装","病毒查杀","硬件检测"]', 4
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '系统崩溃');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000005', '10000000-0000-4000-8000-000000000001', '屏幕损坏', '显示屏碎裂/显示异常/背光故障', '["更换屏幕","检查排线","驱动重装"]', 5
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000001' AND name = '屏幕损坏');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000011', '10000000-0000-4000-8000-000000000002', '屏幕损坏', '碎屏/显示异常', '["更换屏幕","更换触摸总成"]', 1
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '屏幕损坏');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000012', '10000000-0000-4000-8000-000000000002', '电池耗电快', '待机时间短/充电异常', '["更换电池","检查充电接口"]', 2
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '电池耗电快');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000013', '10000000-0000-4000-8000-000000000002', '无法充电', '插电无反应或充不进', '["更换尾插/充电口","更换电池"]', 3
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000002' AND name = '无法充电');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000021', '10000000-0000-4000-8000-000000000003', '无法上网', '无法上网/外网不通', '["检查线路","重启光猫路由","配置下发"]', 1
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = '无法上网');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000022', '10000000-0000-4000-8000-000000000003', 'WiFi连接失败', '搜不到信号或连不上', '["重启设备","检查信道与密码","更换无线AP"]', 2
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = 'WiFi连接失败');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000023', '10000000-0000-4000-8000-000000000003', '网速慢/频繁断网', '带宽不足或时常掉线', '["测速排查","更换网线/水晶头","更换设备"]', 3
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000003' AND name = '网速慢/频繁断网');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000031', '10000000-0000-4000-8000-000000000004', '卡纸', '打印卡纸无法出纸', '["清理卡纸","更换搓纸轮","走纸通道检查"]', 1
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '卡纸');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000032', '10000000-0000-4000-8000-000000000004', '打印空白/模糊', '打印空白或字迹不清', '["更换硒鼓/墨盒","清理激光头","驱动设置"]', 2
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '打印空白/模糊');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000033', '10000000-0000-4000-8000-000000000004', '无法识别墨盒/硒鼓', '提示未安装耗材', '["重新安装耗材","清洁芯片触点","更换耗材"]', 3
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '无法识别墨盒/硒鼓');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000034', '10000000-0000-4000-8000-000000000004', '无法联网打印', '共享/网络打印不通', '["网络配置","端口共享设置","驱动重装"]', 4
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000004' AND name = '无法联网打印');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000041', '10000000-0000-4000-8000-000000000005', '设备无法通电', '通电无反应', '["检查电源/线缆","更换电源适配器","内部检修"]', 1
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '设备无法通电');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000042', '10000000-0000-4000-8000-000000000005', '设备异响', '运行异响/噪音大', '["清洁风扇","更换轴承/风扇","内部检修"]', 2
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '设备异响');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000043', '10000000-0000-4000-8000-000000000005', '按键/接口失灵', '按键无响应或接口接触不良', '["清洁触点","更换按键/接口","内部检修"]', 3
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '按键/接口失灵');
INSERT INTO project_problems (id, category_id, name, description, common_solutions, sort_order)
SELECT '30000000-0000-4000-8000-000000000044', '10000000-0000-4000-8000-000000000005', '其他', '其他未列明故障', '["现场检测"]', 4
WHERE NOT EXISTS (SELECT 1 FROM project_problems WHERE category_id = '10000000-0000-4000-8000-000000000005' AND name = '其他');

-- =============================================================================
-- 6. 旧单回填 (映射: computer→电脑维修, network→网络故障, printer→打印机及办公设备, other→其他设备)
--    属性取该大类 sort_order 最小的一条; 名称快照同步写入
-- =============================================================================

-- computer → 电脑维修
UPDATE repair_orders o SET
    category_id = '10000000-0000-4000-8000-000000000001',
    category_name = '电脑维修',
    property_id = (SELECT pp.id FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000001' ORDER BY pp.sort_order, pp.id LIMIT 1),
    property_name = (SELECT pp.name FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000001' ORDER BY pp.sort_order, pp.id LIMIT 1)
WHERE o.category = 'computer' AND o.category_id IS NULL;

-- network → 网络故障
UPDATE repair_orders o SET
    category_id = '10000000-0000-4000-8000-000000000003',
    category_name = '网络故障',
    property_id = (SELECT pp.id FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000003' ORDER BY pp.sort_order, pp.id LIMIT 1),
    property_name = (SELECT pp.name FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000003' ORDER BY pp.sort_order, pp.id LIMIT 1)
WHERE o.category = 'network' AND o.category_id IS NULL;

-- printer → 打印机及办公设备
UPDATE repair_orders o SET
    category_id = '10000000-0000-4000-8000-000000000004',
    category_name = '打印机及办公设备',
    property_id = (SELECT pp.id FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000004' ORDER BY pp.sort_order, pp.id LIMIT 1),
    property_name = (SELECT pp.name FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000004' ORDER BY pp.sort_order, pp.id LIMIT 1)
WHERE o.category = 'printer' AND o.category_id IS NULL;

-- other → 其他设备
UPDATE repair_orders o SET
    category_id = '10000000-0000-4000-8000-000000000005',
    category_name = '其他设备',
    property_id = (SELECT pp.id FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000005' ORDER BY pp.sort_order, pp.id LIMIT 1),
    property_name = (SELECT pp.name FROM project_properties pp WHERE pp.category_id = '10000000-0000-4000-8000-000000000005' ORDER BY pp.sort_order, pp.id LIMIT 1)
WHERE o.category = 'other' AND o.category_id IS NULL;
