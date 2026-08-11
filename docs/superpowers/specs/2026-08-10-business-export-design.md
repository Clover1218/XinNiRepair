# 业务员导出功能设计文档

**日期**：2026-08-10
**状态**：已确认
**关联**：后端接口文档 v1.0 5.14 导出工单记录；管理后台前端开发文档 V1.1 6.6 导出工单

---

## 1. 背景与问题

5.14 导出工单接口支持两种模式：

- `enterprise`（企业对账单）✅ 已实现
- `repairer`（业务员汇总）⚠️ 已有基础实现，但存在核心缺陷

**核心问题：系统中没有清晰的"维修员/业务员"之分。**

现状分析：

1. **角色体系缺失**：系统只有 `users.role`（0=普通用户、1=平台管理员）和 `memberships.role`（0=普通成员、1=企业管理员），**没有"维修员"角色**。
2. **维修员归属靠推断**：当前 `repairer` 模式完全依赖时间轴 `order_timeline` 中 `action=complete` 的 `operator_id` 推断"谁修的工单"。没有明确的绑定关系，工单表无 `repairer_id` 字段。
3. **无维修员列表接口**：前端导出弹窗业务员模式只能手输 `repairer_id`（UUID），体验差。
4. **企业分组未体现**：文档要求"按企业分组，每个企业内按时间排序"，但当前实现仅排序，Excel 中无分组块与小计。

## 2. 已确认的决策

| 决策点 | 结论 |
| --- | --- |
| 维修员归属 | 工单表新增 `repairer_id` 字段，接单(accept)时绑定 |
| 角色划分 | 平台管理员即业务员（`users.role=1`），不新增角色枚举 |
| 业务员列表 | 新增 `GET /admin/repairers` 列表接口，前端下拉选择 |
| 历史工单 | 新增字段后老数据为空，导出时回退用时间轴 `complete` 操作人补齐 |
| 企业分组 | Excel 内按企业分组块 + 每块小计行 + 全局合计 |
| 维修员变更 | 接单时锁定，工单流转后不可修改 |

## 3. 方案设计

### 3.1 数据模型：`repair_orders` 新增 `repairer_id`

```go
// RepairOrder 新增字段
RepairerID *string `gorm:"type:uuid;index:idx_orders_repairer"` // 维修员(业务员), accept 时写入
Repairer   User    `gorm:"foreignKey:RepairerID"`
```

- 可空：草稿/已上报/已阅状态工单为 NULL，接单后写入
- 加索引：导出与筛选需要按维修员查询
- 业务员 = 平台管理员（`users.role=1`）

### 3.2 接单(accept)绑定维修员

`AdminOrderService.Accept` 在 `reviewed → processing` 状态流转时，写入 `order.RepairerID = adminID`（操作人即维修员）。与时间轴 `accept` 记录操作人天然一致。

### 3.3 新增 `GET /admin/repairers` 列表接口

返回所有平台管理员（`users.role=1`）列表，供业务员导出下拉选择。

**请求**：`GET /admin/repairers`

**响应**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      { "id": "550e8400-...", "nickname": "张三", "avatar_url": "https://..." }
    ]
  }
}
```

### 3.4 业务员导出（mode=repairer）逻辑改造

**查询（新老数据兼容）**：`ListForExportByRepairer` 改为：

```sql
WHERE (repair_orders.repairer_id = ? 
   OR (repair_orders.repairer_id IS NULL AND repair_orders.id IN (
        SELECT DISTINCT order_id FROM order_timeline 
        WHERE operator_id = ? AND action = 'complete')))
  AND repair_orders.completed_at >= ? AND repair_orders.completed_at <= ?
```

- 新工单走 `repairer_id` 精确匹配
- 历史工单（`repairer_id IS NULL`）回退时间轴 `complete` 操作人

**Excel 企业分组 + 小计**：按企业分块，每块一个小组标题 + 小计行（单数、金额），最后全局合计行。

**`repairer` 列取值**：优先 `repairer_id` 关联昵称，为空回退时间轴 `complete` 操作人。

### 3.5 前端导出弹窗改造

业务员模式下，`repairer_id` 从 `/admin/repairers` 下拉选择（filterable），不再手输 UUID。

## 4. 改动文件清单

| 层 | 文件 | 改动 |
| --- | --- | --- |
| 模型 | `backend/internal/model/model.go` | `RepairOrder` 加 `RepairerID` / `Repairer` |
| Repository | `backend/internal/repository/order.go` | 新增 `ListRepairers`；改造 `ListForExportByRepairer` |
| Service | `backend/internal/service/admin_order.go` | `Accept` 写入 `RepairerID` |
| Service | `backend/internal/service/order_export.go` | 企业分组+小计、`repairer` 列取值回退逻辑 |
| Handler | `backend/internal/handler/admin.go` | 新增 `Repairers` 接口 |
| 路由 | `backend/cmd/server/main.go` | 注册 `GET /admin/repairers` |
| 前端 API | `web-frontend/src/api/admin.ts` | 新增 `getRepairers` |
| 前端页面 | `web-frontend/src/views/orders/list.vue` | 导出弹窗业务员下拉 |
| 文档 | `docs/后端接口设计文档v1.0.md` | 新增 5.15 维修员列表；更新 5.14 |
| 文档 | `web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md` | 导出弹窗交互、新增接口 |

## 5. 边界与兼容性

- **历史数据**：老工单 `repairer_id` 为 NULL，导出回退时间轴推断，不影响新逻辑
- **接单操作人即维修员**：`accept` 的 `operator_id` 与 `repairer_id` 一致
- **不可修改**：接单后 `repairer_id` 锁定，不提供修改接口（YAGNI）
- **无数据**：返回错误码 4507（沿用现有 `ErrNoExportData`）

## 6. 不做的事（YAGNI）

- 不新增"维修员"角色枚举，业务员即平台管理员
- 不提供维修员更换/修改接口
- 不做一次性历史数据回填 SQL（导出时动态回退即可）
