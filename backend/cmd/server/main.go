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

	registerRoutes(engine, db, authH, entH, orderH, adminH, tokenSvc)

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
func registerRoutes(r *gin.Engine, db *repository.DB, authH *handler.AuthHandler, entH *handler.EnterpriseHandler, orderH *handler.OrderHandler, adminH *handler.AdminHandler, tokenSvc *service.TokenService) {
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
			enterprises.POST("", middleware.RequirePlatformAdmin(), entH.Create)
			enterprises.POST("/join", entH.Join)     // 仅凭邀请码加入 (3.4)
			enterprises.GET("/join", entH.JoinByGet) // 加入企业 (GET 版, 扫码场景)
			enterprises.GET("/:enterprise_id", entH.Get)
			enterprises.PUT("/:enterprise_id", middleware.RequirePlatformAdmin(), entH.Update)
			enterprises.POST("/:enterprise_id/refresh/code", middleware.RequirePlatformAdmin(), entH.RefreshCode)
			enterprises.GET("/:enterprise_id/members", middleware.RequirePlatformAdmin(), entH.ListMembers)
			enterprises.PUT("/:enterprise_id/members/approve", middleware.RequirePlatformAdmin(), entH.Approve)
			enterprises.PUT("/:enterprise_id/members/reject", middleware.RequirePlatformAdmin(), entH.Reject)
			enterprises.DELETE("/:enterprise_id/members/:user_id", middleware.RequirePlatformAdmin(), entH.Remove)
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

		// 管理后台接口 (第五章, 仅平台管理员)
		admin := v1.Group("/admin")
		admin.Use(middleware.JWTAuth(tokenSvc), middleware.RequirePlatformAdmin())
		{
			admin.GET("/repairers", adminH.Repairers)        // 维修员列表 (5.15)
			admin.GET("/orders/export", adminH.ExportOrders) // 导出工单记录 (5.14, 需在 :order_id 前注册)
			admin.GET("/orders", adminH.ListOrders)
			admin.GET("/orders/:order_id", adminH.OrderDetail)
			admin.POST("/orders/:order_id/review", adminH.Review)
			admin.POST("/orders/:order_id/accept", adminH.Accept)
			admin.POST("/orders/:order_id/reject", adminH.Reject)
			admin.POST("/orders/:order_id/complete", adminH.Complete)
			admin.POST("/orders/:order_id/finance", adminH.UpdateFinance) // 修改对账信息 (5.6.1)
			admin.POST("/orders/:order_id/receipts", adminH.UploadReceipt)
			admin.GET("/enterprises", adminH.ListEnterprises)
			admin.GET("/enterprises/:enterprise_id", adminH.EnterpriseDetail)
			admin.GET("/enterprises/:enterprise_id/members", adminH.ListMembers)
		}
	}

	// 404
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, 4000, "接口不存在")
	})
}
