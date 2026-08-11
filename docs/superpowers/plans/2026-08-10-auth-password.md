# 密码登录与小程序资料完善 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** users 表新增 password 字段；管理后台登录改为 nickname+password；uni-app 小程序新用户登录前强制完善头像/昵称/手机号。

**Architecture:** 后端新增 `/auth/admin-login`（nickname+password，bcrypt 校验，仅 role=1）、`/auth/register`（新用户资料完善注册，用微信新版 code 换手机号）、`/upload/avatar`（注册前公开头像上传到图床）；`/auth/login` 改为新用户返回 `need_profile=true` 不自动注册。管理后台登录页改为昵称+密码；小程序新增资料完善页。

**Tech Stack:** Go + GORM + gin + bcrypt（golang.org/x/crypto）；Vue 3 + Element Plus；uni-app + wot-design-uni。

**设计文档:** `docs/superpowers/specs/2026-08-10-auth-password-design.md`

---

### Task 0: 数据库迁移 - users 新增 password + phone 唯一索引

**Files:**
- Create: `backend/migrations/007_auth_password.sql`

- [ ] **Step 1: 创建迁移文件**

创建 `backend/migrations/007_auth_password.sql`，格式对齐 006 迁移：

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

- [ ] **Step 2: 提交**

```bash
git add backend/migrations/007_auth_password.sql
git commit -m "feat(migration): users 新增 password 字段 + phone 唯一索引"
```

---

### Task 1: 模型层 - User 新增 Password 字段

**Files:**
- Modify: `backend/internal/model/model.go:164-176`

- [ ] **Step 1: 修改模型定义**

在 `User` 结构体 `Nickname` 字段之后新增：

