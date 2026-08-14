// Package repository 提供数据库连接管理和基础数据访问。
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"xin-ni-repair/internal/config"
	"xin-ni-repair/internal/model"
)

// DB 封装 GORM 连接
type DB struct {
	DB *gorm.DB
}

// New 创建数据库连接池, 若 cfg.AutoMigrate=true 则同步执行 GORM AutoMigrate。
// AutoMigrate 只做"新增表/新增列/新增索引"的安全变更, 不会删除任何已有列。
func New(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		TranslateError: true, // 将数据库错误翻译为 gorm.ErrDuplicatedKey 等 sentinel 错误
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// 连接测试
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	// 启动时 AutoMigrate: 仅当显示开启时执行
	if cfg.AutoMigrate {
		migrateCtx, mc := context.WithTimeout(ctx, 30*time.Second)
		defer mc()
		if err := db.WithContext(migrateCtx).AutoMigrate(
			&model.Enterprise{},
			&model.User{},
			&model.Membership{},
			&model.RepairOrder{},
			&model.OrderImage{},
			&model.OrderTimeline{},
		); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("gorm auto migrate failed: %w", err)
		}
	}

	return &DB{DB: db}, nil
}

// Close 关闭连接池
func (db *DB) Close() {
	if sqlDB, err := db.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// Health 健康检查
func (db *DB) Health(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
