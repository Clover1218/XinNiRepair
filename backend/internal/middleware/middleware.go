// Package middleware 提供 Gin 中间件: 请求日志、异常恢复、CORS、请求ID。
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RequestID 注入/传递请求追踪ID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()[:8]
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// Logger 请求日志中间件 (zap)
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", clientIP),
			zap.Duration("latency", latency),
			zap.Int("body_size", c.Writer.Size()),
		}

		if rid, ok := c.Get("request_id"); ok {
			fields = append(fields, zap.String("request_id", rid.(string)))
		}

		errs := c.Errors.ByType(gin.ErrorTypePrivate)
		if len(errs) > 0 {
			errSlice := make([]error, len(errs))
			for i, e := range errs {
				errSlice[i] = e
			}
			fields = append(fields, zap.Errors("errors", errSlice))
		}

		if status >= 500 {
			logger.Error("Server error", fields...)
		} else if status >= 400 {
			logger.Warn("Client error", fields...)
		} else {
			logger.Info("Request", fields...)
		}
	}
}

// Recovery 异常恢复中间件
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rcv := recover(); rcv != nil {
				rid, _ := c.Get("request_id")
				logger.Error("Panic recovered",
					zap.Any("panic", rcv),
					zap.String("request_id", rid.(string)),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(500, gin.H{
					"code":       5000,
					"message":    "服务器内部错误",
					"request_id": rid,
				})
			}
		}()
		c.Next()
	}
}

// CORS 跨域中间件
func CORS(allowedOrigins []string, allowedMethods []string, allowedHeaders []string, allowCreds bool, maxAge time.Duration) gin.HandlerFunc {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowCreds {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 预检请求
		if c.Request.Method == "OPTIONS" {
			if originSet[origin] || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*") {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Access-Control-Allow-Methods", joinStrings(allowedMethods))
			c.Header("Access-Control-Allow-Headers", joinStrings(allowedHeaders))
			c.Header("Access-Control-Max-Age", maxAge.String())
			c.AbortWithStatus(204)
			return
		}

		// 普通请求
		if originSet[origin] || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*") {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Next()
	}
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += ", " + ss[i]
	}
	return result
}
