<script setup lang="ts">
import { User, Location, OfficeBuilding, Picture } from '@element-plus/icons-vue'
import type { OrderListItem } from '@/types'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ row: OrderListItem }>()
const emit = defineEmits<{ (e: 'click', row: OrderListItem): void }>()

// 放大后的状态展示（颜色 + 文案），含 V1.5 本质标签（待补签等）
const statusClassMap: Record<string, string> = {
  draft: 'st-gray',
  reported: 'st-red',
  pending_accept: 'st-orange',
  processing: 'st-blue',
  completed: 'st-green',
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
        <div class="oc-desc" :title="row.description">{{ row.description || '（无故障描述）' }}</div>
        <div v-if="row.amount && row.amount > 0" class="oc-amount">
          ¥{{ Number(row.amount).toFixed(2) }}
        </div>
      </div>
      <div class="oc-mid">
        <span class="status-pill" :class="statusView(row).cls">{{ statusView(row).text }}</span>
        <span class="oc-cat">
          <span v-if="row.category_name">{{ row.category_name }}</span>
          <span v-if="row.property_name"> · {{ row.property_name }}</span>
        </span>
      </div>
      <div class="oc-meta">
        <span class="oc-customer" :title="row.reporter.nickname">
          <el-icon class="oc-ico"><User /></el-icon>{{ row.reporter.nickname }}
        </span>
        <span class="oc-loc" :title="row.room">
          <el-icon class="oc-ico"><Location /></el-icon>{{ row.room || '未填地点' }}
        </span>
        <span class="oc-ent" :title="row.enterprise_name">
          <el-icon class="oc-ico"><OfficeBuilding /></el-icon>{{ row.enterprise_name }}
        </span>
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
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 12px 14px;
  background: #fff;
  cursor: pointer;
  transition: box-shadow 0.2s, border-color 0.2s;
}

.order-card-c:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  border-color: #c6e2ff;
}

/* 缩略图 */
.oc-thumb {
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  border-radius: 8px;
  overflow: hidden;
  background: #f5f7fa;
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
  color: #c0c4cc;
}

.oc-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.oc-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.oc-desc {
  font-size: 15px;
  font-weight: 700;
  color: #303133;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  flex: 1;
}

.oc-amount {
  font-size: 18px;
  font-weight: 800;
  color: #f56c6c;
  white-space: nowrap;
  flex-shrink: 0;
}

.oc-mid {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.oc-cat {
  font-size: 12px;
  color: #606266;
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
  background: #f56c6c;
}

.st-orange {
  background: #e6a23c;
}

.st-blue {
  background: #409eff;
}

.st-green {
  background: #67c23a;
}

.st-gray {
  background: #909399;
}

.st-pending-sign {
  background: linear-gradient(90deg, #ff8c1a, #ff6a00);
  box-shadow: 0 1px 4px rgba(255, 106, 0, 0.45);
  letter-spacing: 1px;
}

/* 图标化元信息：报修人 / 地点 / 企业，图标与文字严格垂直居中 */
.oc-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin-top: 10px;
  font-size: 13px;
  color: #606266;
}

.oc-customer,
.oc-loc,
.oc-ent {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  line-height: 1;
  color: #606266;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.oc-ico {
  font-size: 14px;
  color: #909399;
  flex-shrink: 0;
  align-self: center;
  vertical-align: middle;
}

.oc-time {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}
</style>
