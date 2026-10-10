-- 016 维修备注字段：用于详情展示与「稍后补签 → 补签完工」时回显之前填写的维修备注
ALTER TABLE repair_orders ADD COLUMN remark varchar(500) NOT NULL DEFAULT '';
