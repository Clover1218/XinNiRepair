<script setup lang="ts">
import { User, Location, OfficeBuilding, Picture } from '@element-plus/icons-vue'
import type { OrderListItem } from '@/types'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  row: OrderListItem
  /** 已按企业筛选时隐藏企业行（信息冗余，卡片更紧凑） */
  hideEnterprise?: boolean
}>()
const emit = defineEmits<{ (e: 'click', row: OrderListItem): void }>()

// 放大后的状态展示（颜色 + 文案），含 V1.5 本质标签（待补签等）
const statusClassMap: Record<string, string> = {
  draft: 'st-gray',
  reported: 'st-amber',
  pending_accept: 'st-orange',
  processing: 'st-blue',
  completed: 'st-teal',
  cancelled: 'st-gray',
  rejected: 'st-red'
}
function statusView(row: OrderListItem) {
  if (row.signature_status === 'pending_sign') {
    return { text: '待补签', cls: 'st-pending-sign' }
  }
  return {
    text: row.display_tag || row.status_label,
    cls: statusClassMap[row.status] || 'st-gray'
  }
}
</script>

<template>
  <div class="order-card-c" @click="emit('click', row)">
    <div class="oc-thumb">
      <img v-if="row.images && row.images.length" :src="row.images[0].url" :alt="row.description" />
      <el-icon v-else class="oc-thumb-ph"><Picture /></el-icon>
    </div>
    <div class="oc-body">
      <div class="oc-top">
        <span v-if="row.unread" class="oc-unread-dot" title="有未读动态" />
        <div class="oc-desc" :title="row.description">{{ row.description || '（无故障描述）' }}</div>
        <div v-if="row.amount && row.amount > 0" class="oc-amount">
          ¥{{ Number(row.amount).toFixed(2) }}
        </div>
      </div>
      <div class="oc-mid">
        <span class="status-pill" :class="statusView(row).cls">{{ statusView(row).text }}</span>
        <span
          class="oc-cat"
          :title="(row.category_name || '') + (row.property_name ? ' · ' + row.property_name : '')"
        >
          <span v-if="row.category_name">{{ row.category_name }}</span>
          <span v-if="row.property_name"> · {{ row.property_name }}</span>
        </span>
      </div>
      <!-- 报修人 | 地点 合并一行，过长整体截断 -->
      <div class="oc-meta">
        <div class="oc-line">
          <el-icon class="oc-ico"><User /></el-icon>
          <!-- 用户端 /orders 行不含 reporter（全是本人单），判空回退为「本人」，避免渲染抛错 -->
          <span class="oc-val" :title="row.reporter?.nickname || '本人'">
            {{ row.reporter?.nickname || '本人' }}
          </span>
          <span class="oc-sep">|</span>
          <el-icon class="oc-ico"><Location /></el-icon>
          <span class="oc-val oc-val--flex" :title="row.room">{{ row.room || '未填地点' }}</span>
        </div>
        <!-- 企业单独一行（已选企业筛选时隐藏） -->
        <div v-if="!hideEnterprise" class="oc-line">
          <el-icon class="oc-ico"><OfficeBuilding /></el-icon>
          <span class="oc-val oc-val--flex" :title="row.enterprise_name">{{ row.enterprise_name }}</span>
        </div>
      </div>
      <div class="oc-time">{{ formatDateTime(row.submitted_at) }}</div>
    </div>
  </div>
</template>

<style scoped>
/* C 布局：紧凑扫读卡片（缩略图 + 状态放大 + 故障描述标题 + 大字金额 + 图标化元信息） */
.order-card-c {
  display: flex;
  align-items: stretch;
  gap: 12px;
  border: 1px solid var(--xn-border);
  border-radius: 10px;
  padding: 12px 14px;
  background: var(--xn-surface);
  cursor: pointer;
  transition: box-shadow 0.2s, border-color 0.2s;
}

.order-card-c:hover {
  box-shadow: 0 4px 16px rgba(17, 24, 39, 0.08);
  border-color: var(--xn-primary-light, #b7e0d9);
}

/* 缩略图 */
.oc-thumb {
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  border-radius: 8px;
  overflow: hidden;
  background: var(--xn-tag-bg);
  display: flex;
  align-items: center;
  justify-content: center;
}

.oc-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.oc-thumb-ph {
  font-size: 24px;
  color: var(--xn-text-400);
}

.oc-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.oc-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.oc-unread-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dc2626;
  flex-shrink: 0;
}

.oc-desc {
  font-size: 14px;
  font-weight: 700;
  color: var(--xn-text-900);
  line-height: 1.4;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

.oc-amount {
  font-size: 18px;
  font-weight: 800;
  color: var(--xn-primary);
  white-space: nowrap;
  flex-shrink: 0;
}

.oc-mid {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  min-width: 0;
}

.oc-cat {
  font-size: 12px;
  color: var(--xn-text-500);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

/* 状态放大胶囊 */
.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  color: #fff;
  white-space: nowrap;
  flex-shrink: 0;
}

.st-red {
  background: var(--st-red);
}

.st-amber {
  background: var(--st-amber);
}

.st-orange {
  background: var(--st-orange);
}

.st-blue {
  background: var(--st-blue);
}

.st-teal {
  background: var(--st-teal);
}

.st-gray {
  background: var(--st-gray);
}

.st-pending-sign {
  background: linear-gradient(90deg, #f59e0b, #ea580c);
  box-shadow: 0 1px 4px rgba(234, 88, 12, 0.35);
  letter-spacing: 1px;
}

/* 图标化元信息：报修人|地点 一行、企业单独一行，过长整体截断 */
.oc-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
  font-size: 13px;
  color: var(--xn-text-500);
}

.oc-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  line-height: 1.2;
}

.oc-val {
  color: var(--xn-text-500);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

/* 地点 / 企业占据剩余宽度，超出即截断 */
.oc-val--flex {
  flex: 1;
}

.oc-sep {
  color: var(--xn-text-400);
  flex-shrink: 0;
}

.oc-ico {
  font-size: 14px;
  color: var(--xn-text-400);
  flex-shrink: 0;
  align-self: center;
}

.oc-time {
  margin-top: auto;
  padding-top: 8px;
  font-size: 12px;
  color: var(--xn-text-400);
}
</style>
