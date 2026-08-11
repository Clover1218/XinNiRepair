// Package logger 提供基于 zap 的结构化日志, 支持控制台/文件双输出与自动轮转。
package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置
type Config struct {
	Level  string // debug | info | warn | error
	Format string // console | json
	Output string // stdout | file
	File   FileConfig
}

// FileConfig 文件输出配置
type FileConfig struct {
	Path       string
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // days
	Compress   bool
}

// New 创建 zap.Logger
func New(cfg Config) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)
	encoder := newEncoder(cfg.Format, cfg.Output)
	writer := newWriter(cfg)
	core := zapcore.NewCore(encoder, writer, level)

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	return logger, nil
}

// NewNop 空日志 (测试用)
func NewNop() *zap.Logger {
	return zap.NewNop()
}

func parseLevel(s string) zapcore.LevelEnabler {
	switch s {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func newEncoder(format, output string) zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if format == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	// console 模式: 仅输出到终端时启用彩色, 写入文件时禁用, 避免 ANSI 转义码
	if output == "stdout" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func newWriter(cfg Config) zapcore.WriteSyncer {
	if cfg.Output == "file" {
		if err := os.MkdirAll(filepath.Dir(cfg.File.Path), 0755); err != nil {
			// 回退到 stdout
			return zapcore.AddSync(os.Stdout)
		}
		lumber := &lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSize,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,
			Compress:   cfg.File.Compress,
		}
		return zapcore.AddSync(lumber)
	}
	return zapcore.AddSync(os.Stdout)
}
