<script setup lang="ts">
/**
 * 独立统计页 · 二期「纵向双区卡」（管理后台前端文档 V1.4 第十三章，路径 /stats，甲方选定方案 A）
 * ── 企业数据卡 ──
 *   企业范围（店方：全部默认/指定；审核员：锁定本单位） × 时间（今日默认/昨日/本月/自定义）
 *   → 指标（5.20 metrics：待审核存量/区间上报/区间审核通过/区间退回）
 *   → 分布（5.17 stats：状态饼图 / 项目大类 Top5 / 报修人排行榜 Top5）
 * ── 业务员统计卡（仅店方/超管 isStore）──
 *   业务员（全部默认/指定） × 时间（同上）
 *   → 指标（5.21 repairer-summary：全局待接单存量/区间接单/区间完工；待接单不受所选业务员影响）
 *   → 默认全部=业务员排行表，点击行聚焦单人
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, PieChart } from 'echarts/charts'
import {
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { ECharts, EChartsOption } from 'echarts'
import { adminAPI } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { OrderStats, OrderStatsMetrics, RepairerSummary, RepairerSummaryRow } from '@/types'

echarts.use([
  PieChart,
  BarChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  CanvasRenderer
])

const userStore = useUserStore()
/** 店方/超管：可见两张卡；单位审核员仅见企业数据卡（限本单位） */
const isStore = computed(() => userStore.isStoreStaff)

type TimePreset = 'today' | 'yesterday' | 'month' | 'custom'

/** “全部”哨兵值：Element Plus 对空串 option 会当未选择，故不用 '' 表示全部 */
const ALL = '__all__'

// ── 企业数据卡状态 ──
const entOptions = ref<Array<{ value: string; label: string }>>([])
const enterpriseId = ref('')
const entPreset = ref<TimePreset>('today')
const entCustom = ref<[string, string] | null>(null)
const entLoading = ref(false)
const metrics = ref<OrderStatsMetrics | null>(null)
const stats = ref<OrderStats | null>(null)

// ── 业务员统计卡状态（仅店方） ──
const repOptions = ref<Array<{ value: string; label: string }>>([])
const repairerId = ref('')
const repPreset = ref<TimePreset>('today')
const repCustom = ref<[string, string] | null>(null)
const repLoading = ref(false)
const repSum = ref<RepairerSummary | null>(null)

// ── 图表 ──
const statusRef = ref<HTMLDivElement | null>(null)
const categoryRef = ref<HTMLDivElement | null>(null)
const reporterRef = ref<HTMLDivElement | null>(null)
let statusChart: ECharts | null = null
let categoryChart: ECharts | null = null
let reporterChart: ECharts | null = null

const STATUS_COLOR: Record<string, string> = {
  draft: '#999999',
  reported: '#ff9f0a',
  pending_accept: '#4d80f0',
  processing: '#4d80f0',
  completed: '#07c160',
  cancelled: '#999999',
  rejected: '#fa5151'
}

const emptyOption = (): EChartsOption => ({
  title: {
    text: '暂无数据',
    left: 'center',
    top: 'middle',
    textStyle: { color: '#909399', fontSize: 13, fontWeight: 'normal' }
  }
})

