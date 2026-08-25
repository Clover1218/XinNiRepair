// Package config 提供基于 YAML 配置文件的加载能力，支持环境变量覆盖。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用总配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
	JWT      JWTConfig      `yaml:"jwt"`
	Wechat   WechatConfig   `yaml:"wechat"`
	OSS      OSSConfig      `yaml:"oss"`
	ImageBed ImageBedConfig `yaml:"image_bed"`
	Shop     ShopConfig     `yaml:"shop"`
	CORS     CORSConfig     `yaml:"cors"`
}

// ShopConfig 店铺信息 (用于导出表头等)
type ShopConfig struct {
	Name string `yaml:"name"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Mode            string        `yaml:"mode"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// Addr 返回监听地址
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	DBName          string        `yaml:"dbname"`
	SSLMode         string        `yaml:"sslmode"`
	AutoMigrate     bool          `yaml:"auto_migrate"` // 启动时 GORM AutoMigrate: 仅新建/加列加索引, 不会删列
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// DSN 返回 PostgreSQL 连接字符串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string        `yaml:"level"`
	Format string        `yaml:"format"`
	Output string        `yaml:"output"`
	File   LogFileConfig `yaml:"file"`
}

// LogFileConfig 日志文件配置
type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret          string        `yaml:"secret"`
	Issuer          string        `yaml:"issuer"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

// WechatConfig 微信公众号配置
type WechatConfig struct {
	AppID           string `yaml:"app_id"`
	AppSecret       string `yaml:"app_secret"`
	Token           string `yaml:"token"`
	EncodingAESKey  string `yaml:"encoding_aes_key"`
	MiniprogramState string `yaml:"miniprogram_state"` // 订阅消息跳转小程序版本: developer/trial/formal
}

// OSSConfig 对象存储配置
type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Bucket          string `yaml:"bucket"`
	BaseURL         string `yaml:"base_url"`
}

// ImageBedConfig 图床配置 (自建图床, 通过 multipart/form-data 上传)
type ImageBedConfig struct {
	Endpoint string        `yaml:"endpoint"` // 上传接口地址, 如 http://localhost:81/api/index.php
	Token    string        `yaml:"token"`    // 图床鉴权 token
	Timeout  time.Duration `yaml:"timeout"`  // 请求超时
}

// CORSConfig 跨域配置
type CORSConfig struct {
	AllowedOrigins   []string      `yaml:"allowed_origins"`
	AllowedMethods   []string      `yaml:"allowed_methods"`
	AllowedHeaders   []string      `yaml:"allowed_headers"`
	AllowCredentials bool          `yaml:"allow_credentials"`
	MaxAge           time.Duration `yaml:"max_age"`
}

// Load 从 YAML 文件加载配置并进行环境变量替换
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 环境变量替换: ${VAR} 或 ${VAR:default}
	content := expandEnv(string(data))

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(content), cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	return cfg, nil
}

// expandEnv 替换 ${VAR} 和 ${VAR:default} 语法
func expandEnv(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			start := i + 2
			end := strings.IndexByte(s[start:], '}')
			if end == -1 {
				result.WriteByte(s[i])
				continue
			}
			end += start
			content := s[start:end]
			if colon := strings.IndexByte(content, ':'); colon >= 0 {
				envVal := os.Getenv(content[:colon])
				if envVal == "" {
					envVal = content[colon+1:]
				}
				result.WriteString(envVal)
			} else {
				result.WriteString(os.Getenv(content))
			}
			i = end
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// applyEnvOverrides 环境变量覆盖: 若对应环境变量非空, 则覆盖 YAML 配置值。
// 映射关系与 .env.example 保持同步, 专为 Docker 容器化部署设计。
func (c *Config) applyEnvOverrides() {
	// ── 数据库 ──
	if v := os.Getenv("DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		c.Database.DBName = v
	}
	// 启动 AutoMigrate 开关: 1/true/yes 打开, 其它关
	if v := os.Getenv("DB_AUTO_MIGRATE"); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			c.Database.AutoMigrate = true
		case "0", "false", "no", "off":
			c.Database.AutoMigrate = false
		}
	}

	// ── 微信 ──
	if v := os.Getenv("WECHAT_APP_ID"); v != "" {
		c.Wechat.AppID = v
	}
	if v := os.Getenv("WECHAT_APP_SECRET"); v != "" {
		c.Wechat.AppSecret = v
	}

	// ── 图床 ──
	if v := os.Getenv("EASYIMAGE_ENDPOINT"); v != "" {
		c.ImageBed.Endpoint = v
	}
	if v := os.Getenv("EASYIMAGE_TOKEN"); v != "" {
		c.ImageBed.Token = v
	}
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 10 * time.Second
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "console"
	}
	if c.Log.Output == "" {
		c.Log.Output = "stdout"
	}
	if c.ImageBed.Timeout == 0 {
		c.ImageBed.Timeout = 30 * time.Second
	}
}
