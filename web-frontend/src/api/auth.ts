import client from './client'
import type { LoginResult, UserInfo } from '@/types'

export const authAPI = {
  // 2.1 微信回调登录
  login: (code: string) => client.post<LoginResult>('/auth/login', { code }),

  // 2.4 管理后台密码登录
  adminLogin: (data: { nickname: string; password: string }) =>
    client.post<LoginResult>('/auth/admin-login', data),

  // 2.2 获取当前用户信息
  getMe: () => client.get<UserInfo>('/auth/me')
}
