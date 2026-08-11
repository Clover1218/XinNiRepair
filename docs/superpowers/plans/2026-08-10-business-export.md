# 业务员导出功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完善业务员导出：工单表新增 `repairer_id` 字段（接单时绑定），新增维修员列表接口，业务员导出支持企业分组+小计与老数据回退。

**Architecture:** 在 `repair_orders` 表新增 `repairer_id`（可空，accept 时写入），业务员=平台管理员（`users.role=1`）。新增 `GET /admin/repairers` 列表接口供前端下拉。导出 `repairer` 模式查询改为 `repairer_id` 优先、历史空值回退时间轴 `complete` 操作人；Excel 按企业分组+每块小计+全局合计。

**Tech Stack:** Go + GORM + gin + excelize/v2；Vue 3 + Element Plus。

**设计文档:** `docs/superpowers/specs/2026-08-10-business-export-design.md`

---

### Task 0: 数据库迁移 - 新增 `repair_orders.repairer_id` 列

**Files:**
- Create: `backend/migrations/006_repairer.sql`

- [ ] **Step 1: 创建迁移文件**

创建 `backend/migrations/006_repairer.sql`，格式对齐 005 迁移：

```sql
-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 006
-- 目标: repair_orders 新增维修员字段 (业务员导出功能)
--  1. repairer_id: 维修员(业务员)ID, 接单(accept)时绑定
-- 版本: V1.3
-- =============================================================================

ALTER TABLE repair_orders
    ADD COLUMN IF NOT EXISTS repairer_id UUID REFERENCES users(id);

COMMENT ON COLUMN repair_orders.repairer_id IS '维修员(业务员)ID, accept 接单时写入';

-- 索引: 按维修员查询工单 (导出/筛选)
CREATE INDEX IF NOT EXISTS idx_orders_repairer ON repair_orders (repairer_id);
```

- [ ] **Step 2: 提交**

```bash
git add backend/migrations/006_repairer.sql
git commit -m "feat(migration): repair_orders 新增 repairer_id 列"
```

---

### Task 1: 模型层 - `RepairOrder` 新增 `repairer_id`

**Files:**
- Modify: `backend/internal/model/model.go:193-227`

- [ ] **Step 1: 修改模型定义**

在 `RepairOrder` 结构体的 `Metadata` 字段之后新增：

```go
	RepairerID *string `gorm:"type:uuid;index:idx_orders_repairer"` // 维修员(业务员), accept 时写入
	Repairer   User    `gorm:"foreignKey:RepairerID"`
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/internal/model/model.go
git commit -m "feat(model): repair_orders 新增 repairer_id 字段"
```

---

### Task 2: Repository 层 - 新增维修员列表查询 + 改造业务员导出查询

**Files:**
- Modify: `backend/internal/repository/order.go`（新增方法）

- [ ] **Step 1: 新增 `ListRepairers` 方法**

在 `ListForExportByRepairer` 方法附近新增：

```go
// ListRepairers 查询全部维修员(平台管理员, users.role=1), 按昵称排序 (5.15)
func (r *OrderRepository) ListRepairers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).
		Where("role = ?", model.PlatformRolePlatformAdmin).
		Order("nickname ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}
```

- [ ] **Step 2: 改造 `ListForExportByRepairer`**

将现有实现改为新字段优先、历史空值回退时间轴：

```go
// ListForExportByRepairer 业务员模式下导出 (5.14): repairer_id 优先, 历史空值回退时间轴 complete 操作人, 完工时间正序
func (r *OrderRepository) ListForExportByRepairer(ctx context.Context, repairerID, status string, from, to time.Time) ([]model.RepairOrder, error) {
	sub := r.db.Model(&model.OrderTimeline{}).
		Select("DISTINCT order_id").
		Where("operator_id = ? AND action = ?", repairerID, string(model.ActionComplete))
	base := r.db.WithContext(ctx).Model(&model.RepairOrder{}).
		Where("(repair_orders.repairer_id = ? OR (repair_orders.repairer_id IS NULL AND repair_orders.id IN (?))) AND repair_orders.completed_at IS NOT NULL", repairerID, sub).
		Where("repair_orders.completed_at >= ? AND repair_orders.completed_at <= ?", from, to)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var orders []model.RepairOrder
	err := base.Preload("Reporter").
		Preload("Enterprise").
		Preload("Repairer").
		Order("enterprise_id ASC, completed_at ASC").
		Find(&orders).Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}
```