```go
type User struct {
	ID           string        `gorm:"primaryKey;type:uuid"`
	Openid       string        `gorm:"type:varchar(64);not null;uniqueIndex"`
	Unionid      string        `gorm:"type:varchar(64);index"`
	Nickname     string        `gorm:"type:varchar(32);not null"`
	Password     string        `gorm:"type:varchar(100)"` // 登录密码, bcrypt 哈希; 平台管理员必填, 微信用户可为空
	AvatarUrl    string        `gorm:"type:varchar(512)"`
	Phone        string        `gorm:"type:varchar(20);index"`
	CreatedAt    time.Time     `gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `gorm:"autoUpdateTime"`
	Role         int           `gorm:"type:smallint;not null;default:1;index"`
	Memberships  []Membership  `gorm:"foreignKey:UserID"`
	RepairOrders []RepairOrder `gorm:"foreignKey:ReporterID"`
}
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/internal/model/model.go
git commit -m "feat(model): User 新增 Password 字段"
```

---

### Task 2: 错误码 - 新增 ErrInvalidCredentials

**Files:**
- Modify: `backend/internal/errors/errors.go`

- [ ] **Step 1: 新增错误码**

在认证错误区（`ErrWechatAuthFailed` 之后）新增：

```go
// 认证错误 (1000-1999)
var (
	ErrUnauthorized       = &BizError{Code: 1000, Message: "未登录或登录已过期"}
	ErrTokenInvalid       = &BizError{Code: 1001, Message: "Token无效"}
	ErrTokenExpired       = &BizError{Code: 1002, Message: "Token已过期"}
	ErrWechatAuthFailed   = &BizError{Code: 1010, Message: "微信授权失败"}
	ErrInvalidCredentials = &BizError{Code: 1011, Message: "账号或密码错误"}
)
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/internal/errors/errors.go
git commit -m "feat(errors): 新增 ErrInvalidCredentials 错误码"
```

---

### Task 3: Repository - 新增 FindUserByNickname

**Files:**
- Modify: `backend/internal/repository/auth.go`

- [ ] **Step 1: 新增方法**

在 `FindUserByOpenid` 方法之后新增：

```go
// FindUserByNickname 按昵称查询用户, 不存在时返回 nil (管理后台登录用)
func (r *AuthRepository) FindUserByNickname(ctx context.Context, nickname string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("nickname = ?", nickname).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 3: 提交**

```bash
git add backend/internal/repository/auth.go
git commit -m "feat(repo): 新增 FindUserByNickname"
```

---

### Task 4: WechatService - 新增 GetAccessToken / GetPhoneNumber

**Files:**
- Modify: `backend/internal/service/wechat.go`

- [ ] **Step 1: 引入依赖与状态**

在 `WechatService` 结构体新增 access_token 缓存字段：

```go
// WechatService 封装微信接口调用
type WechatService struct {
	appID     string
	appSecret string
	httpClient *http.Client

	mu            sync.Mutex // 保护 access_token 缓存
	accessToken   string
	accessTokenAt time.Time
}
```

在文件 imports 中新增 `"sync"`。

- [ ] **Step 2: 新增 GetAccessToken 方法**

在 `Code2Session` 方法之后新增：

```go
// getAccessTokenURL 获取全局 access_token 接口地址
const accessTokenURL = "https://api.weixin.qq.com/cgi-bin/token"

// GetAccessToken 获取全局 access_token, 内存缓存至过期前 5 分钟
func (s *WechatService) GetAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Since(s.accessTokenAt) < 7000*time.Second {
		return s.accessToken, nil
	}

	params := url.Values{}
	params.Set("grant_type", "client_credential")
	params.Set("appid", s.appID)
	params.Set("secret", s.appSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, accessTokenURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	if result.AccessToken == "" {
		return "", apperrors.ErrWechatAPI.WithMessage(fmt.Sprintf("获取 access_token 失败(%d): %s", result.Errcode, result.Errmsg))
	}

	s.accessToken = result.AccessToken
	s.accessTokenAt = time.Now()
	return s.accessToken, nil
}
```

- [ ] **Step 3: 新增 GetPhoneNumber 方法**

```go
// getPhoneNumberURL 手机号解密接口地址
const getPhoneNumberURL = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"

// GetPhoneNumber 用 getPhoneNumber 返回的 code 解密手机号 (微信新版规范)
func (s *WechatService) GetPhoneNumber(ctx context.Context, code string) (string, error) {
	token, err := s.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, getPhoneNumberURL+"?access_token="+url.QueryEscape(token), bytes.NewReader(payload))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}

	var result struct {
		Errcode   int    `json:"errcode"`
		Errmsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", apperrors.ErrWechatAPI.WithError(err)
	}
	if result.Errcode != 0 || result.PhoneInfo.PhoneNumber == "" {
		return "", apperrors.ErrWechatAPI.WithMessage(fmt.Sprintf("手机号解密失败(%d): %s", result.Errcode, result.Errmsg))
	}
	return result.PhoneInfo.PhoneNumber, nil
}
```

注意：需在文件 imports 中新增 `"bytes"`（若未引入）。

- [ ] **Step 4: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service/wechat.go
git commit -m "feat(wechat): 新增 GetAccessToken 与 GetPhoneNumber"
```

---

### Task 5: AuthService - AdminLogin / Register / Login 改造

**Files:**
- Modify: `backend/internal/service/auth.go`
- Modify: `backend/go.mod` / `go.sum`（新增 golang.org/x/crypto）

- [ ] **Step 1: 新增依赖**

Run: `cd backend && go get golang.org/x/crypto/bcrypt`
Expected: 下载成功，go.mod 新增依赖

- [ ] **Step 2: LoginResult 新增 NeedProfile 字段**

```go
// LoginResult 微信登录结果
type LoginResult struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int64        `json:"expires_in"`
	User        *UserProfile `json:"user"`
	NeedProfile bool         `json:"need_profile"` // true=需先完善资料(新用户), 由 /auth/register 完成
}
```

- [ ] **Step 3: Login 改造 - 新用户不自动注册**

将 `Login` 方法（微信 code 登录）中"user == nil 时创建用户"的分支改为返回 `NeedProfile: true`：

```go
	if user == nil {
		return &LoginResult{NeedProfile: true}, nil
	}
