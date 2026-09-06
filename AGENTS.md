# AGENTS.md — 新泥电脑维修店报修系统

> 供 AI 编码助手使用的项目指南。修改代码前请先阅读本文，遵守既有架构与约定。

## 1. 项目简介

面向电脑维修店的报修系统，包含三端：

| 端 | 目录 | 说明 |
|---|---|---|
| 后端 | `backend/` | Go + Gin + GORM + PostgreSQL，RESTful API |
| Web 管理后台 | `web-frontend/` | Vue 3 + TypeScript + Element Plus + Vite + Pinia |
| 微信小程序 | `uni-app-frontend/` | uni-app (Vue 3) + wot-design-uni，报修用户/管理端 |

## 2. 目录结构

```
├── backend/
│   ├── cmd/server/          # 入口 main.go（依赖装配、路由注册）
│   ├── internal/
│   │   ├── config/          # 配置加载（config.yaml + 环境变量覆盖）
│   │   ├── handler/         # HTTP 处理器（admin/auth/enterprise/order/upload/wechat…）
│   │   ├── service/         # 业务逻辑（admin_order/order/order_export/notifier/user_admin…）
│   │   ├── repository/      # 数据访问（GORM）
│   │   ├── model/           # 数据模型与常量
│   │   ├── errors/          # 应用错误定义（apperrors）
│   │   └── middleware/      # JWT 鉴权、CORS、日志、Recovery
│   ├── migrations/          # SQL 迁移文件（手动执行）
│   ├── config/config.yaml   # 服务配置（支持 ${ENV_VAR} 覆盖）
│   ├── pkg/                 # 可复用包（imagebed/logger/response）
│   └── Makefile             # make run/build/test/migrate
├── web-frontend/src/
│   ├── api/                 # axios 封装（client.ts 拦截器 + admin.ts/auth.ts）
│   ├── views/               # 页面（enterprises/orders/users/login）
│   ├── router/ stores/ types/ utils/ components/ layouts/
├── uni-app-frontend/
│   ├── pages/               # index/order/enterprise/auth/profile/admin
│   ├── stores/ utils/ components/ types/
│   └── utils/config.ts      # BASE_URL（生产走 Docker 单入口域名）
├── docs/                    # 需求/接口/数据库/部署文档 + 订阅消息模板信息
├── scripts/                 # 运维脚本（如 manage-user-role.sh）
├── docker-compose.yml       # 仅生产部署使用
└── .env / .env.example      # 仅生产部署使用（.env.example 为模板）
```

## 3. 环境说明（重要）

### 开发环境（当前开发方式）

- **不使用 `.env` 和 `docker-compose.yml`**，这两者仅用于生产部署。
- 开发依赖本机安装的服务：
  - PostgreSQL：`localhost:5432`，账号密码见 `backend/config/config.yaml`
  - 图床 EasyImages：`http://localhost:81`
- 后端配置只读 `backend/config/config.yaml`（`server.port=8080`，`mode=debug`，日志写 `backend/logs/server.log`）。
- 环境变量优先级高于 config.yaml（非空时覆盖），但开发时通常不设置。
- 启动方式：`backend/run.bat`（即 `go run cmd/server/main.go`）或 `make run`。
- **数据库迁移需手动执行** `backend/migrations/*.sql`；`config.yaml` 中 `auto_migrate: false`。
- **后端代码改动后必须重启服务**才生效。

### 生产环境（Docker 单入口）

- `docker-compose.yml` 编排四个服务：postgres、easyimage（图床）、backend（Go）、web-frontend（Nginx 托管 Vue + 反代 API）。
- 配置通过 `.env` 注入（参考 `.env.example`）：`DB_USER/DB_PASSWORD/DB_NAME/DB_AUTO_MIGRATE/WECHAT_APP_ID/WECHAT_APP_SECRET/EASYIMAGE_TOKEN/端口映射` 等。
- 前端端口是所有客户端唯一入口；小程序端 `utils/config.ts` 的 `BASE_URL` 指向该域名。

## 4. 常用命令

