package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "xin-ni-repair/internal/errors"
	"xin-ni-repair/internal/service"
	"xin-ni-repair/pkg/response"
)

// JWTAuth 校验 Bearer Token, 并将 user_id 与平台角色 role 注入上下文
func JWTAuth(tokenSvc *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, apperrors.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, role, err := tokenSvc.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			response.FailError(c, err)
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("role", role)
		c.Next()
	}
}

// RequirePlatformAdmin 校验平台管理员 (JWT 中 role=1)
func RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := c.Get("role")
		if !ok || role.(int) != 1 {
			response.Fail(c, apperrors.ErrNotAdmin)
			c.Abort()
			return
		}
		c.Next()
	}
}
