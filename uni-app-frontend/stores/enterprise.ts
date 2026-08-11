import { defineStore } from 'pinia'
import { http } from '@/utils/request'
import { useUserStore } from './user'
import type { EnterpriseBrief } from '@/types'

interface EnterpriseState {
  /** 当前上下文企业 ID */
  currentEnterpriseId: string
  enterprises: EnterpriseBrief[]
}

export const useEnterpriseStore = defineStore('enterprise', {
  state: (): EnterpriseState => ({
    currentEnterpriseId: (uni.getStorageSync('currentEnterpriseId') as string) || '',
    enterprises: []
  }),
  actions: {
    setCurrent(id: string) {
      this.currentEnterpriseId = id
      uni.setStorageSync('currentEnterpriseId', id)
    },
    /**
     * 从用户信息同步企业列表：
     * 无当前企业时默认选中第一个已通过的企业
     */
    syncFromUserInfo(enterprises: EnterpriseBrief[]) {
      this.enterprises = enterprises || []
      if (!this.currentEnterpriseId && this.enterprises.length > 0) {
        const first = this.enterprises.find((e) => e.status === 'approved') || this.enterprises[0]
        this.setCurrent(first.enterprise_id)
      }
    },
    /** 切换当前企业：后端重新签发 JWT，前端同步 token */
    async switchEnterprise(enterpriseId: string) {
      const userStore = useUserStore()
      const data = await http.post<{ access_token: string }>('/auth/switch-enterprise', {
        enterprise_id: enterpriseId
      })
      if (data && data.access_token) {
        userStore.setToken(data.access_token)
      }
      this.setCurrent(enterpriseId)
    }
  }
})
