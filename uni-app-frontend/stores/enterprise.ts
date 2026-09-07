import { defineStore } from 'pinia'
import type { EnterpriseBrief } from '@/types'

interface EnterpriseState {
  /** 当前上下文企业 ID（V1.2：纯本地上下文，用于列表默认过滤/首页待办卡） */
  currentEnterpriseId: string
  enterprises: EnterpriseBrief[]
}

export const useEnterpriseStore = defineStore('enterprise', {
  state: (): EnterpriseState => ({
    currentEnterpriseId: (uni.getStorageSync('currentEnterpriseId') as string) || '',
    enterprises: []
  }),
  actions: {
    /** 本地上下文切换（V1.2：不再依赖 /auth/switch-enterprise） */
    setCurrent(id: string) {
      this.currentEnterpriseId = id
      uni.setStorageSync('currentEnterpriseId', id)
    },
    /**
     * 从用户信息同步企业列表：
     * 保持原上下文企业；若已失效/不存在则默认选中第一个已通过(approved)的单位。
     */
    syncFromUserInfo(enterprises: EnterpriseBrief[]) {
      this.enterprises = enterprises || []
      const approved = this.enterprises.filter((e) => e.status === 'approved')
      const valid =
        this.currentEnterpriseId &&
        approved.some((e) => e.enterprise_id === this.currentEnterpriseId)
      if (!valid) {
        const next = approved[0]
        if (next) this.setCurrent(next.enterprise_id)
        else if (this.currentEnterpriseId) this.setCurrent('')
      }
    },
    /** 当前上下文企业名称 */
    currentEnterpriseName(): string {
      const ent = this.enterprises.find((e) => e.enterprise_id === this.currentEnterpriseId)
      return ent ? ent.enterprise_name : ''
    }
  }
})
