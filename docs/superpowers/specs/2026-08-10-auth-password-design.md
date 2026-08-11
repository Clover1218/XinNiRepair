# 密码登录与小程序资料完善 设计文档

> 日期：2026-08-10
> 状态：已确认
> 关联文档：后端接口设计文档 v1.0、数据库字段设计文档 V1.3、管理后台前端开发文档 V1.1、小程序前端开发任务文档 V1.1

## 一、背景与问题

### 现状
- `users` 表无 `password` 字段；管理后台（web-frontend）登录目前输入"微信授权码 Code"，后端 `LoginMock` 直接以 code 充当 openid，无任何密码校验，安全性差。
- uni-app 小程序登录为"微信一键登录"，新用户自动静默注册，昵称默认"微信用户"，头像/手机号为空，用户体验与数据质量差。

### 目标
1. `users` 表新增 `password` 字段，管理后台登录改为 **nickname + password** 验证。
2. uni-app 新用户登录前必须先完善资料（头像、昵称、手机号），否则不予注册和登录。

## 二、已确认决策

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | password 适用前端 | 仅管理后台（web-frontend） |
| 2 | 密码初始化 | 平台管理员账号仅通过 SQL 手动插入（迁移脚本提供 bcrypt 生成示例） |
| 3 | uni-app 资料收集 | 微信官方组件（chooseAvatar / input type=nickname / getPhoneNumber） |
| 4 | phone 唯一性 | 唯一索引（部分唯一索引，排除空串） |
| 5 | 原 code 登录 | 保留 `/auth/login` 给 uni-app；新增独立 `/auth/admin-login` |
| 6 | 店主账号创建 | SQL 手动插入（不做管理接口） |
| 7 | 老用户处理 | 开发阶段不强制；按 openid 是否存在判断新/老用户 |
| 8 | 手机号解密 | 新版 `phonenumber.getPhoneNumber`（code 换手机号，需 access_token） |
| 9 | 头像存储 | chooseAvatar 临时文件上传至后端图床（pkg/imagebed 已有能力），存永久 URL |
| 10 | 注册接口 | 独立接口 `POST /auth/register` |

## 三、数据库变更

### 迁移脚本 `backend/migrations/007_auth_password.sql`

```sql
-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 007
-- 目标: users 表新增 password 字段 + phone 唯一索引 (密码登录 / 手机号唯一)
-- 版本: V1.4
-- =============================================================================

-- 1. password: bcrypt 哈希(60字符), 微信用户可为空, 平台管理员必填
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password VARCHAR(100);

COMMENT ON COLUMN users.password IS '登录密码, bcrypt 哈希; 平台管理员(role=1)必填, 微信小程序用户可为空';

-- 2. 清理重复手机号: 同一手机号只保留最早创建的一条, 其余置空
UPDATE users u
SET phone = NULL
WHERE phone IS NOT NULL AND phone <> ''
  AND u.id NOT IN (
      SELECT MIN(id) FROM users WHERE phone IS NOT NULL AND phone <> '' GROUP BY phone
  );

-- 3. phone 部分唯一索引 (排除空串, NULL 天然不冲突)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_uniq
    ON users (phone) WHERE phone IS NOT NULL AND phone <> '';

-- 4. 初始店主账号示例 (bcrypt 生成, 密码: 123456):
--    INSERT INTO users (id, openid, nickname, password, role)
--    VALUES (gen_random_uuid(), 'admin-bootstrap', 'admin',
--            '$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx', 1);
--    生成方式: python3 -c "import bcrypt; print(bcrypt.hashpw(b'123456', bcrypt.gensalt()).decode())"
```

### GORM 模型变更（`backend/internal/model/model.go`）

```go
type User struct {
    ...
    Nickname string `gorm:"type:varchar(32);not null"`
    Password string `gorm:"type:varchar(100)"` // 登录密码, bcrypt 哈希
    ...
}
```

### 文档同步（`docs/数据库字段设计文档_V1.3.md`）
- users 表新增 `password VARCHAR(100)` 行
- phone 索引改为唯一索引说明
- GORM 模型同步补充 Password 字段

## 四、后端接口设计

### 4.1 新增 `POST /auth/admin-login` — 管理后台密码登录

```
POST /auth/admin-login
```

**请求体：**
```json
{ "nickname": "admin", "password": "123456" }
```

**逻辑：**
1. 按 nickname 查询用户（repository 新增 `FindUserByNickname`）
2. 校验 bcrypt 密码
3. 校验 `role == PlatformRolePlatformAdmin(1)`，否则返回权限不足错误
4. 签发 JWT，返回与 `/auth/login` 一致的 `LoginResult` 结构