```

注意：当前 handler `Login` 调用的是 `LoginMock`（自动注册版）。改造后：
- `LoginMock` 方法整体删除（管理后台改用 `AdminLogin`，小程序改用 `Login`，无人再调用）
- handler `Login` 改为调用 `s.svc.Login(...)`（见 Task 6 Step 1 一并修改）

- [ ] **Step 4: 新增 AdminLogin 方法**

在 `Login` 方法之后新增：

```go
// AdminLogin 管理后台密码登录 (2.4): nickname + password, 仅平台管理员
func (s *AuthService) AdminLogin(ctx context.Context, nickname, password string) (*LoginResult, error) {
	user, err := s.users.FindUserByNickname(ctx, nickname)
	if err != nil {
		s.logger.Error("find user by nickname failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	if user == nil || user.Password == "" {
		return nil, apperrors.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}
	if user.Role != model.PlatformRolePlatformAdmin {
		return nil, apperrors.ErrNotAdmin
	}

	tokenStr, ttl, err := s.token.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		s.logger.Error("generate access token failed", zap.Error(err))
		return nil, apperrors.ErrInternal.WithError(err)
	}

	profile, err := s.buildProfile(ctx, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: tokenStr,
		ExpiresIn:   int64(ttl.Seconds()),
		User:        profile,
	}, nil
}
```

- [ ] **Step 5: 新增 Register 方法**

```go
// Register 新用户资料完善注册 (2.5): 微信 code 换 openid + phone_code 解密手机号, 创建用户并签发 JWT
func (s *AuthService) Register(ctx context.Context, code, nickname, avatarURL, phoneCode string) (*LoginResult, error) {
	session, err := s.wechat.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}

	// openid 已被占用: 已有用户直接登录
	existing, err := s.users.FindUserByOpenid(ctx, session.Openid)
	if err != nil {
		s.logger.Error("find user by openid failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	if existing != nil {
		return s.issueToken(ctx, existing)
	}

	phone, err := s.wechat.GetPhoneNumber(ctx, phoneCode)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:        uuid.New().String(),
		Openid:    session.Openid,
		Unionid:   session.Unionid,
		Nickname:  nickname,
		AvatarUrl: avatarURL,
		Phone:     phone,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		// 唯一索引冲突: phone 已被占用
		if isUniqueViolation(err) {
			return nil, apperrors.ErrPhoneAlreadyBound.WithMessage("该手机号已被其他账号绑定")
		}
		s.logger.Error("create user failed", zap.Error(err))
		return nil, apperrors.ErrDatabaseError.WithError(err)
	}
	s.logger.Info("new user registered via profile", zap.String("user_id", user.ID))

	return s.issueToken(ctx, user)
}
```

- [ ] **Step 6: 新增 issueToken 辅助方法**

```go
// issueToken 生成 JWT 并组装登录结果
func (s *AuthService) issueToken(ctx context.Context, user *model.User) (*LoginResult, error) {
	tokenStr, ttl, err := s.token.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		s.logger.Error("generate access token failed", zap.Error(err))
		return nil, apperrors.ErrInternal.WithError(err)
	}
	profile, err := s.buildProfile(ctx, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: tokenStr,
		ExpiresIn:   int64(ttl.Seconds()),
		User:        profile,
	}, nil
}
```

- [ ] **Step 7: 新增 isUniqueViolation 辅助方法**

在文件末尾新增（依赖 PostgreSQL 错误码 23505）：

```go
// isUniqueViolation 判断是否为唯一约束冲突 (PostgreSQL 错误码 23505)
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

在文件 imports 中新增 `"errors"` 和 `"github.com/jackc/pgx/v5/pgconn"`。若项目未引入 pgx，改用以下替代实现（不依赖 pgx，用字符串判断）：

```go
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
```

（二选一，推荐字符串判断版本，避免新增依赖。）

- [ ] **Step 8: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 9: 提交**

```bash
git add backend/internal/service/auth.go backend/go.mod backend/go.sum
git commit -m "feat(service): AdminLogin/Register, Login 新用户返回 need_profile"
```

---

### Task 6: AuthHandler - AdminLogin / Register / UploadAvatar

**Files:**
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/handler/upload.go`（新增）

- [ ] **Step 1: handler Login 改用真实 Login + AuthHandler 注入 imagebed**

修改 `Login` handler，将 `LoginMock` 改为 `Login`：

```go
// Login 微信回调登录 (POST /auth/login)
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少微信登录 code"))
		return
	}

	result, err := h.svc.Login(c.Request.Context(), req.Code)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}
```

修改 `AuthHandler` 结构体与构造函数（新增 imagebed）：

```go
// AuthHandler 认证接口处理器
type AuthHandler struct {
	svc    *service.AuthService
	imgBed *imagebed.Client
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(svc *service.AuthService, imgBed *imagebed.Client) *AuthHandler {
	return &AuthHandler{svc: svc, imgBed: imgBed}
}
```

在 imports 中新增 `"xin-ni-repair/pkg/imagebed"`。

- [ ] **Step 2: 新增 AdminLogin handler**

```go
// AdminLogin 管理后台密码登录 (POST /auth/admin-login, 2.4)
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少昵称或密码"))
		return
	}
	result, err := h.svc.AdminLogin(c.Request.Context(), req.Nickname, req.Password)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}
```

- [ ] **Step 3: 新增 Register handler**

```go
// Register 新用户资料完善注册 (POST /auth/register, 2.5)
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Code      string `json:"code" binding:"required"`
		Nickname  string `json:"nickname" binding:"required"`
		AvatarURL string `json:"avatar_url"`
		PhoneCode string `json:"phone_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少必填字段"))
		return
	}
	result, err := h.svc.Register(c.Request.Context(), req.Code, req.Nickname, req.AvatarURL, req.PhoneCode)
	if err != nil {
		response.FailError(c, err)
		return
	}
	response.OK(c, result)
}
```

- [ ] **Step 4: 新建 upload.go 头像上传 handler**

创建 `backend/internal/handler/upload.go`：

