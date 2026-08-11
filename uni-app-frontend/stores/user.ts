import { defineStore } from 'pinia'
import { http } from '@/utils/request'
import type { LoginResult, UserInfo } from '@/types'
import { useEnterpriseStore } from './enterprise'

interface UserState {
  token: string
  userInfo: UserInfo | null
  isLoggedIn: boolean
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    token: (uni.getStorageSync('token') as string) || '',
    userInfo: null,
    isLoggedIn: false
  }),
  actions: {
    /** 微信 code 换取 JWT；新用户需完善资料时返回 needProfile 标记 */
    async login(code: string): Promise<{ needProfile: boolean }> {
      const data = await http.post<LoginResult>('/auth/login', { code })
      if (data.need_profile) {
        return { needProfile: true }
      }
      if (!data.user) {
        throw new Error('登录响应缺少用户信息')
      }
      this.token = data.access_token
      this.userInfo = data.user
      this.isLoggedIn = true
      uni.setStorageSync('token', this.token)
      useEnterpriseStore().syncFromUserInfo(data.user.enterprises)
      return { needProfile: false }
    },
    /** 新用户资料完善注册；phone_code(微信授权) 与 phone(手动输入) 至少传一个 */
    async register(data: { code: string; nickname: string; avatar_url: string; phone_code?: string; phone?: string }) {
      const res = await http.post<LoginResult>('/auth/register', data)
      this.token = res.access_token
      this.userInfo = res.user
      this.isLoggedIn = true
      uni.setStorageSync('token', this.token)
      if (res.user) {
        useEnterpriseStore().syncFromUserInfo(res.user.enterprises)
      }
      return res
    },
    /** 获取当前用户信息（App 启动校验 / 我的页刷新） */
    async fetchUserInfo() {
      const data = await http.get<UserInfo>('/auth/me')
      this.userInfo = data
      this.isLoggedIn = true
      useEnterpriseStore().syncFromUserInfo(data.enterprises)
      return data
    },
    setToken(token: string) {
      this.token = token
      this.isLoggedIn = true
      uni.setStorageSync('token', token)
    },
    logout() {
      this.token = ''
      this.userInfo = null
      this.isLoggedIn = false
      uni.removeStorageSync('token')
      uni.removeStorageSync('currentEnterpriseId')
    }
  }
})