- [ ] **Step 3: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add backend/internal/repository/order.go
git commit -m "feat(repo): 新增 ListRepairers, 业务员导出支持 repairer_id 与历史回退"
```

---

### Task 3: Service 层 - 接单绑定维修员 + 维修员列表方法

**Files:**
- Modify: `backend/internal/service/admin_order.go`

- [ ] **Step 1: 修改 `Accept` 方法**

在 `order.AcceptedAt = &now` 之后新增一行：

```go
	now := time.Now()
	from := order.Status
	order.Status = string(model.OrderProcessing)
	order.AcceptedAt = &now
	order.RepairerID = &adminID // 接单即绑定维修员 (5.4)
	if err := s.orders.Update(ctx, order); err != nil {
		return s.dbErr("update order failed", err)
	}
```

- [ ] **Step 2: 新增 `ListRepairers` service 方法**

在 `ListOrders` 方法附近新增（转发到 repository）：

```go
// ListRepairers 维修员(业务员)列表 (5.15): 平台管理员即维修员
func (s *AdminOrderService) ListRepairers(ctx context.Context) ([]model.User, error) {
	return s.orders.ListRepairers(ctx)
}
```

- [ ] **Step 3: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add backend/internal/service/admin_order.go
git commit -m "feat(service): 接单绑定 repairer_id, 新增维修员列表方法"
```

---

### Task 4: Handler 层 - 新增维修员列表接口

**Files:**
- Modify: `backend/internal/handler/admin.go`

- [ ] **Step 1: 新增 `Repairers` handler 方法**

在 `ExportOrders` 方法前新增：

```go
// Repairers 维修员(业务员)列表 (GET /admin/repairers, 5.15)
func (h *AdminHandler) Repairers(c *gin.Context) {
	users, err := h.orders.ListRepairers(c.Request.Context())
	if err != nil {
		response.FailError(c, err)
		return
	}
	list := make([]AdminReporter, 0, len(users))
	for _, u := range users {
		list = append(list, AdminReporter{ID: u.ID, Nickname: u.Nickname, AvatarURL: u.AvatarUrl})
	}
	response.OK(c, gin.H{"list": list})
}
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/internal/handler/admin.go
git commit -m "feat(handler): 新增 GET /admin/repairers 维修员列表接口"
```

---

### Task 5: 路由注册

**Files:**
- Modify: `backend/cmd/server/main.go:222-235`

- [ ] **Step 1: 注册路由**

在 admin 分组内新增：

```go
		{
			admin.GET("/repairers", adminH.Repairers) // 维修员列表 (5.15)
			admin.GET("/orders/export", adminH.ExportOrders) // 导出工单记录 (5.14, 需在 :order_id 前注册)
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(route): 注册 /admin/repairers"
```

---

### Task 6: Service 层 - 导出 Excel 企业分组 + 小计

**Files:**
- Modify: `backend/internal/service/order_export.go`

- [ ] **Step 1: 修改 `buildExcel`，企业分组小计**

将 `buildExcel` 中的数据行循环改造为按企业分组输出。替换 `// ── 数据行 ──` 至 `// ── 合计行 ──` 区块：