**响应：** 与 2.1 微信登录相同结构（access_token / expires_in / user）

**错误：**
- 用户不存在或密码错误 → `ErrInvalidCredentials`（新增错误码，区分"账号不存在"与"密码错误"统一提示，避免账号枚举）
- 非平台管理员 → 复用权限错误

### 4.2 改造 `POST /auth/login` — 小程序微信登录（新用户不自动注册）

**现状：** openid 不存在时自动创建用户（昵称"微信用户"）。
**改造：** openid 不存在时**不再创建**，返回响应体：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "need_profile": true,
    "access_token": null,
    "user": null
  }
}
```

`LoginResult` 结构新增 `NeedProfile bool` 字段。小程序前端收到 `need_profile=true` 跳转资料完善页。

老用户（openid 已存在）逻辑不变，直接返回 access_token + user（`need_profile=false`）。

### 4.3 新增 `POST /auth/register` — 新用户资料完善注册

```
POST /auth/register
```

**请求体：**
```json
{
  "code": "微信登录code",
  "nickname": "张三",
  "avatar_url": "https://图床返回的永久URL",
  "phone_code": "getPhoneNumber返回的code"
}
```

**逻辑：**
1. `code` 调 `Code2Session` 换 openid
2. `phone_code` 调 `phonenumber.getPhoneNumber` 解密手机号（需先取 access_token）
3. 校验：openid 未被占用；phone 未被占用（唯一索引兜底）
4. 创建用户：openid / nickname / avatar_url / phone
5. 签发 JWT，返回与登录一致结构

**注意：** 校验手机号是否已被其他用户绑定 → 返回 `ErrPhoneAlreadyBound`（复用现有错误码）。

### 4.4 新增 `POST /upload/avatar` — 公开头像上传

**说明：** 新用户在注册前（无 JWT token）需上传头像，因此本接口**不经过 JWTAuth 中间件**，是公开接口。

```
POST /upload/avatar   (multipart/form-data, 字段名: file)
```

**逻辑：**
1. 接收文件（复用现有图片校验能力：jpg/png/webp，≤2MB）
2. 调 imagebed `Client.Upload` 上传，得永久 URL
3. 返回 `{ "url": "https://..." }`

**安全考虑：** 公开上传存在滥用风险，通过图片类型 + 大小限制控制；如需进一步限制可后续加简单 token 校验。此接口同时可供既有用户后续改头像复用。

### 4.5 `WechatService` 扩展（`backend/internal/service/wechat.go`）

新增两个方法：
```go
// GetAccessToken 获取全局 access_token (缓存至过期前5分钟)
func (s *WechatService) GetAccessToken(ctx context.Context) (string, error)

// GetPhoneNumber 用 getPhoneNumber 返回的 code 解密手机号 (微信新版)
func (s *WechatService) GetPhoneNumber(ctx context.Context, code string) (string, error)
```

- `GetAccessToken`：`GET https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=&secret=`，返回 `access_token` + `expires_in`。注意在 service 内做简单缓存（内存 + 过期时间），避免每次调用都请求微信。
- `GetPhoneNumber`：`POST https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=`，body `{"code": "..."}`，响应 `phone_info.phoneNumber`。

### 4.6 AuthService 扩展（`backend/internal/service/auth.go`）

- `AdminLogin(ctx, nickname, password) (*LoginResult, error)`
- `Register(ctx, code, nickname, avatarURL, phoneCode) (*LoginResult, error)`
- `Login(ctx, code)` 改造：openid 不存在时返回 `NeedProfile: true`
- 密码加密/校验：bcrypt（`golang.org/x/crypto/bcrypt`，需 `go get`）

### 4.7 AuthRepository 扩展（`backend/internal/repository/auth.go`）

- `FindUserByNickname(ctx, nickname) (*model.User, error)`
- `CountUserByPhone(ctx, phone) (int64, error)`（可选，唯一索引兜底后可不做；保留用于友好错误提示）

### 4.8 错误码新增（`backend/internal/errors/errors.go`）

- `ErrInvalidCredentials`：账号或密码错误

### 4.9 路由注册（`backend/cmd/server/main.go`）

```go
auth.POST("/login", authH.Login)          // 已有, 保留 (小程序)
auth.POST("/admin-login", authH.AdminLogin) // 新增 (管理后台)
auth.POST("/register", authH.Register)      // 新增 (小程序新用户资料完善)

// 公开上传 (不挂 JWTAuth)
v1.POST("/upload/avatar", authH.UploadAvatar) // 新增 (注册前头像上传)
```

## 五、管理后台（web-frontend）改造

### 5.1 登录页 `src/views/login/index.vue`
- 输入框：由"微信授权码 Code"改为 **昵称** + **密码** 两个输入框
- 调 `authAPI.adminLogin({ nickname, password })`
- 校验逻辑保留：仅平台管理员可进（`isPlatformAdmin`）

### 5.2 API `src/api/auth.ts`
```typescript
adminLogin: (data: { nickname: string; password: string }) =>
  client.post<LoginResult>('/auth/admin-login', data),