```go
package handler

import (
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/pkg/response"
)

// maxAvatarSize 头像大小上限 2MB
const maxAvatarSize = 2 << 20

// UploadAvatar 公开头像上传 (POST /upload/avatar, multipart/form-data, 字段名 file)
// 注册前调用, 不经过 JWTAuth; 仅做大小/格式校验后转交图床
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperrors.ErrInvalidParam.WithMessage("缺少 file 文件"))
		return
	}
	if file.Size <= 0 || file.Size > maxAvatarSize {
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("头像大小需不超过 2MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		response.Fail(c, apperrors.ErrImageInvalid.WithMessage("仅支持 jpg/png/webp 格式"))
		return
	}

	f, err := file.Open()
	if err != nil {
		response.FailError(c, err)
		return
	}
	defer f.Close()

	result, err := h.imgBed.Upload(c.Request.Context(), file.Filename, f)
	if err != nil {
		response.Fail(c, apperrors.ErrOSSUpload.WithMessage("头像上传失败"))
		return
	}
	response.OK(c, gin.H{"url": result.URL})
}
```

注意：需要 `"path/filepath"` import（若未列出请补上）。

- [ ] **Step 5: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 6: 提交**

```bash
git add backend/internal/handler/auth.go backend/internal/handler/upload.go
git commit -m "feat(handler): AdminLogin/Register/UploadAvatar"
```

---

### Task 7: 路由注册 + main.go 装配

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: NewAuthHandler 传参**

修改 `main.go` 中 AuthHandler 构造：

```go
	authSvc := service.NewAuthService(authRepo, tokenSvc, wechatSvc, logger)
	authH := handler.NewAuthHandler(authSvc, imgBed)
```

注意：`imgBed` 变量定义在 authH 之后，需将 `imgBed := imagebed.New(...)` 的声明移到 `authH := ...` 之前。调整顺序为：

```go
	// ── 7. 注册路由 ──
	authRepo := repository.NewAuthRepository(db.DB)
	tokenSvc := service.NewTokenService(cfg.JWT)
	wechatSvc := service.NewWechatService(cfg.Wechat)

	imgBed := imagebed.New(imagebed.Config{
		Endpoint: cfg.ImageBed.Endpoint,
		Token:    cfg.ImageBed.Token,
		Timeout:  cfg.ImageBed.Timeout,
	})

	authSvc := service.NewAuthService(authRepo, tokenSvc, wechatSvc, logger)
	authH := handler.NewAuthHandler(authSvc, imgBed)

	entRepo := repository.NewEnterpriseRepository(db.DB)
	memRepo := repository.NewMembershipRepository(db.DB)
	entSvc := service.NewEnterpriseService(entRepo, memRepo, logger)
	entH := handler.NewEnterpriseHandler(entSvc)

	orderRepo := repository.NewOrderRepository(db.DB)
	imgRepo := repository.NewOrderImageRepository(db.DB)
	tlRepo := repository.NewOrderTimelineRepository(db.DB)
	orderSvc := service.NewOrderService(orderRepo, imgRepo, tlRepo, memRepo, imgBed, logger)
	orderH := handler.NewOrderHandler(orderSvc)

	adminOrderSvc := service.NewAdminOrderService(orderRepo, imgRepo, tlRepo, imgBed, logger)
	exportSvc := service.NewOrderExportService(orderRepo, tlRepo, cfg.Shop.Name, logger)
	adminH := handler.NewAdminHandler(adminOrderSvc, entSvc, exportSvc)
```

- [ ] **Step 2: 注册路由**

在 v1 分组中新增：

```go
	v1 := r.Group("/api/v1")
	{
		// 认证接口
		v1.POST("/auth/login", authH.Login)
		v1.POST("/auth/admin-login", authH.AdminLogin) // 管理后台密码登录 (2.4)
		v1.POST("/auth/register", authH.Register)      // 新用户资料完善注册 (2.5)
		v1.POST("/upload/avatar", authH.UploadAvatar)  // 注册前公开头像上传 (2.6)

		auth := v1.Group("/auth")
		auth.Use(middleware.JWTAuth(tokenSvc))
		{
			auth.GET("/me", authH.Me)
			auth.PUT("/bind-phone", authH.BindPhone)
		}
```