```go
	// ── 数据行（repairer 模式按企业分组 + 每块小计） ──
	row := headerRow + 1
	totalAmount := 0.0
	totalCount := 0

	// 分组：按 Enterprise.Name 分组，保持查询顺序（已按 enterprise_id 排序）
	type group struct {
		name   string
		orders []model.RepairOrder
	}
	var groups []group
	groupIndex := make(map[string]int)
	for _, o := range orders {
		name := o.Enterprise.Name
		if name == "" {
			name = "（未分配企业）"
		}
		if idx, ok := groupIndex[name]; ok {
			groups[idx].orders = append(groups[idx].orders, o)
		} else {
			groupIndex[name] = len(groups)
			groups = append(groups, group{name: name, orders: []model.RepairOrder{o}})
		}
	}

	amountColIdx := -1
	for i, field := range fields {
		if field == "amount" {
			amountColIdx = i + 2
			break
		}
	}

	for gi, g := range groups {
		if req.Mode == ExportModeRepairer && len(groups) > 1 {
			// 小组标题行
			f.SetCellValue(sheet, "A"+fmt.Sprint(row), g.name)
			row++
		}
		groupAmount := 0.0
		groupCount := 0
		for _, o := range g.orders {
			amount := amountOf(o)
			content := o.RepairContent
			if strings.TrimSpace(content) == "" {
				content = o.ProjectName
			}
			metadata := s.parseMetadata(o.Metadata)
			repairerName := s.repairerNameOf(o, repairerNames)

			f.SetCellValue(sheet, "A"+fmt.Sprint(row), groupCount+1)
			col := 2
			for _, field := range fields {
				f.SetCellValue(sheet, cellName(col)+fmt.Sprint(row), s.fieldValue(field, o, amount, content, metadata, repairerName))
				col++
			}
			groupAmount += amount
			groupCount++
			row++
		}
		if req.Mode == ExportModeRepairer && len(groups) > 1 {
			// 小组小计行
			f.SetCellValue(sheet, "A"+fmt.Sprint(row), "小计")
			if amountColIdx > 0 {
				f.SetCellValue(sheet, cellName(amountColIdx)+fmt.Sprint(row), fmt.Sprintf("%.2f", groupAmount))
			}
			f.SetCellValue(sheet, cellName(len(fields)+1)+fmt.Sprint(row), fmt.Sprintf("共 %d 单", groupCount))
			row++
		}
		totalAmount += groupAmount
		totalCount += groupCount
	}
	_ = gi

	// ── 合计行 ──
	sumRow := row
	f.SetCellValue(sheet, "A"+fmt.Sprint(sumRow), "合计")
	if amountColIdx > 0 {
		f.SetCellValue(sheet, cellName(amountColIdx)+fmt.Sprint(sumRow), fmt.Sprintf("%.2f", totalAmount))
	}
	f.SetCellValue(sheet, cellName(len(fields)+1)+fmt.Sprint(sumRow), fmt.Sprintf("共 %d 单", totalCount))
```

注意：原有 `totalAmount`/`totalCount` 的声明需一并移除（改在上方声明）。

- [ ] **Step 2: 新增 `repairerNameOf` 辅助方法**

```go
// repairerNameOf 维修员列取值: 优先 repair_orders.repairer_id, 为空回退时间轴 complete 操作人
func (s *OrderExportService) repairerNameOf(o model.RepairOrder, repairerNames map[string]string) string {
	if o.Repairer != nil && o.Repairer.Nickname != "" {
		return o.Repairer.Nickname
	}
	return repairerNames[o.ID]
}
```

- [ ] **Step 3: 调整表头信息区（repairer 模式的 infoValue 取值）**

`buildExcel` 中 `infoValue` 对 repairer 模式改为：

```go
	} else {
		infoLabel, infoValue = "维修员", ""
		for _, o := range orders {
			if v := s.repairerNameOf(o, repairerNames); v != "" {
				infoValue = v
				break
			}
		}
	}
```

- [ ] **Step 4: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/order_export.go
git commit -m "feat(export): 业务员汇总按企业分组+小计, repairer 列取值支持回退"
```

---

### Task 7: 前端 - 新增维修员 API + 导出弹窗下拉

**Files:**
- Modify: `web-frontend/src/api/admin.ts`
- Modify: `web-frontend/src/views/orders/list.vue`

- [ ] **Step 1: admin.ts 新增 getRepairers**

```typescript
  // V1.1 5.15 维修员(业务员)列表（导出弹窗下拉使用）
  getRepairers: () =>
    client.get<{ list: Array<{ id: string; nickname: string; avatar_url: string }> }>('/admin/repairers'),
```

- [ ] **Step 2: list.vue 新增 repairerOptions 与加载**

```typescript
const repairerOptions = ref<Array<{ id: string; nickname: string; avatar_url: string }>>([])