```bash
# ── 后端（在 backend/ 下）──
go run cmd/server/main.go    # 开发运行（或 run.bat / make run）
go build ./...               # 编译验证（改完 Go 代码必做）
go vet ./...                 # 静态检查
go test -race ./...          # 测试

# ── Web 前端（在 web-frontend/ 下）──
pnpm install                 # 安装依赖（pnpm）
pnpm dev                     # Vite 开发服务器
npx vue-tsc --noEmit --skipLibCheck   # 类型检查（改完 TS/Vue 必做）
pnpm build                   # 类型检查 + 构建

# ── 小程序（在 uni-app-frontend/ 下）──
# 使用 HBuilderX / 微信开发者工具运行到微信开发者工具
npx vue-tsc --noEmit         # 类型检查
```

## 5. 架构与分层约定

- 分层：`handler → service → repository → model`，禁止跨层调用（如 handler 直连 repository）。
- 依赖装配集中在 `cmd/server/main.go`，新增 service/handler 后需在此注册并挂路由。
- 路由统一挂在 `/api/v1` 下；鉴权用 `middleware.JWTAuth`。角色为**双层模型**：全局 `users.role` 0=普通用户 / 1=维修业务员（原平台管理员，店方）/ 2=超级管理员；单位内 `memberships.role` 0=普通成员 / 1=单位审核员。后台工单处理接口用 `middleware.RequirePlatformAdmin`（role>=1），单位审核员操作（本单位成员审批、本单位工单审核）按该企业 membership.role=1 校验，超级管理员（role=2）接口单独控制。
- 错误处理统一走 `apperrors` + `pkg/response`：成功 `response.OK(c, data)`，失败 `response.FailError(c, err)`；JSON 格式为 `{"code": xxx, "message": "..."}`。
- 响应结构体字段按需使用 `omitempty`，避免返回空值噪音。

## 6. 业务与领域要点

- **工单状态机**：draft(草稿) → reported(已上报) → pending_accept(待接单) → processing(处理中) → completed(已处理/完成)；`reject` 退回（原因≥10字，回 draft，退回非独立状态）、`cancelled`(已取消) 终态、completed 可 `reopen`(重新打开)→processing。审核通过由单位审核员执行：写 `audited_at/audited_by`（`reviewed_at` 已废弃）。
- **角色（双层模型）**：全局 users.role 0=普通用户 / 1=维修业务员（原平台管理员，店方：接单/处理/完工/收据与费用登记）/ 2=超级管理员（另管项目字典）；单位内 memberships.role 0=普通成员 / 1=单位审核员（成员审批 + 本单位工单审核 reported→pending_accept/退回 + 汇总统计）；membership 状态 pending/approved/rejected/removed。
- **企业邀请**：邀请码 + 有效期（刷新接口），成员加入支持 `auto_approve`（免审核）开关，企业设置更新走 `PUT /api/v1/enterprises/:id`（name + auto_approve，局部更新）。
- **项目字典（库表存储，替代 JSON）**：`project_categories` / `project_properties` / `project_problems` 三表（软删除 `deleted_at` + `sort_order` 排序，大类↔属性/常见问题一对多）；工单存 `category_id`/`property_id` + 名称快照（`category_name`/`property_name`），常见问题仅作描述快捷填充、不落库。
- **工单完结对账**：completed 时必填 `repair_content`/`quantity`/`unit_price` + 收据（1-3 张，完工后可补充、可替换不可删），金额列为 GORM 生成列 `quantity * unit_price`；metadata（维修结果/方式/保修期/时长/额外备注）。
- **导出**（5.14）：enterprise（企业对账单）/ repairer（业务员汇总，按企业分组 + 小计）两种模式，excelize 内存生成 xlsx；无数据返回业务错误（前端需正确解析 Blob 错误体）。
- **微信订阅消息**：模板配置见 `docs/小程序订阅消息模板信息.md`；发送封装在 `service/notifier.go`，通知失败仅记日志不阻塞业务；模板字段名（如 time13）必须与微信后台一致。

