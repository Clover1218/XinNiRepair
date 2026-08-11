// Package service 承载业务逻辑, 遵循 service/repository 分层架构。
package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"xin-ni-repair/internal/config"
	apperrors "xin-ni-repair/internal/errors"
)

// TokenClaims JWT 自定义声明
type TokenClaims struct {
	UserID string `json:"uid"`
	Role   int    `json:"role"` // 平台角色: 0-普通用户 1-平台管理员
	jwt.RegisteredClaims
}

// TokenService 负责 JWT 签发与校验
type TokenService struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// NewTokenService 创建 TokenService
func NewTokenService(cfg config.JWTConfig) *TokenService {
	return &TokenService{
		secret:    []byte(cfg.Secret),
		issuer:    cfg.Issuer,
		accessTTL: cfg.AccessTokenTTL,
	}
}

// GenerateAccessToken 生成访问令牌, 返回 token 与有效期
func (s *TokenService) GenerateAccessToken(userID string, role int) (string, time.Duration, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	claims := TokenClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, s.accessTTL, nil
}

// ParseToken 解析并校验 JWT, 返回用户ID与平台角色
func (s *TokenService) ParseToken(tokenStr string) (string, int, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	}
	if s.issuer != "" {
		opts = append(opts, jwt.WithIssuer(s.issuer))
	}

	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	}, opts...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", 0, apperrors.ErrTokenExpired
		}
		return "", 0, apperrors.ErrTokenInvalid
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return "", 0, apperrors.ErrTokenInvalid
	}
	return claims.UserID, claims.Role, nil
}