```

## 六、uni-app 小程序改造

### 6.1 登录流程

```
登录页(微信一键登录) → POST /auth/login (code)
  ├─ need_profile=false → 进入工单列表 (老用户)
  └─ need_profile=true → 跳转资料完善页
```

### 6.2 新增资料完善页 `pages/auth/profile.vue`

- **头像**：`<button open-type="chooseAvatar" @chooseavatar="onChooseAvatar">` 获取临时文件 → 调已有图片上传接口（图床）→ 得永久 URL
- **昵称**：`<input type="nickname" v-model="nickname" />`（微信自带昵称填充）
- **手机号**：`<button open-type="getPhoneNumber" @getphonenumber="onGetPhone">` 获取 `code`
- **提交**：`POST /auth/register { code, nickname, avatar_url, phone_code }` → 成功进入工单列表
- 校验：昵称必填（微信组件不强制，需前端校验非空）；头像、手机号通过授权按钮获取

### 6.3 用户 store `stores/user.ts`
- `login(code)`：解析响应，若 `need_profile` 则返回标记（不 setToken）
- 新增 `register(data)`：提交资料完善，成功后写入 token/userInfo

### 6.4 图片上传
复用项目已有图床上传能力（后端 `pkg/imagebed` / 前端现有上传接口），将 chooseAvatar 临时文件转为永久 URL。

## 七、文档更新

| 文档 | 变更 |
|------|------|
| `docs/后端接口设计文档v1.0.md` | 新增 2.4 admin-login、2.5 register；更新 2.1 登录响应增加 need_profile |
| `docs/数据库字段设计文档_V1.3.md` | users 表 password 字段、phone 唯一索引 |
| `web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md` | 登录页改造说明、admin-login API |
| `uni-app-frontend/docs/新泥报修系统-小程序前端开发任务文档V1.1.md` | 登录流程改造、资料完善页说明 |

## 八、改动文件清单

**后端：**
1. `backend/migrations/007_auth_password.sql`（新增）
2. `backend/internal/model/model.go`（User 加 Password）
3. `backend/internal/errors/errors.go`（ErrInvalidCredentials）
4. `backend/internal/repository/auth.go`（FindUserByNickname）
5. `backend/internal/service/wechat.go`（GetAccessToken / GetPhoneNumber）
6. `backend/internal/service/auth.go`（AdminLogin / Register / Login 改造）
7. `backend/internal/handler/auth.go`（AdminLogin / Register / UploadAvatar handler）
8. `backend/internal/handler/upload.go` 或并入 auth.go（头像上传校验 + imagebed 调用）
9. `backend/cmd/server/main.go`（路由）
10. `backend/go.mod` / `go.sum`（新增 golang.org/x/crypto）

**管理后台（web-frontend）：**
11. `web-frontend/src/api/auth.ts`（adminLogin）
12. `web-frontend/src/views/login/index.vue`（登录页改造）

**小程序（uni-app）：**
13. `uni-app-frontend/pages/auth/profile.vue`（新增，资料完善页）
14. `uni-app-frontend/pages/auth/login.vue`（登录流程适配 need_profile）
15. `uni-app-frontend/stores/user.ts`（login 改造 + register）
16. `uni-app-frontend/utils/request.ts`（新增 uploadAvatar）
17. `uni-app-frontend/pages.json`（注册新页面）
18. `uni-app-frontend/types/index.ts`（LoginResult 增加 need_profile）

**文档：**
19. `docs/后端接口设计文档v1.0.md`
20. `docs/数据库字段设计文档_V1.3.md`
21. `web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md`
22. `uni-app-frontend/docs/新泥报修系统-小程序前端开发任务文档V1.1.md`

## 九、验证清单

1. 后端 `go build ./...` 通过
2. 迁移 007 在 PostgreSQL 执行成功，phone 唯一索引生效
3. SQL 插入店主账号后可 `POST /auth/admin-login` 成功登录
4. 错误密码 / 不存在账号 / 非管理员 登录均被拒绝
5. 小程序：新 openid → `/auth/login` 返回 `need_profile=true`；完善资料后 `/auth/register` 成功并可登录
6. 小程序：老 openid → `/auth/login` 直接返回 token
7. 重复手机号注册被拒绝
8. 管理后台登录页显示昵称+密码输入框
9. 注册前 `POST /upload/avatar` 可上传头像返回永久 URL