## 7. 前端约定

### Web 管理后台
- API 定义集中在 `src/api/`，axios 实例在 `client.ts`：统一 token 注入、错误拦截（含 Blob 错误体解析、`Content-Disposition` 文件名 `filename*=utf-8''` 解码）。
- 路由守卫按角色跳转：平台管理员 → `/enterprises`，登录态判断依赖 user store 的 `isPlatformAdmin`（JWT payload base64 解码需注意 padding 补齐）。
- 列表页模式：状态 Tab + 筛选栏 + el-table（`sortable="custom"` 服务端排序）+ el-pagination。

### 小程序端（uni-app）
- 请求统一在 `utils/request.ts`（`BASE_URL` 在 `utils/config.ts`）；上传用 **base64 JSON**（`http.post`）而非 `uni.uploadFile`。
- 上传接口必须同时兼容 multipart/form-data（Web 端二进制流）与 JSON+base64（小程序端）两种形式。
- 订阅消息授权：提交新工单时 `wx.requestSubscribeMessage` 一次性请求三个模板（处理中/退回/完结），拒绝授权不阻塞提交。
- UI 遵循 flexbox 布局：标签与内容等宽对齐、合理间距、垂直居中。

## 8. 已知约束与经验教训（务必遵守）

1. 环境变量非空时覆盖 config.yaml 同名配置。
2. 数据库 schema 变更：GORM `AutoMigrate` **不会修改已有列**，改列须手写 SQL 迁移到 `backend/migrations/` 并执行（可用 `go run ./cmd/migrate migrations/xxx.sql` 逐条执行，或 psql）。
3. 微信手机号解密/获取依赖有效的 AppID/Secret；接口返回非 2xx 或空 body 时要给出明确错误文案，不要让 `unexpected end of JSON input` 之类的原始错误透出。
4. `wx.cloud.callContainer` 不支持 multipart/form-data，小程序上传一律 base64。
5. `service.Update*` 类接口用指针字段做局部更新；校验前先判断"值未变化"（幂等），例如超管回传自身 role=2 不应触发"不可设为超管"拦截。
6. 改完代码后：Go 必跑 `go build`，前端必跑 `vue-tsc`；后端改动需重启进程。
7. koa-connect 包装会导致 ctx.state 丢失（历史教训）：中间件桥接需用原生实现，勿用 wrapper。

## 9. 文档索引

- `docs/v2/后端接口设计文档v1.2.md` — 全部 API 规范（V1.2 起，单位审核员 Web 登录/成员身份管理；v1.1/v1.0 旧版留存 docs/v2 与 docs/v1）
- `docs/v2/数据库字段设计文档_V1.4.md` — 数据库设计（当前基线）
- `docs/v2/报修系统需求规格说明书_V1.3.md` — 需求与章节编号来源（v1.2 就地修订后全面同步）
- `docs/v2/新泥报修系统-管理后台前端开发文档V1.2.md` — Web 后台页面规范（V1.2 起；V1.1 旧档在 docs/v1，改页面须同步）
- `docs/v1/电脑维修店报修系统部署文档.md` — 生产部署（旧版留存）
- `docs/小程序订阅消息模板信息.md` — 订阅消息模板 ID 与字段


## 10. 测试流程

### 10.1 修改后最低验证要求

修改 Go 后端代码后必须执行：

```bash
go build ./...
go vet ./...
```
如涉及已有测试：
```bash
go test ./...
```
### 10.2 API 集成验收

涉及 API 修改时，除编译验证外，应进行实际 API 调用验收。

测试流程：

连接本地 PostgreSQL
启动后端服务
使用测试账号通过正常登录接口获取 JWT
调用相关 API
验证 HTTP 状态码
验证响应 code/message/data
验证数据库状态变化
验证关联状态机与权限逻辑

禁止仅通过代码阅读判断功能正确。

### 10.3 测试账号
|角色|昵称|密码|
|---|---|---|
|维修业务员|测试用户1|admin123|
|超级管理员|管理员|admin123|

测试账号仅存在于本地开发数据库。