- [ ] **Step 3: 编译验证**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(route): 注册 admin-login/register/upload-avatar"
```

---

### Task 8: 管理后台 - 登录页改昵称+密码

**Files:**
- Modify: `web-frontend/src/api/auth.ts`
- Modify: `web-frontend/src/views/login/index.vue`

- [ ] **Step 1: admin.ts 新增 adminLogin**

```typescript
export const authAPI = {
  // 2.1 微信回调登录
  login: (code: string) => client.post<LoginResult>('/auth/login', { code }),

  // 2.4 管理后台密码登录
  adminLogin: (data: { nickname: string; password: string }) =>
    client.post<LoginResult>('/auth/admin-login', data),

  // 2.2 获取当前用户信息
  getMe: () => client.get<UserInfo>('/auth/me')
}
```

- [ ] **Step 2: 登录页改为昵称+密码**

将 `web-frontend/src/views/login/index.vue` 的 `code` ref 改为两个 ref：

```typescript
const nickname = ref('')
const password = ref('')
```

`handleLogin` 改为：

```typescript
const handleLogin = async () => {
  if (!nickname.value.trim() || !password.value) {
    ElMessage.warning('请输入昵称和密码')
    return
  }
  loading.value = true
  try {
    const res = await authAPI.adminLogin({ nickname: nickname.value.trim(), password: password.value })
    userStore.setUser(res.data.user, res.data.access_token)

    // 判断是否为平台管理员（JWT role：1=平台管理员，0=普通用户）
    if (!userStore.isPlatformAdmin) {
      ElMessage.error('该账号无管理权限')
      userStore.logout()
      return
    }

    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || ''
    router.push(redirect || '/enterprises')
  } finally {
    loading.value = false
  }
}
```

模板部分，将 `v-model="code"` 的 el-form-item 替换为两个：

```html
      <el-form @submit.prevent>
        <el-form-item>
          <el-input
            v-model="nickname"
            placeholder="请输入昵称"
            size="large"
            clearable
            @keyup.enter="handleLogin"
          >
            <template #prefix>
              <el-icon><User /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          >
            <template #prefix>
              <el-icon><Key /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-button
          type="primary"
          size="large"
          class="login-btn"
          :loading="loading"
          @click="handleLogin"
        >
          登 录
        </el-button>
      </el-form>
```

注意：`User` 图标需确认已从 `@element-plus/icons-vue` 导入（与 `Key` 相同方式）。

- [ ] **Step 3: 类型检查**

Run: `cd web-frontend && npx vue-tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add web-frontend/src/api/auth.ts web-frontend/src/views/login/index.vue
git commit -m "feat(admin-web): 登录改为昵称+密码"
```

---

### Task 9: 小程序 - 类型 + store 改造

**Files:**
- Modify: `uni-app-frontend/types/index.ts`
- Modify: `uni-app-frontend/stores/user.ts`
- Modify: `uni-app-frontend/utils/request.ts`

- [ ] **Step 1: types/index.ts 修改 LoginResult**

```typescript
export interface LoginResult {
  access_token: string
  expires_in: number
  user: UserInfo | null
  /** true=需先完善资料(新用户), 调 /auth/register 后获取完整登录结果 */
  need_profile?: boolean
}
```

注意：`user` 由 `UserInfo` 改为 `UserInfo | null`（need_profile 时后端返回 null）。

- [ ] **Step 2: stores/user.ts 的 login 改造**

```typescript
    /** 微信 code 换取 JWT；新用户需完善资料时返回 need_profile 标记 */
    async login(code: string): Promise<{ needProfile: boolean }> {
      const data = await http.post<LoginResult>('/auth/login', { code })
      if (data.need_profile) {
        return { needProfile: true }
      }
      if (!data.user) {
        throw new Error('登录响应缺少用户信息')
      }
      this.token = data.access_token
      this.userInfo = data.user
      this.isLoggedIn = true
      uni.setStorageSync('token', this.token)
      useEnterpriseStore().syncFromUserInfo(data.user.enterprises)
      return { needProfile: false }
    },
```

新增 register action：

```typescript
    /** 新用户资料完善注册 */
    async register(data: { code: string; nickname: string; avatar_url: string; phone_code: string }) {
      const res = await http.post<LoginResult>('/auth/register', data)
      this.token = res.access_token
      this.userInfo = res.user
      this.isLoggedIn = true
      uni.setStorageSync('token', this.token)
      if (res.user) {
        useEnterpriseStore().syncFromUserInfo(res.user.enterprises)
      }
      return res
    },
```

- [ ] **Step 3: request.ts 新增 uploadAvatar**

在 `uploadReceipt` 之后新增：

```typescript
/**
 * 头像上传（POST /upload/avatar，multipart/form-data）
 * 注册前公开接口，无需 token；单张 ≤2MB，jpg/png/webp
 */
export function uploadAvatar(filePath: string): Promise<{ url: string }> {
  return new Promise((resolve, reject) => {
    uni.uploadFile({
      url: `${BASE_URL}/upload/avatar`,
      filePath,
      name: 'file',
      success: (res) => {
        try {
          const body = JSON.parse(res.data) as ApiResponse
          if (body.code === 0) {
            resolve(body.data as never)
          } else {
            uni.showToast({ title: body.message || '上传失败', icon: 'none' })
            reject(body)
          }
        } catch (e) {
          reject(e)
        }
      },
      fail: (err) => {
        uni.showToast({ title: '上传失败，请重试', icon: 'none' })
        reject(err)
      }
    })
  })
}
```

- [ ] **Step 4: 类型检查**

Run: `cd uni-app-frontend && npx tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 5: 提交**

