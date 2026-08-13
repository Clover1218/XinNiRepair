# 第一阶段：构建 Vue 项目
FROM node:22-alpine AS build

WORKDIR /app

# 启用 corepack 并使用 pnpm（项目锁文件是 pnpm-lock.yaml）
RUN corepack enable && corepack prepare pnpm@latest --activate

# 复制依赖并安装（pnpm 会读取 lockfile 保证与开发环境一致的依赖树）
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com

# 复制源码并打包
COPY . .
RUN pnpm build

# 第二阶段：使用 Nginx 运行
FROM nginx:stable-alpine

# 删除 Nginx 默认页面
RUN rm -rf /usr/share/nginx/html/*

# 复制构建好的 dist 文件到 Nginx 目录
COPY --from=build /app/dist /usr/share/nginx/html

# 复制自定义 Nginx 配置（下一步创建）
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 暴露 80 端口
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
