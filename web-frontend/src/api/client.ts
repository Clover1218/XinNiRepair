import axios from 'axios'
import { ElMessage } from 'element-plus'

const client = axios.create({
  // 相对路径，由 Vite dev server 代理到本地后端（见 vite.config.ts server.proxy），避免 CORS
  baseURL: '/api/v1',
  timeout: 10000
})

client.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

client.interceptors.response.use(
  response => {
    // blob 响应（如 5.14 导出 Excel）直接返回，不按 JSON 校验
    if (response.config.responseType === 'blob') {
      return response
    }
    const data = response.data
    if (data.code !== 0) {
      // 业务错误
      ElMessage.error(data.message || '操作失败')
      return Promise.reject(data)
    }
    return data
  },
  error => {
    if (error.response?.status === 401) {
      // Token 过期或未登录
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
      window.location.href = '/login'
    } else {
      ElMessage.error(error.response?.data?.message || error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export default client