```bash
git add uni-app-frontend/types/index.ts uni-app-frontend/stores/user.ts uni-app-frontend/utils/request.ts
git commit -m "feat(uni): 登录响应支持 need_profile, 新增 register/uploadAvatar"
```

---

### Task 10: 小程序 - 登录页适配 + 资料完善页

**Files:**
- Modify: `uni-app-frontend/pages/auth/login.vue`
- Create: `uni-app-frontend/pages/auth/profile.vue`
- Modify: `uni-app-frontend/pages.json`

- [ ] **Step 1: pages.json 注册新页面**

在 `pages/auth/login` 之后新增：

```json
    {
      "path": "pages/auth/profile",
      "style": {
        "navigationBarTitleText": "完善资料"
      }
    },
```

- [ ] **Step 2: login.vue handleLogin 适配 need_profile**

将 `handleLogin` 中登录成功跳转逻辑改为：

```typescript
    /** 微信一键登录：wx.login 拿 code -> POST /auth/login 换取 JWT */
    async handleLogin() {
      if (this.loading) return
      this.loading = true
      try {
        const code = await new Promise<string>((resolve, reject) => {
          uni.login({
            provider: 'weixin',
            success: (res) => resolve(res.code),
            fail: reject
          })
        })
        const { needProfile } = await this.userStore.login(code)
        if (needProfile) {
          // 新用户：先完善资料（头像/昵称/手机号）
          uni.redirectTo({ url: '/pages/auth/profile' })
          return
        }
        uni.showToast({ title: '登录成功', icon: 'success' })
        setTimeout(() => {
          uni.switchTab({ url: '/pages/order/list' })
        }, 500)
      } catch (e) {
        console.error('登录失败', e)
      } finally {
        this.loading = false
      }
    }
```

- [ ] **Step 3: 创建资料完善页 profile.vue**

创建 `uni-app-frontend/pages/auth/profile.vue`：

