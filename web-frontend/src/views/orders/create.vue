<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type UploadFile, type UploadUserFile } from 'element-plus'
import { adminAPI, fetchOrderOptions } from '@/api/admin'
import { useUserStore } from '@/stores/user'
import type { CategoryOption, ProblemOption, PropertyOption } from '@/types'

/**
 * 上报工单（Web 后台版，与小程序共用同一套后端接口）。
 *
 * 使用场景：单位审核员 / 维修业务员 / 超级管理员代客户单位上报故障。
 * 注意：后端提交时会校验「当前用户必须是该单位已审批成员」(verifyMembership)，
 * 因此企业下拉只能用 /orders/options 返回的「我加入并通过审核的单位」，
 * 不能用超管的企业全列表，否则提交会被 403。
 *
 * 图片时序：选图即传（即时上传，落 temporary），提交/保存时回传 image_ids，
 * 由后端 updateOrder 做 diff 关联（新→active、移除→deleted）。新建单无 order_id 时
 * 走 free 接口落「未绑定哨兵」temporary，建单后由 image_ids 关联激活。
 */

const router = useRouter()
const userStore = useUserStore()

const MAX_IMAGES = 9

const pageLoading = ref(false)
const submitting = ref(false)
const saving = ref(false)

// ── 字典（一次拉全：大类 → 属性 → 常见问题、可见单位、紧急程度） ──
const categories = ref<CategoryOption[]>([])
const enterprises = ref<Array<{ id: string; name: string }>>([])
const urgencyOptions = ref<Array<{ value: string; label: string }>>([])

// ── 表单 ──
const form = ref({
  enterprise_id: '',
  category_id: '',
  property_id: '',
  description: '',
  urgency: 'normal',
  room: '',
  contact_name: '',
  contact_phone: ''
})

/** 服务端草稿 id；为空表示尚未落库，首次保存/提交时才创建 */
const orderId = ref('')

const propertyOptions = computed<PropertyOption[]>(() => {
  const c = categories.value.find(x => x.id === form.value.category_id)
  return c?.properties ?? []
})

const problemOptions = computed<ProblemOption[]>(() => {
  const p = propertyOptions.value.find(x => x.id === form.value.property_id)
  return p?.problems ?? []
})

// ── 故障图片：选图即传（即时上传，落 temporary），提交时回传 image_ids 由后端 diff 激活 ──
const imageFiles = ref<UploadUserFile[]>([])
/** uid → 已上传图片的 id 与 url（即时上传返回） */
const uploadedMap = new Map<number, { id: string; url: string }>()

const removeImageFile = (uid: number) => {
  const idx = imageFiles.value.findIndex(f => f.uid === uid)
  if (idx >= 0) imageFiles.value.splice(idx, 1)
  uploadedMap.delete(uid)
}

const handleImageExceed = () => {
  ElMessage.warning(`最多上传 ${MAX_IMAGES} 张故障图片`)
}

// 选图即传：校验通过后立即上传（无 order_id 走 free 接口落哨兵 temporary，有则绑定工单）。
// 上传成功写入 uploadedMap[uid] = {id,url}；失败置 fail 便于重选。
const handleImageChange = async (file: UploadFile) => {
  if (file.status !== 'ready') return
  const raw = file.raw
  if (!raw) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(raw.type)) {
    ElMessage.error('仅支持 jpg/png/webp 格式的图片')
    removeImageFile(file.uid)
    return
  }
  if (raw.size > 5 * 1024 * 1024) {
    ElMessage.error('单张图片不能超过 5MB')
    removeImageFile(file.uid)
    return
  }
  try {
    const endpoint = orderId.value
      ? adminAPI.uploadOrderImage(orderId.value, raw)
      : adminAPI.uploadOrderImageFree(raw)
    const res = await endpoint
    if (file.uid !== undefined) uploadedMap.set(file.uid, { id: res.data.id, url: res.data.url })
    file.status = 'success'
  } catch (e) {
    file.status = 'fail'
    ElMessage.error('图片上传失败，请重试')
  }
}

const handleImageRemove = (file: UploadUserFile) => {
  if (file.uid !== undefined) uploadedMap.delete(file.uid)
}

// ── 字典与默认值 ──
const fetchOptions = async () => {
  pageLoading.value = true
  try {
    const res = await fetchOrderOptions()
    categories.value = res.data.categories ?? []
    enterprises.value = res.data.enterprises ?? []
    urgencyOptions.value = res.data.urgent_levels ?? []
    applyDefaults()
  } finally {
    pageLoading.value = false
  }
}

