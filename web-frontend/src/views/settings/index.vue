<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { adminAPI } from '@/api/admin'
import type { SettingItem } from '@/types'

const settings = ref<Record<string, SettingItem>>({})
const windowHours = ref<number>(24)
const saving = ref(false)
const loading = ref(false)

const WINDOW_KEY = 'completed_edit_window_hours'

const loadSettings = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getSettings()
    settings.value = res.data.settings || {}
    const item = settings.value[WINDOW_KEY]
    if (item) {
      const n = Number(item.value)
      windowHours.value = Number.isNaN(n) ? 24 : n
    }
  } catch (e) {
    // 错误已由拦截器提示
  } finally {
    loading.value = false
  }
}

const save = async () => {
  if (!Number.isInteger(windowHours.value) || windowHours.value <= 0) {
    ElMessage.warning('时限需为正整数（小时）')
    return
  }
  saving.value = true
  try {
    await adminAPI.updateSetting(WINDOW_KEY, String(windowHours.value))
    ElMessage.success('设置已保存')
    await loadSettings()
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div class="settings-page">
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <span>系统设置</span>
          <span class="sub">仅超级管理员可修改</span>
        </div>
      </template>

      <el-form label-position="top" v-loading="loading">
        <el-form-item label="完工后二次修改时限（小时）">
          <el-input-number v-model="windowHours" :min="1" :precision="0" controls-position="right" />
          <div class="hint">
            工单完工后，在此时限内业务员可自行修改对账信息或重新打开；
            超时后仅超级管理员可操作。当前默认 24 小时。
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button type="primary" :loading="saving" @click="save">保存设置</el-button>
      </template>
    </el-card>
  </div>
</template>

<style scoped>
.settings-page {
  padding: 16px;
}
.settings-card {
  max-width: 640px;
}
.card-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.card-header .sub {
  font-size: 12px;
  color: #909399;
}
.hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
  margin-top: 6px;
}
</style>
