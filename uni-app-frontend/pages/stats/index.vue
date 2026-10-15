<template>
  <view class="stats">
    <!-- 时间预设（仅企业统计需要；业务员统计是累计口径） -->
    <view v-if="isEnterprise" class="seg">
      <view
        v-for="p in presets"
        :key="p.key"
        class="seg-item"
        :class="{ 'seg-item-on': preset === p.key }"
        @click="onPreset(p.key)"
      >
        <text class="seg-txt">{{ p.label }}</text>
      </view>
    </view>

    <view v-if="loading" class="tip">加载中…</view>

    <template v-else>
      <!-- 企业数据统计（单位审核员 / 店方） -->
      <block v-if="isEnterprise">
        <view v-if="!metrics" class="tip">暂无统计数据</view>
        <block v-else>
          <view class="grid">
            <view class="metric">
              <text class="metric-label">待审核存量</text>
              <text class="metric-val metric-val-warn">{{ metrics.pending_review }}</text>
            </view>
            <view class="metric">
              <text class="metric-label">区间上报</text>
              <text class="metric-val">{{ metrics.submitted }}</text>
            </view>
            <view class="metric">
              <text class="metric-label">审核通过</text>
              <text class="metric-val metric-val-ok">{{ metrics.audited }}</text>
            </view>
            <view class="metric">
              <text class="metric-label">退回</text>
              <text class="metric-val metric-val-bad">{{ metrics.rejected }}</text>
            </view>
          </view>

          <view v-if="statusRows.length" class="panel">
            <view class="panel-title">状态分布</view>
            <view v-for="r in statusRows" :key="r.status" class="row">
              <text class="row-label">{{ r.label }}</text>
              <text class="row-val">{{ r.count }}</text>
            </view>
          </view>

          <view v-if="categoryRows.length" class="panel">
            <view class="panel-title">项目大类 Top5</view>
            <view v-for="(r, i) in categoryRows" :key="i" class="row">
              <text class="row-label">{{ r.category_name || '未分类' }}</text>
              <text class="row-val">{{ r.count }}</text>
            </view>
          </view>

          <view v-if="reporterRows.length" class="panel">
            <view class="panel-title">报修人 Top5</view>
            <view v-for="(r, i) in reporterRows" :key="i" class="row">
              <text class="row-label">{{ r.nickname }}</text>
              <text class="row-val">{{ r.count }}</text>
            </view>
          </view>
        </block>
      </block>

      <!-- 业务员数据统计（维修业务员） -->
      <block v-else>
        <view v-if="!overview" class="tip">暂无统计数据</view>
        <block v-else>
          <view class="panel">
            <view class="panel-title">今日</view>
            <view class="grid">
              <view class="metric">
                <text class="metric-label">今日接单</text>
                <text class="metric-val metric-val-ok">{{ overview.today_accepted }}</text>
              </view>
              <view class="metric">
                <text class="metric-label">今日完工</text>
                <text class="metric-val">{{ overview.today_completed }}</text>
              </view>
            </view>
          </view>

          <view class="panel">
            <view class="panel-title">累计</view>
            <view class="grid">
              <view class="metric">
                <text class="metric-label">可接单</text>
                <text class="metric-val metric-val-warn">{{ overview.pending_accept }}</text>
              </view>
              <view class="metric">
                <text class="metric-label">处理中</text>
                <text class="metric-val metric-val-info">{{ overview.my_processing }}</text>
              </view>
              <view class="metric">
                <text class="metric-label">累计接单</text>
                <text class="metric-val">{{ overview.my_accepted }}</text>
              </view>
              <view class="metric">
                <text class="metric-label">累计完工</text>
                <text class="metric-val metric-val-ok">{{ overview.my_completed }}</text>
              </view>
            </view>
          </view>
        </block>
      </block>
    </template>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { statsAPI } from '@/api'
import { isStoreStaff, isUnitReviewer, reviewerEnterprises } from '@/utils/auth'
import { setTabBarSelected } from '@/utils/tabbar'
import type { OrderMetrics, OrderStats, RepairerOverview } from '@/types'

type StatsType = 'enterprise' | 'repairer'
type Preset = 'today' | 'yesterday' | 'month'

function fmtDate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 预设 → start/end（后端对纯日期 end 按"含当天"处理） */
function timeWindow(preset: Preset): { start: string; end: string } {
  const now = new Date()
  const today = fmtDate(now)
  if (preset === 'yesterday') {
    const y = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
    return { start: `${fmtDate(y)} 00:00:00`, end: `${today} 00:00:00` }
  }
  if (preset === 'month') {
    return { start: `${fmtDate(new Date(now.getFullYear(), now.getMonth(), 1))} 00:00:00`, end: '' }
  }
  return { start: `${today} 00:00:00`, end: '' }
}