/** 与小程序一致：只有一个单位时自动选中；联系人/电话用当前账号预填 */
const applyDefaults = () => {
  if (!form.value.enterprise_id && enterprises.value.length === 1) {
    form.value.enterprise_id = enterprises.value[0].id
  }
  const user = userStore.userInfo
  if (!user) return
  if (!form.value.contact_name) form.value.contact_name = user.nickname || ''
  if (!form.value.contact_phone && /^1\d{10}$/.test(user.phone || '')) {
    form.value.contact_phone = user.phone
  }
}

const handleCategoryChange = () => {
  form.value.property_id = ''
}

/** 选中常见问题 → 追加到描述末尾（空格分隔），与小程序行为一致 */
const handleProblemPick = (p: ProblemOption) => {
  const text = (p.name || '').trim()
  if (!text) return
  const cur = form.value.description.trim()
  form.value.description = cur ? `${cur} ${text}` : text
}

// ── 提交相关 ──

/** 后端 contact 是单字段，约定「姓名 空格 电话」 */
const contactValue = () =>
  `${form.value.contact_name.trim()} ${form.value.contact_phone.trim()}`.trim()

const validateForm = (): string => {
  const f = form.value
  if (!f.enterprise_id) return '请选择报修单位'
  if (!f.description.trim()) return '请描述遇到的问题'
  if (f.description.length > 500) return '报修描述不能超过 500 字'
  if (!f.room.trim()) return '请输入位置'
  if (!f.contact_name.trim()) return '请输入联系人'
  if (!/^1\d{10}$/.test(f.contact_phone.trim())) return '请输入正确的 11 位手机号'
  if (!f.category_id) return '请选择设备类型'
  if (!f.property_id) return '请选择问题类别'
  if (!f.urgency) return '请选择紧急程度'
  return ''
}

const buildPayload = () => ({
  enterprise_id: form.value.enterprise_id || undefined,
  category_id: form.value.category_id || undefined,
  property_id: form.value.property_id || undefined,
  description: form.value.description || undefined,
  urgency: form.value.urgency || undefined,
  room: form.value.room || undefined,
  contact: contactValue() || undefined
})

/**
 * 落库：确保草稿存在 → 传 image_ids 由后端 diff 激活（图片已在选图时即时上传为 temporary）。
 * 新建单：首次 create 拿到 order_id 后，updateOrder(image_ids) 把哨兵 temporary 记录绑定并激活。
 */
const persistOrder = async () => {
  if (!orderId.value) {
    const res = await adminAPI.createOrder(buildPayload())
    orderId.value = res.data.order_id
  } else {
    await adminAPI.updateOrder(orderId.value, buildPayload())
  }
  const ids = imageFiles.value
    .map(f => (f.uid !== undefined ? uploadedMap.get(f.uid)?.id : undefined))
    .filter((x): x is string => !!x)
  await adminAPI.updateOrder(orderId.value, { image_ids: ids })
}

const handleSaveDraft = async () => {
  if (saving.value) return
  saving.value = true
  try {
    await persistOrder()
    ElMessage.success('草稿已保存')
  } finally {
    saving.value = false
  }
}

const handleSubmit = async () => {
  const err = validateForm()
  if (err) {
    ElMessage.warning(err)
    return
  }
  if (submitting.value) return
  submitting.value = true
  try {
    await persistOrder()
    await adminAPI.submitOrder(orderId.value)
    ElMessage.success('提交成功')
    router.push(`/orders/${orderId.value}`)
  } finally {
    submitting.value = false
  }
}

const handleReset = () => {
  form.value = {
    enterprise_id: enterprises.value.length === 1 ? enterprises.value[0].id : '',
    category_id: '',
    property_id: '',
    description: '',
    urgency: 'normal',
    room: '',
    contact_name: '',
    contact_phone: ''
  }
  imageFiles.value = []
  orderId.value = ''
  applyDefaults()
  ElMessage.info('已重置表单')
}

onMounted(fetchOptions)
</script>

