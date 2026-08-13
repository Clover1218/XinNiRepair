# 第一阶段：编译（使用轻量级 Alpine 镜像）
FROM golang:1.26-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

# 复制依赖文件并下载（利用 Docker 缓存，加快构建速度）
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源码并编译
COPY . .
RUN go build -o /app/main ./cmd/server

# 第二阶段：运行（极小镜像）
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求，设置时区
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /root/

# 从编译阶段复制可执行文件
COPY --from=builder /app/main .

# 复制配置文件（程序默认读取 ./config/config.yaml）
COPY --from=builder /app/config ./config

# 复制数据库迁移脚本
COPY --from=builder /app/migrations ./migrations

# 暴露内部端口（Go 程序监听的端口，比如 8080）
EXPOSE 8080

# 启动命令
CMD ["./main"]