```vue
<template>
  <view class="profile-page">
    <view class="profile-card">
      <view class="profile-header">
        <view class="avatar-wrap" @click="chooseAvatar">
          <image v-if="avatarUrl" class="avatar-img" :src="avatarUrl" mode="aspectFill"></image>
          <view v-else class="avatar-placeholder">
            <text class="avatar-plus">＋</text>
            <text class="avatar-tip">选择头像</text>
          </view>
        </view>
        <view class="avatar-hint">头像用于展示与工单报修人识别</view>
      </view>

      <view class="form-item">
        <text class="form-label">昵称</text>
        <input
          v-model="nickname"
          class="form-input"
          type="nickname"
          placeholder="请输入微信昵称"
          placeholder-class="form-placeholder"
        />
      </view>

      <view class="form-item">
        <text class="form-label">手机号</text>
        <button v-if="!phoneCode" class="phone-btn" open-type="getPhoneNumber" @getphonenumber="onGetPhone">
          微信授权获取手机号
        </button>
        <view v-else class="phone-done">已授权获取手机号</view>
      </view>

      <wd-button
        type="primary"
        size="large"
        block
        round
        :loading="submitting"
        loading-color="#ffffff"
        :disabled="!canSubmit"
        @click="handleSubmit"
      >
        完成注册
      </wd-button>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { useUserStore } from '@/stores/user'
import { uploadAvatar } from '@/utils/request'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore()
    }
  },
  data() {
    return {
      nickname: '',
      avatarUrl: '',
      avatarTempPath: '',
      phoneCode: '',
      submitting: false
    }
  },
  computed: {
    canSubmit(): boolean {
      return !!this.nickname.trim() && !!this.avatarUrl && !!this.phoneCode
    }
  },
  methods: {
    /** 选择头像（微信官方 chooseAvatar） */
    chooseAvatar() {
      // 使用 button open-type=chooseAvatar 方式获取，此处为占位；实际用以下方式触发
      uni.chooseMedia({
        count: 1,
        mediaType: ['image'],
        success: async (res) => {
          const temp = res.tempFiles[0].tempFilePath
          this.avatarTempPath = temp
          try {
            const { url } = await uploadAvatar(temp)
            this.avatarUrl = url
          } catch (e) {
            // 错误已由 uploadAvatar Toast
          }
        }
      })
    },
    /** 微信手机号授权回调 */
    onGetPhone(e: any) {
      if (e.detail && e.detail.code) {
        this.phoneCode = e.detail.code
      } else if (e.detail && e.detail.errMsg && !e.detail.errMsg.includes('ok')) {
        uni.showToast({ title: '手机号授权失败', icon: 'none' })
      }
    },
    /** 提交注册 */
    async handleSubmit() {
      if (this.submitting || !this.canSubmit) return
      this.submitting = true
      try {
        const code = await new Promise<string>((resolve, reject) => {
          uni.login({
            provider: 'weixin',
            success: (res) => resolve(res.code),
            fail: reject
          })
        })
        await this.userStore.register({
          code,
          nickname: this.nickname.trim(),
          avatar_url: this.avatarUrl,
          phone_code: this.phoneCode
        })
        uni.showToast({ title: '注册成功', icon: 'success' })
        setTimeout(() => {
          uni.switchTab({ url: '/pages/order/list' })
        }, 500)
      } catch (e) {
        console.error('注册失败', e)
      } finally {
        this.submitting = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.profile-page {
  min-height: 100vh;
  background: #f5f7fa;
  padding: 40rpx 32rpx;
  box-sizing: border-box;
}

.profile-card {
  background: #ffffff;
  border-radius: 24rpx;
  padding: 48rpx 40rpx;
}

.profile-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 48rpx;
}

.avatar-wrap {
  width: 144rpx;
  height: 144rpx;
  border-radius: 50%;
  overflow: hidden;
  background: #eef3ff;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2rpx dashed #4d80f0;
}

.avatar-img {
  width: 100%;
  height: 100%;
}

.avatar-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.avatar-plus {
  font-size: 48rpx;
  color: #4d80f0;
  line-height: 1;
}

.avatar-tip {
  font-size: 20rpx;
  color: #4d80f0;
  margin-top: 8rpx;
}

.avatar-hint {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: #999999;
}

.form-item {
  margin-bottom: 40rpx;
}

.form-label {
  display: block;
  font-size: 26rpx;
  color: #333333;
  margin-bottom: 16rpx;
}

.form-input {
  width: 100%;
  height: 88rpx;
  background: #f5f7fa;
  border-radius: 16rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  font-size: 28rpx;
}

.form-placeholder {
  color: #999999;
}

.phone-btn {
  width: 100%;
  height: 88rpx;
  background: #f5f7fa;
  border-radius: 16rpx;
  font-size: 28rpx;
  color: #4d80f0;
  line-height: 88rpx;
  padding: 0 24rpx;
  text-align: left;
}

.phone-done {
  width: 100%;
  height: 88rpx;
  background: #f5f7fa;
  border-radius: 16rpx;
  font-size: 28rpx;
  color: #67c23a;
  line-height: 88rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
}
</style>
```

注意：`chooseAvatar` 目前用 `uni.chooseMedia` 占位实现，微信小程序正式环境应改用 `<button open-type="chooseAvatar" @chooseavatar="onChooseAvatar">`。如需正式实现，在模板中替换为 button 并在回调中上传。两种方式都保留给实现者选择。

- [ ] **Step 4: 类型检查**

Run: `cd uni-app-frontend && npx tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 5: 提交**

```bash
git add uni-app-frontend/pages.json uni-app-frontend/pages/auth/login.vue uni-app-frontend/pages/auth/profile.vue
git commit -m "feat(uni): 登录流程适配 need_profile, 新增资料完善页"
```

---

### Task 11: 后端接口文档更新

**Files:**
- Modify: `docs/后端接口设计文档v1.0.md`

- [ ] **Step 1: 更新 2.1 微信登录响应**

在 2.1 节响应示例与说明中补充 need_profile：

```markdown
**说明**：微信回调后前端拿到 code，传给后端换取 JWT Token。
- 老用户（openid 已存在）：直接返回 access_token + user
- 新用户（openid 不存在）：**不自动注册**，返回 `need_profile: true`，前端引导完善资料后调 2.5 注册接口
```

在响应示例 data 中补充：

```json
{
  "access_token": "eyJhbGciOi...",
  "expires_in": 7200,
  "user": { ... },
  "need_profile": false
}
```

- [ ] **Step 2: 新增 2.4 管理后台密码登录**

在 2.3 绑定手机号之后新增：

```markdown
### 2.4 管理后台密码登录

```
POST /auth/admin-login
```

**说明**：管理后台登录，昵称 + 密码验证，仅平台管理员（`role=1`）可登录。

**请求体：**

```json
{
  "nickname": "admin",
  "password": "123456"
}
```

**响应**：同 2.1 微信登录（access_token / expires_in / user）。

**错误**：
- 账号或密码错误 → 错误码 1011
- 非平台管理员 → 错误码 2001
```

- [ ] **Step 3: 新增 2.5 新用户注册**

```markdown
### 2.5 新用户资料完善注册