<template>
  <div v-loading="pageLoading">
    <el-alert
      v-if="!pageLoading && enterprises.length === 0"
      class="empty-tip"
      type="warning"
      show-icon
      :closable="false"
      title="你当前不是任何单位的已审批成员，无法上报工单"
      description="上报工单要求账号已加入某个单位并通过审批。请联系超级管理员先把你的账号加入对应单位。"
    />

    <template v-if="enterprises.length > 0">
      <!-- ① 发生了什么 -->
      <el-card shadow="never" class="card">
        <template #header>
          <span class="card-title">① 发生了什么？</span>
        </template>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form label-position="top">
              <el-form-item label="设备类型" required>
                <el-select
                  v-model="form.category_id"
                  placeholder="请选择项目大类"
                  class="fill"
                  @change="handleCategoryChange"
                >
                  <el-option
                    v-for="c in categories"
                    :key="c.id"
                    :label="c.name"
                    :value="c.id"
                  />
                </el-select>
              </el-form-item>
            </el-form>
          </el-col>
          <el-col :span="12">
            <el-form label-position="top">
              <el-form-item label="问题类别" required>
                <el-select
                  v-model="form.property_id"
                  :placeholder="form.category_id ? '请选择项目属性' : '请先选择项目大类'"
                  class="fill"
                  :disabled="!form.category_id"
                >
                  <el-option
                    v-for="p in propertyOptions"
                    :key="p.id"
                    :label="p.name"
                    :value="p.id"
                  />
                </el-select>
              </el-form-item>
            </el-form>
          </el-col>
        </el-row>

        <el-form label-position="top">
          <el-form-item label="故障图片">
            <el-upload
              v-model:file-list="imageFiles"
              list-type="picture-card"
              accept=".jpg,.jpeg,.png,.webp"
              :auto-upload="false"
              :limit="MAX_IMAGES"
              :on-change="handleImageChange"
              :on-remove="handleImageRemove"
              :on-exceed="handleImageExceed"
            >
              <el-icon><Plus /></el-icon>
            </el-upload>
            <div class="hint">
              最多 {{ MAX_IMAGES }} 张，单张不超过 5MB，支持 jpg/png/webp
            </div>
          </el-form-item>

          <el-form-item label="故障描述" required>
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="4"
              maxlength="500"
              show-word-limit
              placeholder="请描述遇到的问题"
            />
          </el-form-item>

          <el-form-item v-if="problemOptions.length > 0" label="常见问题">
            <div class="chips">
              <el-tag
                v-for="p in problemOptions"
                :key="p.id"
                class="chip"
                type="info"
                effect="plain"
                @click="handleProblemPick(p)"
              >
                {{ p.name }}
              </el-tag>
            </div>
            <div class="hint">点击可把问题描述追加到「故障描述」末尾</div>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- ② 在哪里 -->
      <el-card shadow="never" class="card">
        <template #header>
          <span class="card-title">② 在哪里？</span>
        </template>

        <el-form label-position="top">
          <el-form-item label="报修单位" required>
            <el-select v-model="form.enterprise_id" placeholder="请选择报修单位" class="unit-select">
              <el-option
                v-for="e in enterprises"
                :key="e.id"
                :label="e.name"
                :value="e.id"
              />
            </el-select>
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="位置" required>
                <el-input v-model="form.room" maxlength="20" show-word-limit placeholder="如：A 栋 301" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="联系人" required>
                <el-input v-model="form.contact_name" maxlength="20" placeholder="如：张三" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="联系电话" required>
                <el-input v-model="form.contact_phone" maxlength="11" placeholder="11 位手机号" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </el-card>

      <!-- ③ 帮我们分类 -->
      <el-card shadow="never" class="card">
        <template #header>
          <span class="card-title">③ 帮我们分类</span>
        </template>

        <el-form label-position="top">
          <el-form-item label="紧急程度" required>
            <el-radio-group v-if="urgencyOptions.length > 0" v-model="form.urgency">
              <el-radio-button
                v-for="u in urgencyOptions"
                :key="u.value"
                :value="u.value"
              >
                {{ u.label }}
              </el-radio-button>
            </el-radio-group>
            <span v-else class="hint">字典未返回紧急程度，请刷新重试</span>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="card">
        <div class="footer-bar">
          <el-button @click="handleReset">重置</el-button>
          <el-button :loading="saving" @click="handleSaveDraft">保存草稿</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            提交上报
          </el-button>
        </div>
        <div class="hint footer-hint">
          提交后工单进入「待审核」，由该单位的审核员处理后流转给维修业务员。
        </div>
      </el-card>
    </template>
  </div>
</template>

<style scoped>
.card {
  margin-bottom: 16px;
}

.card-title {
  font-size: 15px;
  font-weight: 500;
}

.fill {
  width: 100%;
}

.unit-select {
  width: 320px;
}

.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chip {
  cursor: pointer;
  user-select: none;
}

.footer-bar {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.footer-hint {
  margin-top: 8px;
  text-align: right;
}

.empty-tip {
  margin-bottom: 16px;
}
</style>
