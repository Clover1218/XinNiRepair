<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { authAPI } from '@/api/auth'
import { useUserStore } from '@/stores/user'

const nickname = ref('')
const password = ref('')
const loading = ref(false)
const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const handleLogin = async () => {
  if (!nickname.value.trim() || !password.value) {
    ElMessage.warning('请输入昵称和密码')
    return
  }
  loading.value = true
  try {
    const res = await authAPI.adminLogin({ nickname: nickname.value.trim(), password: password.value })
    userStore.setUser(res.data.user, res.data.access_token)

    // 判断是否为平台管理员（JWT role：1=平台管理员，0=普通用户）
    if (!userStore.isPlatformAdmin) {
      ElMessage.error('该账号无管理权限')
      userStore.logout()
      return
    }

    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || ''
    router.push(redirect || '/enterprises')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <el-icon :size="36" color="#409eff"><Tools /></el-icon>
        <h1 class="login-title">新泥维修后台</h1>
        <p class="login-subtitle">电脑维修报修系统 · 管理后台</p>
      </div>

      <el-form @submit.prevent>
        <el-form-item>
          <el-input
            v-model="nickname"
            placeholder="请输入昵称"
            size="large"
            clearable
            @keyup.enter="handleLogin"
          >
            <template #prefix>
              <el-icon><User /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          >
            <template #prefix>
              <el-icon><Key /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-button
          type="primary"
          size="large"
          class="login-btn"
          :loading="loading"
          @click="handleLogin"
        >
          登 录
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #001529 0%, #0a3d62 100%);
}

.login-card {
  width: 400px;
  padding: 40px 36px 32px;
  background-color: #fff;
  border-radius: 10px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
}

.login-header {
  text-align: center;
  margin-bottom: 28px;
}

.login-title {
  margin: 12px 0 6px;
  font-size: 22px;
  color: #303133;
}

.login-subtitle {
  margin: 0;
  font-size: 13px;
  color: #909399;
}

.login-btn {
  width: 100%;
}
</style>
