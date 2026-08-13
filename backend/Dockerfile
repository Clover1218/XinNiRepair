# 第一阶段：编译（使用轻量级 Alpine 镜像）
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 复制依赖文件并下载（利用 Docker 缓存，加快构建速度）
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源码并编译
COPY . .
# 注意：如果你的 main.go 不在根目录，请修改这里的路径，例如 cd ./cmd/server && go build ...
RUN go build -o /app/main ./cmd/server

# 第二阶段：运行（极小镜像）
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求，设置时区
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /root/

# 从编译阶段复制可执行文件
COPY --from=builder /app/main .

# 暴露内部端口（Go 程序监听的端口，比如 8080）
EXPOSE 8080

# 启动命令
CMD ["./main"]