function fmt(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 预设/自定义 → start/end（datetime 字符串，后端 end 解析为 24h 内/精确） */
function timeWindow(preset: TimePreset, custom: [string, string] | null): { start?: string; end?: string } {
  const now = new Date()
  const today = fmt(now)
  if (preset === 'today') return { start: `${today} 00:00:00` }
  if (preset === 'yesterday') {
    const y = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
    return { start: `${fmt(y)} 00:00:00`, end: `${today} 00:00:00` }
  }
  if (preset === 'month') {
    return { start: `${fmt(new Date(now.getFullYear(), now.getMonth(), 1))} 00:00:00` }
  }
  if (preset === 'custom' && custom) {
    return { start: `${custom[0]} 00:00:00`, end: `${custom[1]} 23:59:59` }
  }
  return {}
}

// ── 企业数据卡：指标 + 分布 ──
const entMetricItems = computed(() => {
  const m = metrics.value
  return [
    { key: '待审核', value: m?.pending_review ?? '--', color: '#ff9f0a', hint: '当前存量' },
    { key: '新增上报', value: m?.submitted ?? '--', color: '#4d80f0', hint: '' },
    { key: '审核通过', value: m?.audited ?? '--', color: '#07c160', hint: '' },
    { key: '退回', value: m?.rejected ?? '--', color: '#fa5151', hint: '' }
  ]
})

const categoryRows = computed(() => {
  const list = stats.value?.by_category || []
  const rows = list.slice(0, 5).map(c => ({ name: c.category_name || '未分类', value: c.count }))
  if (list.length > 5) {
    rows.push({ name: '其他', value: list.slice(5).reduce((a, b) => a + b.count, 0) })
  }
  return rows
})

async function loadEntCard() {
  entLoading.value = true
  try {
    const p = timeWindow(entPreset.value, entCustom.value)
    const scope: { enterprise_id?: string } = {}
    if (enterpriseId.value && enterpriseId.value !== ALL) scope.enterprise_id = enterpriseId.value
    const [s, m] = await Promise.all([
      adminAPI.getOrderStats({ ...scope, ...p }),
      adminAPI.getOrderStatsMetrics({ ...scope, ...p })
    ])
    stats.value = s.data
    metrics.value = m.data
    await nextTick()
    renderCharts()
  } catch {
    stats.value = null
    metrics.value = null
  } finally {
    entLoading.value = false
  }
}

// ── 业务员统计卡 ──
/** 全部业务员时 = 各行求和；指定业务员时 = 对应行 */
const repMetricItems = computed(() => {
  const rows = repSum.value?.list || []
  const isAll = repairerId.value === ALL
  let accepted = 0
  let completed = 0
  let name = '全部业务员'
  if (!isAll) {
    const hit = rows.find(r => r.repairer_id === repairerId.value)
    name = hit?.repairer_name || name
    accepted = hit?.accepted ?? 0
    completed = hit?.completed ?? 0
  } else {
    accepted = rows.reduce((a, r) => a + r.accepted, 0)
    completed = rows.reduce((a, r) => a + r.completed, 0)
  }
  return {
    pending: repSum.value?.global_pending_accept ?? '--',
    accepted,
    completed,
    selectedName: name,
    isAll
  }
})

function completionRate(row: RepairerSummaryRow): string {
  if (!row.accepted) return '—'
  return `${Math.round((row.completed / row.accepted) * 100)}%`
}

async function loadRepCard() {
  if (!isStore.value) return
  repLoading.value = true
  try {
    const p = timeWindow(repPreset.value, repCustom.value)
    const params: { repairer_id?: string; start?: string; end?: string } = { ...p }
    if (repairerId.value && repairerId.value !== ALL) params.repairer_id = repairerId.value
    const res = await adminAPI.getOrderStatsRepairerSummary(params)
    repSum.value = res.data
  } catch {
    repSum.value = null
  } finally {
    repLoading.value = false
  }
}

function focusRepairer(row: RepairerSummaryRow) {
  if (repairerId.value === ALL && row.repairer_id) {
    repairerId.value = row.repairer_id
  }
}

function backToAllRepairers() {
  repairerId.value = ALL
}

// ── 图表渲染 ──
function renderCharts() {
  const s = stats.value
  if (!s) return

  if (statusChart) {
    const rows = (s.by_status || []).filter(r => r.count > 0)
    statusChart.setOption(
      rows.length
        ? {
            tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
            legend: { bottom: 0, textStyle: { fontSize: 12 } },
            series: [
              {
                type: 'pie',
                radius: ['40%', '65%'],
                center: ['50%', '42%'],
                data: rows.map(r => ({
                  name: r.label,
                  value: r.count,
                  itemStyle: { color: STATUS_COLOR[r.status] || '#888780' }
                })),
                label: { formatter: '{b}\n{c}' }
              }
            ]
          }
        : emptyOption(),
      true
    )
  }
  if (categoryChart) {
    const rows = categoryRows.value
    categoryChart.setOption(
      rows.length
        ? {
            tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
            grid: { left: 8, right: 24, top: 12, bottom: 8, containLabel: true },
            xAxis: { type: 'value', minInterval: 1 },
            yAxis: { type: 'category', data: rows.map(r => r.name).reverse(), axisLabel: { fontSize: 12 } },
            series: [
              {
                type: 'bar',
                data: rows.map(r => r.value).reverse(),
                itemStyle: { color: '#4d80f0', borderRadius: [0, 4, 4, 0] },
                barWidth: 16
              }
            ]
          }
        : emptyOption(),
      true
    )
  }
  if (reporterChart) {
    const rows = (s.top_reporters || []).map(r => ({ name: r.nickname || '未命名用户', value: r.count }))
    reporterChart.setOption(
      rows.length
        ? {
            tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
            grid: { left: 8, right: 24, top: 12, bottom: 8, containLabel: true },
            xAxis: { type: 'value', minInterval: 1 },
            yAxis: { type: 'category', data: rows.map(r => r.name).reverse(), axisLabel: { fontSize: 12 } },
            series: [
              {
                type: 'bar',
                data: rows.map(r => r.value).reverse(),
                itemStyle: { color: '#534ab7', borderRadius: [0, 4, 4, 0] },
                barWidth: 16
              }
            ]
          }
        : emptyOption(),
      true
    )
  }
}

function handleResize() {
  statusChart?.resize()
  categoryChart?.resize()
  reporterChart?.resize()
}

// ── 初始化 ──
async function initScope() {
  if (isStore.value) {
    entOptions.value = [{ value: ALL, label: '全部企业' }]
    try {
      const res = await adminAPI.getEnterprises({ page: 1, page_size: 100 })
      res.data.list.forEach(e => entOptions.value.push({ value: e.id, label: e.name }))
    } catch {
      /* 忽略：仅「全部企业」也可用 */
    }
    enterpriseId.value = ALL
    repOptions.value = [{ value: ALL, label: '全部业务员' }]
    try {
      const res = await adminAPI.getRepairers()
      ;(res.data.list || []).forEach((r: { id: string; nickname: string }) =>
        repOptions.value.push({ value: r.id, label: r.nickname })
      )
    } catch {
      /* 忽略 */
    }
    repairerId.value = ALL
  } else {
    const list = userStore.reviewerEnterprises
    if (!list.length) return
    entOptions.value = list.map(e => ({ value: e.enterprise_id, label: e.enterprise_name }))
    enterpriseId.value = list[0].enterprise_id
  }
}

onMounted(async () => {
  await initScope()
  if (statusRef.value) statusChart = echarts.init(statusRef.value)
  if (categoryRef.value) categoryChart = echarts.init(categoryRef.value)
  if (reporterRef.value) reporterChart = echarts.init(reporterRef.value)
  window.addEventListener('resize', handleResize)
  loadEntCard()
  loadRepCard()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  statusChart?.dispose()
  categoryChart?.dispose()
  reporterChart?.dispose()
})

watch([enterpriseId, entPreset, entCustom], () => loadEntCard())
watch([repairerId, repPreset, repCustom], () => loadRepCard())
</script>

<template>
  <div class="stats-page">
    <!-- ══ 企业数据卡 ══ -->
    <el-card shadow="never" class="module-card">
      <template #header>
        <div class="card-head">
          <span class="card-title">企业数据</span>
          <span class="card-sub">按企业统计审核链路指标与工单结构分布</span>
        </div>
      </template>
      <div class="toolbar-row">
        <el-select v-model="enterpriseId" class="scope-select" placeholder="企业范围">
          <el-option v-for="opt in entOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <el-radio-group v-model="entPreset">
          <el-radio-button value="today">今日</el-radio-button>
          <el-radio-button value="yesterday">昨日</el-radio-button>
          <el-radio-button value="month">本月</el-radio-button>
          <el-radio-button value="custom">自定义</el-radio-button>
        </el-radio-group>
        <el-date-picker
          v-if="entPreset === 'custom'"
          v-model="entCustom"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          class="range-picker"
        />
      </div>

      <div v-loading="entLoading" class="card-body">
        <div class="metric-grid">
          <div v-for="item in entMetricItems" :key="item.key" class="metric-box">
            <div class="metric-key">
              {{ item.key }}
              <span v-if="item.hint" class="metric-hint">{{ item.hint }}</span>
            </div>
            <div class="metric-val" :style="{ color: item.color }">{{ item.value }}</div>
          </div>
        </div>

        <el-row :gutter="16" class="chart-row">
          <el-col :span="8">
            <div class="chart-card">
              <div class="chart-title">状态分布</div>
              <div ref="statusRef" class="chart"></div>
            </div>
          </el-col>
          <el-col :span="8">
            <div class="chart-card">
              <div class="chart-title">项目大类分布 Top5</div>
              <div ref="categoryRef" class="chart"></div>
            </div>
          </el-col>
          <el-col :span="8">
            <div class="chart-card">
              <div class="chart-title">报修人排行榜 Top5</div>
              <div ref="reporterRef" class="chart"></div>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>

    <!-- ══ 业务员统计卡（仅店方/超管） ══ -->
    <el-card v-if="isStore" shadow="never" class="module-card">
      <template #header>
        <div class="card-head">
          <span class="card-title">业务员统计</span>
          <span class="card-sub">接单与完工（按接单/完工时间计）；待接单为全局存量</span>
        </div>
      </template>
      <div class="toolbar-row">
        <el-select v-model="repairerId" class="scope-select" placeholder="业务员范围">
          <el-option v-for="opt in repOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <el-radio-group v-model="repPreset">
          <el-radio-button value="today">今日</el-radio-button>
          <el-radio-button value="yesterday">昨日</el-radio-button>
          <el-radio-button value="month">本月</el-radio-button>
          <el-radio-button value="custom">自定义</el-radio-button>
        </el-radio-group>
        <el-date-picker
          v-if="repPreset === 'custom'"
          v-model="repCustom"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          class="range-picker"
        />
      </div>

      <div v-loading="repLoading" class="card-body">
        <div class="metric-grid metric-grid-3">
          <div class="metric-box">
            <div class="metric-key">
              待接单
              <span class="metric-hint">全局·不随所选业务员</span>
            </div>
            <div class="metric-val" style="color: #e6a23c">{{ repMetricItems.pending }}</div>
          </div>
          <div class="metric-box">
            <div class="metric-key">接单数</div>
            <div class="metric-val" style="color: #4d80f0">{{ repMetricItems.accepted }}</div>
          </div>
          <div class="metric-box">
            <div class="metric-key">完工数</div>
            <div class="metric-val" style="color: #07c160">{{ repMetricItems.completed }}</div>
          </div>
        </div>

        <!-- 全部业务员：排行表；点行聚焦单人 -->
        <template v-if="repairerId === ALL">
          <el-table
            :data="repSum?.list || []"
            stripe
            size="small"
            class="rep-table"
            @row-click="focusRepairer"
          >
            <el-table-column prop="repairer_name" label="维修员" min-width="180">
              <template #default="{ row }">{{ row.repairer_name || '未命名' }}</template>
            </el-table-column>
            <el-table-column prop="accepted" label="接单数" width="140" align="right" />
            <el-table-column prop="completed" label="完工数" width="140" align="right" />
            <el-table-column label="完工率" width="160" align="right">
              <template #default="{ row }">{{ completionRate(row) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" align="center">
              <template #default="{ row }">
                <el-button link type="primary" size="small" @click.stop="focusRepairer(row)">查看</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="!repSum?.list?.length" class="empty-tip">该时间范围内暂无业务员接单/完工记录</div>
        </template>

        <!-- 指定业务员：单人聚焦卡 -->
        <template v-else>
          <div class="rep-focus">
            <div>
              <div class="rep-focus-name">{{ repMetricItems.selectedName }}</div>
              <div class="rep-focus-sub">
                当前聚焦该业务员：区间接单 {{ repMetricItems.accepted }} · 区间完工 {{ repMetricItems.completed }}
                · 全局待接单 {{ repMetricItems.pending }}
              </div>
            </div>
            <el-button size="small" @click="backToAllRepairers">返回全部业务员</el-button>
          </div>
        </template>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.stats-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.module-card {
  border-radius: 8px;
}

.card-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.card-sub {
  font-size: 12px;
  color: #909399;
}

.toolbar-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 14px;
}

.scope-select {
  width: 220px;
}

.range-picker {
  width: 260px;
}

.card-body {
  min-height: 120px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-grid-3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-box {
  padding: 14px 16px;
  border-radius: 8px;
  background-color: #f5f7fa;
}

.metric-key {
  font-size: 13px;
  color: #606266;
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.metric-hint {
  font-size: 11px;
  color: #b0b3ba;
}

.metric-val {
  margin-top: 6px;
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.chart-row {
  margin-top: 14px;
}

.chart-card {
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.chart-title {
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}

.chart {
  width: 100%;
  height: 260px;
}

.rep-table {
  margin-top: 4px;
}

.empty-tip {
  padding: 24px 0;
  text-align: center;
  font-size: 13px;
  color: #909399;
}

.rep-focus {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fafcff;
}

.rep-focus-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.rep-focus-sub {
  margin-top: 6px;
  font-size: 13px;
  color: #606266;
}
</style>
