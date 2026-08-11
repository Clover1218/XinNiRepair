# 电脑维修店报修系统 — Makefile
# Usage:
#   make run       —— 开发运行
#   make build     —— 编译
#   make test      —— 测试+竞态检测
#   make lint      —— 静态检查 (需要 golangci-lint)
#   make migrate   —— 执行数据库迁移 (需要 psql)
#   make dev       —— 清理→编译→运行

.PHONY: run build test lint clean migrate dev help

APP_NAME    := xin-ni-repair
ENTRY       := ./cmd/server
BUILD_DIR   := ./build
CONFIG_DIR  := ./config

# ── 开发运行 ──
run:
	go run $(ENTRY)/main.go

# ── 编译 ──
build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(ENTRY)

# ── 清理 + 编译 + 运行 ──
dev: clean build
	cp $(CONFIG_DIR)/config.yaml $(BUILD_DIR)/
	cd $(BUILD_DIR) && ./$(APP_NAME)

# ── 测试 ──
test:
	go test -race -count=1 ./...

# ── 测试 + 覆盖率 ──
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# ── 静态检查 ──
lint:
	golangci-lint run ./...

# ── 数据库迁移 ──
migrate:
	@echo "Running migration..."
	psql -h localhost -U postgres -d xin_ni_repair -f ./migrations/001_init.sql

# ── 依赖整理 ──
tidy:
	go mod tidy

# ── 清理 ──
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# ── 帮助 ──
help:
	@echo "Available targets:"
	@echo "  run      开发运行"
	@echo "  build    编译二进制"
	@echo "  dev      清理→编译→运行"
	@echo "  test     测试 + 竞态检测"
	@echo "  cover    测试 + 覆盖率报告"
	@echo "  lint     静态检查"
	@echo "  migrate  执行数据库迁移 (psql)"
	@echo "  tidy     下载 & 整理依赖"
	@echo "  clean    清理构建产物"