const loadRepairerOptions = async () => {
  if (repairerOptions.value.length) return
  const res = await adminAPI.getRepairers()
  repairerOptions.value = res.data.list
}
```

- [ ] **Step 3: openExportDialog 中加载维修员**

```typescript
  // 加载企业列表供下拉选择（mode=enterprise 时使用）
  await loadEnterpriseOptions()
  // 加载维修员列表供下拉选择（mode=repairer 时使用）
  await loadRepairerOptions()
  exportDialogVisible.value = true
```

- [ ] **Step 4: 模板业务员输入改为下拉**

将 `<el-input v-model="exportRepairerId" placeholder="请输入维修员 ID（待后端提供专用列表接口）" ... />` 替换为：

```html
        <el-form-item v-else label="维修员（必填）">
          <el-select v-model="exportRepairerId" placeholder="请选择维修员" filterable class="export-select">
            <el-option
              v-for="r in repairerOptions"
              :key="r.id"
              :label="r.nickname"
              :value="r.id"
            />
          </el-select>
        </el-form-item>
```

- [ ] **Step 5: 类型检查**

Run: `cd web-frontend && npx vue-tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 6: 提交**

```bash
git add web-frontend/src/api/admin.ts web-frontend/src/views/orders/list.vue
git commit -m "feat(frontend): 导出弹窗业务员改为下拉选择维修员"
```

---

### Task 8: 文档 - 后端接口文档新增 5.15、更新 5.14

**Files:**
- Modify: `docs/后端接口设计文档v1.0.md`

- [ ] **Step 1: 新增 5.15 维修员列表接口**

在 5.14 章节末尾追加：

```markdown
### 5.15 维修员列表

```
GET /admin/repairers
```

**鉴权**：用户角色为平台管理员。

**说明**：返回全部维修员（平台管理员，`users.role=1`）列表，供业务员导出时下拉选择。

**响应示例**：

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
```

- [ ] **Step 2: 更新 5.14 业务规则**

在 5.14 "业务员汇总（mode=repairer）" 下补充：

```markdown
+ 业务员汇总（mode=repairer）
 - 必须指定 repairer_id
 - 通过工单 `repairer_id` 字段（接单时绑定）关联查询该维修员完成的工单
 - 历史工单（`repairer_id` 为空）回退：按时间轴 `complete` 操作人关联
 - 按企业分组，每组企业内按时间排序
 - Excel 内按企业分块：小组标题 + 小计行（单数、金额），底部全局合计行
 - 底部生成汇总行：总单数、总金额
 - 用途：内部绩效核算
```

- [ ] **Step 3: 提交**

```bash
git add docs/后端接口设计文档v1.0.md
git commit -m "docs: 新增 5.15 维修员列表, 更新 5.14 业务员导出规则"
```

---

### Task 9: 文档 - 前端开发文档更新

**Files:**
- Modify: `web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md`

- [ ] **Step 1: admin.ts 片段新增 getRepairers**

```typescript
  // 5.15 维修员(业务员)列表（导出弹窗下拉使用）
  getRepairers: () => client.get('/admin/repairers'),
```

- [ ] **Step 2: 6.6 导出弹窗描述更新**

找到"维修员（必填）"手输 ID 的描述，改为下拉选择，并补充说明：

```markdown
**V1.1 更新：**
- 业务员模式"维修员"改为下拉选择（数据来源 `GET /admin/repairers`，仅平台管理员）
- 业务员汇总导出 Excel 按企业分块（小组标题 + 小计行），底部全局合计
```

- [ ] **Step 3: 提交**

```bash
git add web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md
git commit -m "docs(frontend): 导出弹窗业务员下拉与分组小计说明"
```

---

### Task 10: 全量验证

- [ ] **Step 1: 后端编译**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 2: 前端类型检查**

Run: `cd web-frontend && npx vue-tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 3: 功能自测（手工）**

1. 启动后端 + 前端 dev server
2. 管理后台 → 工单列表 → 导出 → 业务员汇总
3. 验证：维修员下拉可选中平台管理员
4. 选择日期范围导出，Excel 中按企业分组 + 小计 + 合计正确
