import { defineStore } from 'pinia'
import { http, rawLoginRequest, wxLoginCode } from '@/utils/request'
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
    /** 写入登录态（token + user + 企业上下文），供 login/register/silent 共用 */
    applyLogin(result: LoginResult) {
      if (!result.user) return
      this.token = result.access_token
      this.userInfo = result.user
      this.isLoggedIn = true
      uni.setStorageSync('token', this.token)
      useEnterpriseStore().syncFromUserInfo(result.user.enterprises)
    },
    /** 微信 code 换取 JWT；新用户需完善资料时返回 needProfile 标记 */
    async login(code: string): Promise<{ needProfile: boolean }> {
      const data = await http.post<LoginResult>('/auth/login', { code })
      if (data.need_profile) {
        return { needProfile: true }
      }
      if (!data.user) {
        throw new Error('登录响应缺少用户信息')
      }
      this.applyLogin(data)
      return { needProfile: false }
    },
    /**
     * 静默续期（问题 2）：本地无 token 时用 wx.login 换新 token，无需用户操作。
     * 成功返回 true；全新用户(need_profile)/网络失败返回 false（由页面引导去登录页）。
     * 注意：有 token 直接放行——过期由请求层 401 先静默续期再重放兜底。
     */
    async ensureLoggedIn(): Promise<boolean> {
      if (this.token) return true
      try {
        const code = await wxLoginCode()
        const data = await rawLoginRequest(code)
        if (data.need_profile || !data.user) return false
        this.applyLogin(data)
        return true
      } catch (e) {
        return false
      }
    },
    /** 新用户资料完善注册；phone_code(微信授权) 与 phone(手动输入) 至少传一个 */
    async register(data: {
      code: string
      nickname: string
      avatar_url: string
      phone_code?: string
      phone?: string
    }) {
      const res = await http.post<LoginResult>('/auth/register', data)
      this.applyLogin(res)
      return res
    },
    /** 获取当前用户信息（我的页刷新 / 首页模式判定） */
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
