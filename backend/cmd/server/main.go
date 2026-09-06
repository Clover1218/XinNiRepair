// 电脑维修店报修系统 — 服务入口
//
// 职责:
//  1. 加载配置
//  2. 初始化日志 / 数据库
//  3. 注册中间件和路由
//  4. 优雅启动 / 关闭 HTTP 服务
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"xin-ni-repair/internal/config"
	"xin-ni-repair/internal/handler"
	"xin-ni-repair/internal/middleware"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/imagebed"
	applogger "xin-ni-repair/pkg/logger"
	"xin-ni-repair/pkg/response"
)

func main() {

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, err := applogger.New(applogger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
		File: applogger.FileConfig{
			Path:       cfg.Log.File.Path,
			MaxSize:    cfg.Log.File.MaxSize,
			MaxBackups: cfg.Log.File.MaxBackups,
			MaxAge:     cfg.Log.File.MaxAge,
			Compress:   cfg.Log.File.Compress,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	logger.Error("applyEnvOverrides called", zap.String("db_host", os.Getenv("DB_HOST")))
	logger.Info("Configuration loaded",
		zap.String("mode", cfg.Server.Mode),
		zap.String("addr", cfg.Server.Addr()),
	)

	// ── 3. 设置 Gin 模式 ──
	gin.SetMode(cfg.Server.Mode)

	// ── 4. 初始化数据库 ──
	ctx := context.Background()
	db, err := repository.New(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("Database connection failed", zap.Error(err))
	}

	repository.InitAdminUser(db.DB)

	defer db.Close()
	logger.Info("Database connected",
		zap.String("host", cfg.Database.Host),
		zap.String("db", cfg.Database.DBName),
	)

	// ── 5. 创建 Gin 引擎 ──
	engine := gin.New()

	// ── 6. 注册全局中间件 ──
	engine.Use(
		middleware.Recovery(logger),
		middleware.RequestID(),
		middleware.Logger(logger),
		middleware.CORS(
			cfg.CORS.AllowedOrigins,
			cfg.CORS.AllowedMethods,
			cfg.CORS.AllowedHeaders,
			cfg.CORS.AllowCredentials,
			cfg.CORS.MaxAge,
		),
	)

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
	// 初始化店主账号 (幂等, 首次启动自动创建, 密码可通过 ADMIN_PASSWORD 配置)
	authH := handler.NewAuthHandler(authSvc, imgBed, logger)

	// 订阅消息推送 (依赖微信 access_token 与用户 openid)
	notifier := service.NewOrderNotifier(wechatSvc, authRepo, logger)

	entRepo := repository.NewEnterpriseRepository(db.DB)
	memRepo := repository.NewMembershipRepository(db.DB)
	projectRepo := repository.NewProjectRepository(db.DB)
	entSvc := service.NewEnterpriseService(entRepo, memRepo, logger)
	accessSvc := service.NewAccessService(memRepo)
	entH := handler.NewEnterpriseHandler(entSvc, accessSvc, logger)

	orderRepo := repository.NewOrderRepository(db.DB)
	imgRepo := repository.NewOrderImageRepository(db.DB)
	tlRepo := repository.NewOrderTimelineRepository(db.DB)
	orderSvc := service.NewOrderService(orderRepo, imgRepo, tlRepo, memRepo, projectRepo, imgBed, notifier, logger)
	orderH := handler.NewOrderHandler(orderSvc, logger)

	adminOrderSvc := service.NewAdminOrderService(orderRepo, imgRepo, tlRepo, accessSvc, imgBed, notifier, logger)
	exportSvc := service.NewOrderExportService(orderRepo, tlRepo, cfg.Shop.Name, logger)
	userAdminSvc := service.NewUserAdminService(authRepo, logger)
	projectSvc := service.NewProjectService(projectRepo, logger)
	adminH := handler.NewAdminHandler(adminOrderSvc, entSvc, exportSvc, userAdminSvc, accessSvc, logger)
	projectH := handler.NewProjectHandler(projectSvc, logger)

	registerRoutes(engine, db, authH, entH, orderH, adminH, projectH, tokenSvc)

	// ── 8. 启动 HTTP 服务 ──
	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Server starting", zap.String("addr", cfg.Server.Addr()))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited gracefully")
}

// registerRoutes 注册所有 API 路由
func registerRoutes(r *gin.Engine, db *repository.DB, authH *handler.AuthHandler, entH *handler.EnterpriseHandler, orderH *handler.OrderHandler, adminH *handler.AdminHandler, projectH *handler.ProjectHandler, tokenSvc *service.TokenService) {
	// ── 健康检查 ──
	r.GET("/health", func(c *gin.Context) {
		ctx, canc := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer canc()

		if err := db.Health(ctx); err != nil {
			response.Error(c, http.StatusServiceUnavailable, 5000, fmt.Sprintf("database unhealthy: %v", err))
			return
		}
		response.OK(c, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// ── API v1 ──
	v1 := r.Group("/api/v1")
	{
		// 公开接口: 协议文档 (返回纯 HTML, 不需要 JWT)
		v1.GET("/agreement/user", handler.UserAgreement)
		v1.GET("/agreement/privacy", handler.PrivacyPolicy)

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

		// 企业管理接口
		enterprises := v1.Group("/enterprises")
		enterprises.Use(middleware.JWTAuth(tokenSvc))
		{
			// 创建企业: 仅店方角色 (role=1 维修业务员 / role=2 超级管理员, 3.1)
			enterprises.POST("", middleware.RequirePlatformAdmin(), entH.Create)
			enterprises.POST("/join", entH.Join)     // 仅凭邀请码加入 (3.4)
			enterprises.GET("/join", entH.JoinByGet) // 加入企业 (GET 版, 扫码场景)
			enterprises.GET("/:enterprise_id", entH.Get)
			// 单位内管理接口: 该企业单位审核员(membership.role=1) 或 店方角色, handler 内统一校验
			enterprises.PUT("/:enterprise_id", entH.Update)
			enterprises.POST("/:enterprise_id/refresh/code", entH.RefreshCode)
			enterprises.GET("/:enterprise_id/members", entH.ListMembers)
			enterprises.PUT("/:enterprise_id/members/approve", entH.Approve)
			enterprises.PUT("/:enterprise_id/members/reject", entH.Reject)
			enterprises.DELETE("/:enterprise_id/members/:user_id", entH.Remove)
		}

		// 报修工单接口 (用户端)
		orders := v1.Group("/orders")
		orders.Use(middleware.JWTAuth(tokenSvc))
		{
			orders.GET("/options", orderH.Options)
			orders.POST("", orderH.Create)
			orders.GET("", orderH.List)
			orders.GET("/:order_id", orderH.Detail)
			orders.PUT("/:order_id", orderH.Update)
			orders.DELETE("/:order_id", orderH.Delete)
			orders.POST("/:order_id/submit", orderH.Submit)
			orders.POST("/:order_id/cancel", orderH.Cancel)
			orders.POST("/:order_id/images", orderH.UploadImage)
		}

		// 管理后台接口 (第五章+第六章): 双层角色控制
		admin := v1.Group("/admin")
		admin.Use(middleware.JWTAuth(tokenSvc))
		{
			// ── 仅店方角色 (维修业务员 role>=1 / 超级管理员) ──
			staff := admin.Group("")
			staff.Use(middleware.RequirePlatformAdmin())
			{
				staff.GET("/repairers", adminH.Repairers)                         // 维修员列表 (5.15)
				staff.GET("/orders/export", adminH.ExportOrders)                  // 导出工单记录 (5.14)
				staff.GET("/enterprises", adminH.ListEnterprises)                 // 企业列表 (5.8)
				staff.GET("/enterprises/:enterprise_id", adminH.EnterpriseDetail) // 企业详情 (5.9)
			}

			// ── 工单处理: 店方角色或单位审核员(限本单位), 服务内按 Operator 校验 ──
			admin.GET("/orders", adminH.ListOrders) // 工单列表 (5.1)
			admin.GET("/orders/:order_id", adminH.OrderDetail)
			admin.POST("/orders/:order_id/audit", adminH.Audit)                  // 审核通过 (5.3)
			admin.POST("/orders/:order_id/review", adminH.Audit)                 // 兼容旧版 /review (行为同 audit)
			admin.POST("/orders/:order_id/accept", adminH.Accept)                // 接单 (5.4)
			admin.POST("/orders/:order_id/reject", adminH.Reject)                // 退回 (5.5)
			admin.POST("/orders/:order_id/complete", adminH.Complete)            // 完工 (5.6)
			admin.POST("/orders/:order_id/reopen", adminH.Reopen)                // 重新打开 (5.16)
			admin.POST("/orders/:order_id/finance", adminH.UpdateFinance)        // 修改对账信息 (5.6.1)
			admin.POST("/orders/:order_id/receipts", adminH.UploadReceipt)       // 上传收据 (5.7)
			admin.GET("/enterprises/:enterprise_id/members", adminH.ListMembers) // 成员列表 (5.10)

			// ── 用户管理 (第六章, 仅超级管理员) ──
			users := admin.Group("/users")
			users.Use(middleware.RequireSuperAdmin())
			{
				users.GET("", adminH.ListUsers)                              // 用户列表 (6.1)
				users.GET("/:user_id", adminH.UserDetail)                    // 用户详情 (6.2)
				users.PUT("/:user_id", adminH.UpdateUser)                    // 更新用户属性 (6.3)
				users.POST("/:user_id/reset-password", adminH.ResetPassword) // 重置密码 (6.4)
			}

			// ── 项目字典管理 (6.5, 仅超级管理员) ──
			super := admin.Group("")
			super.Use(middleware.RequireSuperAdmin())
			{
				super.GET("/categories", projectH.ListCategories)                 // 大类列表 (6.5.1)
				super.POST("/categories", projectH.CreateCategory)                // 新增大类
				super.PUT("/categories/:category_id", projectH.UpdateCategory)    // 修改大类
				super.DELETE("/categories/:category_id", projectH.DeleteCategory) // 软删除大类
				super.GET("/properties", projectH.ListProperties)                 // 属性列表 (6.5.2)
				super.POST("/properties", projectH.CreateProperty)                // 新增属性
				super.PUT("/properties/:property_id", projectH.UpdateProperty)    // 修改属性
				super.DELETE("/properties/:property_id", projectH.DeleteProperty) // 软删除属性
				super.GET("/problems", projectH.ListProblems)                     // 常见问题列表 (6.5.3)
				super.POST("/problems", projectH.CreateProblem)                   // 新增常见问题
				super.PUT("/problems/:problem_id", projectH.UpdateProblem)        // 修改常见问题
				super.DELETE("/problems/:problem_id", projectH.DeleteProblem)     // 软删除常见问题
			}
		}
	}

	// 404
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, 4000, "接口不存在")
	})
}