export default defineComponent({
  name: 'StatsIndex',
  data() {
    return {
      type: 'enterprise' as StatsType,
      enterpriseId: '',
      preset: 'today' as Preset,
      metrics: null as OrderMetrics | null,
      stats: null as OrderStats | null,
      overview: null as RepairerOverview | null,
      loading: false
    }
  },
  computed: {
    presets(): { key: Preset; label: string }[] {
      return [
        { key: 'today', label: '今日' },
        { key: 'yesterday', label: '昨日' },
        { key: 'month', label: '本月' }
      ]
    },
    isEnterprise(): boolean {
      return this.type === 'enterprise'
    },
    statusRows(): { status: string; label: string; count: number }[] {
      return (this.stats?.by_status || []).filter((r) => r.count > 0)
    },
    categoryRows(): { category_name: string; count: number }[] {
      return (this.stats?.by_category || []).filter((r) => r.count > 0).slice(0, 5)
    },
    reporterRows(): { nickname: string; count: number }[] {
      return (this.stats?.top_reporters || []).filter((r) => r.count > 0).slice(0, 5)
    }
  },
  methods: {
    async load() {
      this.loading = true
      try {
        if (this.isEnterprise) {
          const w = timeWindow(this.preset)
          const params: { enterprise_id?: string; start?: string; end?: string } = {}
          // ⚠️ GET 传参先建空对象、仅非空赋值：undefined 会被 uni.request 序列化成 "undefined"
          if (this.enterpriseId) params.enterprise_id = this.enterpriseId
          if (w.start) params.start = w.start
          if (w.end) params.end = w.end
          const [m, s] = await Promise.all([
            statsAPI.metrics(params),
            statsAPI.summary(params)
          ])
          this.metrics = m
          this.stats = s
        } else {
          const params: { enterprise_id?: string } = {}
          if (this.enterpriseId) params.enterprise_id = this.enterpriseId
          this.overview = await statsAPI.repairerOverview(params)
        }
      } catch (e) {
        // 单位审核员未指定本单位时后端会 403，这里给出可读提示
        const msg = (e as { message?: string })?.message || '加载统计失败'
        uni.showToast({ title: msg, icon: 'none' })
        this.metrics = null
        this.stats = null
        this.overview = null
      } finally {
        this.loading = false
      }
    },
    onPreset(p: Preset) {
      if (this.preset === p) return
      this.preset = p
      this.load()
    }
  },
  /**
   * V1.7：统计页升为 tabBar 页，而 switchTab 不支持 URL 参数，
   * 因此统计口径一律按角色推导——维修业务员看「业务员数据统计」，其余（单位审核员）看「企业数据统计」。
   * 若确实带了 type / enterprise_id 参数（如日后从其它页面 navigateTo 进入），仍优先采用。
   */
  onLoad(options: Record<string, string>) {
    const q = options || {}
    this.type = q.type === 'repairer' ? 'repairer' : isStoreStaff() && !isUnitReviewer() ? 'repairer' : 'enterprise'
    this.enterpriseId = q.enterprise_id || ''
    // 单位审核员必须限定本单位（后端硬性要求），缺省时取其所任审核员的第一个单位
    if (this.isEnterprise && !this.enterpriseId && isUnitReviewer()) {
      this.enterpriseId = reviewerEnterprises()[0]?.enterprise_id || ''
    }
    uni.setNavigationBarTitle({ title: this.isEnterprise ? '企业数据统计' : '业务员数据统计' })
  },
  onShow() {
    setTabBarSelected(1)
    this.load()
  }
})
</script>

<style lang="scss" scoped>
.stats {
  /* 底部留出 tabBar 高度 + 安全区 + 20px 呼吸位，避免最后一块面板被固定导航栏遮挡 */
  padding: 24rpx 24rpx calc(var(--tabbar-height, 100rpx) + var(--safe-bottom, 0px) + 20px);
}

.seg {
  display: flex;
  margin-bottom: 24rpx;
  background: var(--xn-bg-soft);
  border-radius: var(--xn-radius-pill);
  padding: 6rpx;
}
.seg-item {
  flex: 1;
  text-align: center;
  padding: 14rpx 0;
  border-radius: var(--xn-radius-pill);
}
.seg-item-on {
  background: var(--xn-surface);
}
.seg-txt {
  font-size: 26rpx;
  color: var(--xn-text-600);
}
.seg-item-on .seg-txt {
  color: var(--xn-primary);
  font-weight: 600;
}

.tip {
  padding: 60rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: var(--xn-text-400);
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
  margin-bottom: 24rpx;
}
.metric {
  padding: 24rpx 20rpx;
  background: var(--xn-surface);
  border: 1rpx solid var(--xn-border);
  border-radius: var(--xn-radius-card);
}
.metric-label {
  display: block;
  font-size: 24rpx;
  color: var(--xn-text-500);
}
.metric-val {
  display: block;
  margin-top: 10rpx;
  font-size: 44rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.metric-val-warn {
  color: #ea580c;
}
.metric-val-ok {
  color: #0f766e;
}
.metric-val-bad {
  color: #dc2626;
}
.metric-val-info {
  color: #2563eb;
}

.panel {
  margin-bottom: 24rpx;
  background: var(--xn-surface);
  border: 1rpx solid var(--xn-border);
  border-radius: var(--xn-radius-card);
  overflow: hidden;
}
.panel-title {
  display: block;
  padding: 20rpx 24rpx 12rpx;
  font-size: 28rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 24rpx;
  border-top: 1rpx solid var(--xn-border-soft);
}
.row-label {
  flex: 1;
  min-width: 0;
  font-size: 26rpx;
  color: var(--xn-text-700);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row-val {
  margin-left: 16rpx;
  font-size: 28rpx;
  font-weight: 600;
  color: var(--xn-text-900);
}
</style>
