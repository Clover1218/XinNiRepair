#!/bin/bash
# ============================================================
# 管理用户角色脚本
# 通过 phone 或 nickname 搜索用户, 修改其 Role (0=普通用户, 1=平台管理员)
# 读取 .env 环境变量连接 Docker 内的 PostgreSQL
# ============================================================
set -euo pipefail

# ── 切换到项目根目录 (脚本所在目录的上一级) ──
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

# ── 加载 .env ──
if [ ! -f "$ENV_FILE" ]; then
  echo "错误: 未找到 .env 文件 ($ENV_FILE)"
  exit 1
fi
# 安全地 source .env (只导出 DB_ 开头的变量)
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

# ── 校验必要的环境变量 ──
: "${DB_USER:?请在 .env 中设置 DB_USER}"
: "${DB_NAME:?请在 .env 中设置 DB_NAME}"
DB_PASSWORD="${DB_PASSWORD:-}"

# ── Docker 容器名 (与 docker-compose.yml 中一致) ──
CONTAINER_NAME="repair-postgres"

# ── 检查容器是否运行 ──
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "错误: PostgreSQL 容器 '$CONTAINER_NAME' 未运行"
  echo "请先启动: docker compose up -d postgres"
  exit 1
fi

# ── psql 执行器: 在容器内运行 SQL ──
exec_sql() {
  docker exec -i "$CONTAINER_NAME" \
    psql -U "$DB_USER" -d "$DB_NAME" -t -A -F '|' "$@"
}

# ── 搜索用户 ──
search_users() {
  local keyword="$1"
  exec_sql -c "
    SELECT id, nickname, phone, role,
           CASE WHEN role = 1 THEN '平台管理员' ELSE '普通用户' END AS role_label
    FROM users
    WHERE phone ILIKE '%' || '${keyword}' || '%'
       OR nickname ILIKE '%' || '${keyword}' || '%'
    ORDER BY created_at DESC;
  "
}

# ── 修改用户角色 ──
update_role() {
  local user_id="$1"
  local new_role="$2"
  exec_sql -c "UPDATE users SET role = ${new_role} WHERE id = '${user_id}';"
}

# ── 主流程 ──
echo "============================================"
echo "  用户角色管理工具"
echo "============================================"
echo ""

read -r -p "请输入搜索关键词 (phone 或 nickname): " keyword

if [ -z "$keyword" ]; then
  echo "错误: 关键词不能为空"
  exit 1
fi

echo ""
echo "正在搜索..."
echo "--------------------------------------------"

results=$(search_users "$keyword")

if [ -z "$results" ] || [ "$results" = "" ]; then
  echo "未找到匹配的用户"
  exit 0
fi

# ── 显示搜索结果 ──
echo "找到以下用户:"
echo "--------------------------------------------"
printf "%-3s | %-36s | %-16s | %-20s | %s\n" "#" "ID" "Nickname" "Phone" "当前角色"
echo "--------------------------------------------"

# 读取结果到数组
mapfile -t lines <<< "$results"
user_ids=()

for i in "${!lines[@]}"; do
  IFS='|' read -r id nickname phone role role_label <<< "${lines[$i]}"
  user_ids+=("$id")
  printf "%-3s | %-36s | %-16s | %-20s | %s\n" "$((i+1))" "$id" "$nickname" "$phone" "$role_label"
done

echo "--------------------------------------------"
echo ""

# ── 选择用户 ──
read -r -p "输入序号选择用户 (1-${#lines[@]}): " choice

if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "${#lines[@]}" ]; then
  echo "错误: 无效的序号"
  exit 1
fi

selected_id="${user_ids[$((choice-1))]}"
echo "已选择用户: ${selected_id}"
echo ""

# ── 选择新角色 ──
echo "请选择新角色:"
echo "  0) 普通用户"
echo "  1) 平台管理员"
read -r -p "输入 0 或 1: " new_role

if [ "$new_role" != "0" ] && [ "$new_role" != "1" ]; then
  echo "错误: 角色只能是 0 或 1"
  exit 1
fi

role_name="普通用户"
if [ "$new_role" = "1" ]; then
  role_name="平台管理员"
fi

echo ""
read -r -p "确认将用户 ${selected_id} 的角色修改为 ${role_name}? (y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# ── 执行修改 ──
update_role "$selected_id" "$new_role"

if [ $? -eq 0 ]; then
  echo ""
  echo "✓ 修改成功! 用户 ${selected_id} 的角色已更新为: ${role_name}"
else
  echo ""
  echo "✗ 修改失败"
  exit 1
fi