```
POST /auth/register
```

**说明**：微信新用户完善资料（头像/昵称/手机号）后注册，返回完整登录结果。需先上传头像（2.6）获得 avatar_url。

**请求体：**

```json
{
  "code": "微信登录 code",
  "nickname": "张三",
  "avatar_url": "https://图床地址/xxx.jpg",
  "phone_code": "getPhoneNumber 返回的 code"
}
```

**错误**：
- 手机号已被其他账号绑定 → 错误码 4510
```

- [ ] **Step 4: 新增 2.6 头像上传**

```markdown
### 2.6 头像上传

```
POST /upload/avatar
```

**说明**：注册前公开上传头像（multipart/form-data，字段名 `file`），无需登录。单张 ≤2MB，仅支持 jpg/png/webp。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": { "url": "https://图床地址/xxx.jpg" }
}
```
```

- [ ] **Step 5: 提交**

```bash
git add docs/后端接口设计文档v1.0.md
git commit -m "docs: 新增 2.4/2.5/2.6 接口, 更新 2.1"
```

---

### Task 12: 数据库文档更新

**Files:**
- Modify: `docs/数据库字段设计文档_V1.3.md`

- [ ] **Step 1: users 表新增 password 行**

在 2.2 users 表 `nickname` 行之后新增：

```markdown
| 5 | password | VARCHAR(100) | — | NULL | 登录密码 bcrypt 哈希；平台管理员必填，微信用户可为空 |
```

并将后续序号顺延（原 5-9 → 6-10）。

- [ ] **Step 2: phone 唯一索引更新**

在"索引"表格中新增：

```markdown
| idx_users_phone_uniq | phone | UNIQUE(部分) | 手机号唯一（WHERE phone IS NOT NULL AND phone <> ''） |
```

- [ ] **Step 3: GORM 模型同步**

```go
type User struct {
    ID        string     `gorm:"primaryKey;type:uuid"`
    Openid    string     `gorm:"type:varchar(64);not null;uniqueIndex"`
    Unionid   string     `gorm:"type:varchar(64);index"`
    Nickname  string     `gorm:"type:varchar(32);not null"`
    Password  string     `gorm:"type:varchar(100)"` // 登录密码 bcrypt 哈希
    AvatarUrl string     `gorm:"type:varchar(512)"`
    Phone     string     `gorm:"type:varchar(20);index"`
    ...
}
```

- [ ] **Step 4: 提交**

```bash
git add docs/数据库字段设计文档_V1.3.md
git commit -m "docs(db): users 表 password 字段 + phone 唯一索引"
```

---

### Task 13: 前端/小程序文档更新

**Files:**
- Modify: `web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md`
- Modify: `uni-app-frontend/docs/新泥报修系统-小程序前端开发任务文档V1.1.md`

- [ ] **Step 1: 管理后台文档登录说明**

在登录页相关章节补充：

```markdown
**V1.1 更新：**
- 登录方式改为昵称 + 密码（`POST /auth/admin-login`），仅平台管理员可登录
- 原"微信授权码 Code"输入已移除
```

- [ ] **Step 2: 小程序文档登录流程更新**

```markdown
**登录流程更新：**
- 微信一键登录调 `POST /auth/login`
- 新用户（openid 不存在）返回 `need_profile: true`，跳转资料完善页 `pages/auth/profile`
- 资料完善页：头像（chooseAvatar/上传图床）+ 昵称（type=nickname）+ 手机号（getPhoneNumber）→ `POST /auth/register`
- 老用户直接进入工单列表
```

- [ ] **Step 3: 提交**

```bash
git add web-frontend/docs/新泥报修系统-管理后台前端开发文档V1.1.md uni-app-frontend/docs/新泥报修系统-小程序前端开发任务文档V1.1.md
git commit -m "docs: 登录与资料完善文档更新"
```

---

### Task 14: 全量验证

- [ ] **Step 1: 后端编译**

Run: `cd backend && go build ./...`
Expected: 无错误输出

- [ ] **Step 2: 前端类型检查**

Run: `cd web-frontend && npx vue-tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 3: 小程序类型检查**

Run: `cd uni-app-frontend && npx tsc --noEmit`
Expected: 无错误输出

- [ ] **Step 4: 功能自测（手工）**

1. 执行迁移 `007_auth_password.sql`
2. SQL 插入店主账号（bcrypt 生成密码）
3. 管理后台：昵称+密码登录成功；错误密码/非管理员被拒
4. 小程序：新 openid 登录 → 跳资料完善页；上传头像/填昵称/授权手机号 → 注册成功进入列表
5. 小程序：老 openid 登录 → 直接进入列表
