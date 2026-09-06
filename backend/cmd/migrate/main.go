// 迁移工具: 逐条执行 SQL 迁移文件 (与 GORM AutoMigrate 互补, 用于重命名/改列/回填等 DDL)
//
// 用法:
//
//	go run ./cmd/migrate migrations/008_v1_4_state_dict.sql
//	# 或一次多个文件
//	go run ./cmd/migrate migrations/001_init.sql migrations/008_v1_4_state_dict.sql
//
// 说明: 将文件按分号拆分为单条语句执行 (本仓库迁移文件均为简单 DDL/DML, 不含 DO 块/
// 函数体); 逐条执行保证错误定位, 失败即中止。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"xin-ni-repair/internal/config"
	"xin-ni-repair/internal/repository"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <migration.sql> [more.sql...]")
		os.Exit(1)
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	db, err := repository.New(context.Background(), cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	for _, file := range os.Args[1:] {
		if err := runFile(db.DB, file); err != nil {
			fmt.Fprintf(os.Stderr, "migration %s failed: %v\n", file, err)
			os.Exit(1)
		}
		fmt.Printf("migration %s ok\n", file)
	}
}

func runFile(db *gorm.DB, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	statements := splitStatements(string(raw))
	for i, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("statement #%d failed: %w\nSQL: %s", i+1, err, stmt)
		}
	}
	return nil
}

// splitStatements 按分号拆分语句; 先剔除整行注释 (--), 避免注释内分号干扰。
// 适用于无 DO 块/过程体/行内注释的迁移文件。
func splitStatements(sql string) []string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	parts := strings.Split(b.String(), ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
