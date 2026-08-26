/**
 * 全局配置
 *
 * ⚠️ BASE_URL 部署说明（Docker 单入口架构）：
 * 后端容器(backend:8080) 不对外暴露端口，所有 API 请求统一通过前端 Nginx 的 /api/* 反代转发。
 * 因此 uni-app 小程序必须指向"前端 Nginx 的主机地址 + /api/v1"，而不是后端 8080。
 *
 * 请按实际部署环境修改：
 *  - 本地联调（HBuilderX 运行到微信开发者工具）：
 *      开发机 IP + FRONTEND_EXPOSE_PORT，例：http://192.168.1.10/api/v1
 *  - 生产部署（上 HTTPS + 备案域名 / 微信小程序合法域名）：
 *      https://repair.your-domain.com/api/v1
 *  - 微信开发者工具里若勾选了"不校验合法域名"，可用 http + 局域网 IP 临时调试
 */
// 1.13.15.207:5004
// 127.0.0.1:8080
// 'https://services.cloverstia.site/api/v1'
export const BASE_URL = 'http://127.0.0.1:8080/api/v1'
export const IS_LOCAL = true
