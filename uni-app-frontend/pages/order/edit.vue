<template>
  <view class="page">
    <!-- 报修信息 -->
    <view class="form-card">
      <!-- 报修单位 -->
      <picker
        mode="selector"
        :range="enterpriseOptions"
        range-key="name"
        :value="enterpriseIndex"
        @change="onEnterpriseChange"
      >
        <view class="form-cell">
          <text class="cell-label">报修单位</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.enterprise_id }">
            {{ enterpriseLabel || '请选择单位' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 项目大类 -->
      <picker
        mode="selector"
        :range="categoryOptions"
        range-key="name"
        :value="categoryIndex"
        @change="onCategoryChange"
      >
        <view class="form-cell">
          <text class="cell-label">项目大类</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.category_id }">
            {{ categoryLabel || '请选择' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 项目属性（联动：仅所选大类下属性） -->
      <picker
        mode="selector"
        :range="propertyOptions"
        range-key="name"
        :value="propertyIndex"
        :disabled="propertyOptions.length === 0"
        @change="onPropertyChange"
      >
        <view class="form-cell">
          <text class="cell-label">项目属性</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.property_id }">
            {{ propertyLabel || (propertyOptions.length === 0 ? '请先选择大类' : '请选择') }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 常见问题快捷预填（联动：仅所选属性下的常见问题） -->
      <view v-if="problemOptions.length > 0" class="issue-chips">
        <view class="chips-label">常见问题（点击预填描述）</view>
        <view
          v-for="p in problemOptions"
          :key="p.id"
          class="issue-chip"
          :class="{ active: form.description === problemDesc(p) }"
          @click="onProblemPick(p)"
        >
          {{ p.name }}
        </view>
      </view>

      <!-- 紧急程度 -->
      <picker mode="selector" :range="urgencyOptions" range-key="label" @change="onUrgencyChange">
        <view class="form-cell">
          <text class="cell-label">紧急程度</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.urgency }">
            {{ urgencyLabel || '请选择' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 报修描述 -->
      <view class="form-cell form-cell-column">
        <text class="cell-label">报修描述</text>
        <textarea
          class="cell-textarea"
          v-model="form.description"
          placeholder="请描述故障现象（1-500字）"
          placeholder-class="input-placeholder"
          maxlength="500"
          auto-height
        />
      </view>

      <!-- 位置 -->
      <view class="form-cell">
        <text class="cell-label">位置</text>
        <input
          class="cell-input"
          v-model="form.room"
          placeholder="如：三楼财务科301"
          placeholder-class="input-placeholder"
          maxlength="20"
        />
      </view>

      <!-- 联系人及电话 -->
      <view class="form-cell">
        <text class="cell-label">联系人</text>
        <input
          class="cell-input"
          v-model="form.contact"
          placeholder="如：张会计 13800138000"
          placeholder-class="input-placeholder"
          maxlength="40"
        />
      </view>
    </view>

    <!-- 故障图片 -->
    <view class="form-card">
      <view class="upload-title">
        <text>故障图片（{{ imageList.length }}/9）</text>
        <text class="upload-tip">支持 jpg/png/webp，单张 ≤5MB</text>
      </view>
      <view class="upload-grid">
        <view v-for="(img, index) in imageList" :key="index" class="upload-item">
          <image
            class="upload-img"
            :src="img.localPath || img.url"
            mode="aspectFill"
            @click="previewImage(index)"
            @error="onImageError(index)"
          ></image>
          <view class="upload-del" @click.stop="removeImage(index)">×</view>
        </view>
        <view v-if="imageList.length < 9" class="upload-add" @click="chooseImage">
          <text class="upload-add-icon">+</text>
          <text class="upload-add-text">添加图片</text>
        </view>
      </view>
    </view>

    <!-- 底部操作栏 -->
    <view class="footer">
      <view class="footer-btn">
        <wd-button plain round block @click="deleteDraft">删除</wd-button>
      </view>
      <view class="footer-btn">
        <wd-button round block @click="saveDraft">保存草稿</wd-button>
      </view>
      <view class="footer-btn">
        <wd-button type="primary" round block :loading="submitting" @click="submitOrder">提交报修</wd-button>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http, uploadOrderImage } from '@/utils/request'
import type { CategoryOption, OptionsResult, OrderDetail, PropertyOption, ProblemOption } from '@/types'

interface ImageItem {
  url: string
  /** 上传前的本地临时路径，用于缩略图即时展示 */
  localPath?: string
}

export default defineComponent({
  data() {
    return {
      orderId: '',
      submitting: false,
      saving: false,
      options: null as OptionsResult | null,
      enterpriseOptions: [] as { id: string; name: string }[],
      categoryOptions: [] as CategoryOption[],
      propertyOptions: [] as PropertyOption[],
      problemOptions: [] as ProblemOption[],
      urgencyOptions: [] as { value: string; label: string }[],
      form: {
        enterprise_id: '',
        category_id: '',
        property_id: '',
        description: '',
        urgency: '',
        room: '',
        contact: ''
      },
      imageList: [] as ImageItem[]
    }
  },
  computed: {
    enterpriseIndex(): number {
      return this.enterpriseOptions.findIndex((o) => o.id === this.form.enterprise_id)
    },
    enterpriseLabel(): string {
      const o = this.enterpriseOptions.find((x) => x.id === this.form.enterprise_id)
      return o ? o.name : ''
    },
    categoryIndex(): number {
      return this.categoryOptions.findIndex((c) => c.id === this.form.category_id)
    },
    categoryLabel(): string {
      const c = this.categoryOptions.find((x) => x.id === this.form.category_id)
      return c ? c.name : ''
    },
    propertyIndex(): number {
      return this.propertyOptions.findIndex((p) => p.id === this.form.property_id)
    },
    propertyLabel(): string {
      const p = this.propertyOptions.find((x) => x.id === this.form.property_id)
      return p ? p.name : ''
    },
    urgencyLabel(): string {
      const u = this.urgencyOptions.find((o) => o.value === this.form.urgency)
      return u ? u.label : ''
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
    if (!this.orderId) {
      uni.showToast({ title: '参数错误', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 800)
      return
    }
    this.init()
  },
  methods: {
    async init() {
      try {
        const [opts, detail] = await Promise.all([
          http.get<OptionsResult>('/orders/options'),
          http.get<OrderDetail>(`/orders/${this.orderId}`)
        ])
        this.options = opts
        this.enterpriseOptions = opts.enterprises || []
        this.categoryOptions = opts.categories || []
        this.urgencyOptions = opts.urgent_levels || []
        this.applyDetail(detail)
      } catch (e) {
        console.error('初始化失败', e)
      }
    },
    /** 回填草稿：按 ID 匹配，加载对应大类下的属性/问题 */
    applyDetail(detail: OrderDetail) {
      this.form.enterprise_id = detail.enterprise_id || ''
      this.form.category_id = detail.category_id || ''
      this.form.property_id = detail.property_id || ''
      this.form.description = detail.description || ''
      this.form.urgency = detail.urgency || ''
      this.form.room = detail.room || ''
      this.form.contact = detail.contact || ''
      this.imageList = (detail.images || []).map((img) => ({ url: img.url }))

      // 恢复属性/问题联动选项
      if (this.form.category_id) {
        const cat = this.categoryOptions.find((c) => c.id === this.form.category_id)
        this.propertyOptions = cat ? cat.properties || [] : []
        if (this.form.property_id) {
          const prop = this.propertyOptions.find((p) => p.id === this.form.property_id)
          this.problemOptions = prop ? prop.problems || [] : []
        }
      }
    },
    onEnterpriseChange(e: { detail: { value: number } }) {
      const opt = this.enterpriseOptions[e.detail.value]
      if (opt) this.form.enterprise_id = opt.id
    },
    /** 切换大类：重置属性与问题 */
    onCategoryChange(e: { detail: { value: number } }) {
      const opt = this.categoryOptions[e.detail.value]
      if (!opt) return
      this.form.category_id = opt.id
      this.form.property_id = ''
      this.problemOptions = []
      this.propertyOptions = opt.properties || []
    },
    /** 切换属性：重置问题列表 */
    onPropertyChange(e: { detail: { value: number } }) {
      const opt = this.propertyOptions[e.detail.value]
      if (!opt) return
      this.form.property_id = opt.id
      this.problemOptions = opt.problems || []
    },
    /** 常见问题：预填描述（可继续编辑），不入库 */
    onProblemPick(p: ProblemOption) {
      const prefill = this.problemDesc(p)
      this.form.description = this.form.description === prefill ? '' : prefill
    },
    problemDesc(p: ProblemOption): string {
      return (p.description || p.name || '').trim()
    },
    onUrgencyChange(e: { detail: { value: number } }) {
      const opt = this.urgencyOptions[e.detail.value]
      if (opt) this.form.urgency = opt.value
    },
    chooseImage() {
      const remain = 9 - this.imageList.length
      if (remain <= 0) {
        uni.showToast({ title: '最多上传9张图片', icon: 'none' })
        return
      }
      uni.showActionSheet({
        itemList: ['从聊天记录选择', '拍照或从相册选择'],
        success: (res) => {
          if (res.tapIndex === 0) {
            this.chooseFromChat(remain)
          } else if (res.tapIndex === 1) {
            this.chooseFromCameraOrAlbum(remain)
          }
        }
      })
    },
    /** 从微信聊天记录中选择图片 */
    chooseFromChat(count: number) {
      wx.chooseMessageFile({
        count,
        type: 'image',
        success: async (res) => {
          const files: { path: string; name: string; size: number }[] = res.tempFiles || []
          const validFiles = files.filter((file) => {
            const ext = (file.name || '').split('.').pop()?.toLowerCase()
            return ['jpg', 'jpeg', 'png', 'webp'].includes(ext) && file.size <= 5 * 1024 * 1024
          })
          if (validFiles.length === 0) {
            uni.showToast({ title: '仅支持 jpg/png/webp 且 ≤5MB 的图片', icon: 'none' })
            return
          }
          if (files.length !== validFiles.length) {
            uni.showToast({ title: '已过滤不支持的图片', icon: 'none' })
          }
          await this.uploadFiles(validFiles.map((f) => ({ path: f.path, size: f.size })))
        },
        fail: (err) => {
          console.error('选择图片失败', err)
        }
      })
    },
    /** 拍照或从相册选择图片 */
    chooseFromCameraOrAlbum(count: number) {
      uni.chooseImage({
        count,
        sizeType: ['compressed'],
        sourceType: ['camera', 'album'],
        success: async (res) => {
          const files = (res.tempFiles || []).map((f) => ({
            path: (f as any).path || '',
            size: (f as any).size || 0
          }))
          if (files.length === 0) return
          const validFiles = files.filter((f) => f.size <= 5 * 1024 * 1024)
          if (validFiles.length === 0) {
            uni.showToast({ title: '单张图片不能超过5MB', icon: 'none' })
            return
          }
          await this.uploadFiles(validFiles)
        },
        fail: (err) => {
          console.error('选择图片失败', err)
        }
      })
    },
    /** 批量上传图片到服务器 */
    async uploadFiles(files: { path: string; size: number }[]) {
      uni.showLoading({ title: '上传中...' })
      let uploaded = 0
      for (const file of files) {
        try {
          const data = await uploadOrderImage(this.orderId, file.path)
          this.imageList.push({ url: data.url, localPath: file.path })
          uploaded++
        } catch (e) {
          console.error('图片上传失败', e)
        }
      }
      uni.hideLoading()
      if (uploaded > 0) {
        uni.showToast({ title: `已上传${uploaded}张`, icon: 'success' })
      }
    },
    previewImage(index: number) {
      uni.previewImage({
        current: this.imageList[index].url,
        urls: this.imageList.map((i) => i.url)
      })
    },
    onImageError(index: number) {
      const img = this.imageList[index]
      if (img && img.localPath) {
        img.localPath = undefined
        return
      }
      uni.showToast({ title: '图片加载失败，请检查图床域名配置', icon: 'none' })
    },
    removeImage(index: number) {
      this.imageList.splice(index, 1)
    },
    /** 构造提交参数：images 恒为完整列表，其余字段按需传 */
    buildPayload(all = false): Record<string, unknown> {
      const f = this.form
      const payload: Record<string, unknown> = {
        images: this.imageList.map((i) => i.url)
      }
      if (all || f.enterprise_id) payload.enterprise_id = f.enterprise_id
      if (all || f.category_id) payload.category_id = f.category_id
      if (all || f.property_id) payload.property_id = f.property_id
      if (all || f.description) payload.description = f.description
      if (all || f.urgency) payload.urgency = f.urgency
      if (all || f.room) payload.room = f.room
      if (all || f.contact) payload.contact = f.contact
      return payload
    },
    async saveDraft() {
      if (this.saving) return
      this.saving = true
      try {
        await http.put(`/orders/${this.orderId}`, this.buildPayload(false))
        uni.showToast({ title: '已保存草稿', icon: 'success' })
      } catch (e) {
        console.error('保存草稿失败', e)
      } finally {
        this.saving = false
      }
    },
    async deleteDraft() {
      const res = await uni.showModal({ title: '提示', content: '确定删除该草稿吗？删除后不可恢复' })
      if (!res.confirm) return
      try {
        await http.delete(`/orders/${this.orderId}`)
        uni.showToast({ title: '已删除', icon: 'success' })
        setTimeout(() => uni.navigateBack(), 500)
      } catch (e) {
        console.error('删除草稿失败', e)
      }
    },
    validateForm(): string {
      const f = this.form
      if (!f.enterprise_id) return '请选择报修单位'
      if (!f.category_id) return '请选择项目大类'
      if (!f.property_id) return '请选择项目属性'
      if (!f.description || f.description.trim().length < 1) return '请输入报修描述'
      if (f.description.length > 500) return '报修描述不能超过500字'
      if (!f.urgency) return '请选择紧急程度'
      if (!f.room || f.room.trim().length < 1) return '请输入位置（房间号）'
      if (!f.contact || f.contact.trim().length < 1) return '请输入联系人及电话'
      if (f.contact.length > 40) return '联系人不能超过40字'
      return ''
    },
    /** 一次性请求三条订阅消息授权（处理中/退回/完结） */
    requestSubscribeAuth(): Promise<void> {
      return new Promise((resolve) => {
        if (typeof wx === 'undefined' || typeof wx.requestSubscribeMessage !== 'function') {
          resolve()
          return
        }
        wx.requestSubscribeMessage({
          tmplIds: [
            'GzsQVCeBG4ObOgoYuYkeZ4VZh711fmH9D3T9taI4TJE', // 工单处理提醒(处理中)
            '3Gw9MOYxZN9sC8ka02RyrZK1y6guc3wE1H2wcWjNy0w', // 工单状态提醒(退回)
            'zj71qQ57GcxS6zzkqc2a4PI9ufJftolzmB-f0ed4f5I' // 报修工单完结通知
          ],
          success: () => resolve(),
          fail: () => resolve()
        })
      })
    },
    /** 提交：先请求订阅消息授权，再全量保存草稿，最后调用 submit 接口 */
    async submitOrder() {
      const err = this.validateForm()
      if (err) {
        uni.showToast({ title: err, icon: 'none' })
        return
      }
      if (this.submitting) return
      this.submitting = true
      try {
        await this.requestSubscribeAuth()
        await http.put(`/orders/${this.orderId}`, this.buildPayload(true))
        await http.post(`/orders/${this.orderId}/submit`, {})
        uni.showToast({ title: '提交成功', icon: 'success' })
        setTimeout(() => {
          uni.redirectTo({ url: `/pages/order/detail?id=${this.orderId}` })
        }, 600)
      } catch (e) {
        console.error('提交失败', e)
      } finally {
        this.submitting = false
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding-bottom: 160rpx;
  box-sizing: border-box;
}

.form-card {
  background-color: #ffffff;
  margin: 20rpx 24rpx;
  border-radius: 20rpx;
  padding: 8rpx 28rpx;
}

.form-cell {
  display: flex;
  align-items: center;
  min-height: 104rpx;
  border-bottom: 1rpx solid #f2f3f5;

  &:last-child {
    border-bottom: none;
  }

  .cell-label {
    width: 160rpx;
    font-size: 28rpx;
    color: #333333;
    flex-shrink: 0;
  }

  .cell-input {
    flex: 1;
    font-size: 28rpx;
    color: #333333;
    padding: 20rpx 0;
  }

  .cell-value {
    flex: 1;
    text-align: right;
    font-size: 28rpx;
    color: #333333;
    padding: 20rpx 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;

    &.is-placeholder {
      color: #bbbbbb;
    }

    .cell-arrow {
      margin-left: 8rpx;
      color: #cccccc;
    }
  }
}

.form-cell-column {
  flex-direction: column;
  align-items: flex-start;
  padding: 24rpx 0;

  .cell-label {
    width: auto;
    margin-bottom: 16rpx;
  }

  .cell-textarea {
    width: 100%;
    min-height: 140rpx;
    font-size: 28rpx;
    color: #333333;
  }
}

.input-placeholder {
  color: #bbbbbb;
}

/* 常见问题快捷 chips */
.issue-chips {
  padding: 20rpx 0 28rpx;

  .chips-label {
    font-size: 24rpx;
    color: #999999;
    margin-bottom: 16rpx;
  }

  .issue-chip {
    display: inline-block;
    padding: 10rpx 24rpx;
    margin-right: 16rpx;
    margin-bottom: 16rpx;
    background-color: #f5f6f8;
    border-radius: 999rpx;
    font-size: 24rpx;
    color: #666666;

    &.active {
      background-color: rgba(77, 128, 240, 0.1);
      color: #4d80f0;
    }
  }
}

.upload-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 28rpx 0 20rpx;
  font-size: 28rpx;
  font-weight: 600;
  color: #333333;

  .upload-tip {
    font-size: 22rpx;
    font-weight: 400;
    color: #aaaaaa;
  }
}

.upload-grid {
  display: flex;
  flex-wrap: wrap;
  padding-bottom: 28rpx;

  .upload-item,
  .upload-add {
    width: 200rpx;
    height: 200rpx;
    margin-right: 16rpx;
    margin-bottom: 16rpx;
    border-radius: 16rpx;
    position: relative;
    overflow: hidden;
  }

  .upload-img {
    width: 100%;
    height: 100%;
  }

  .upload-del {
    position: absolute;
    top: 0;
    right: 0;
    width: 44rpx;
    height: 44rpx;
    background-color: rgba(0, 0, 0, 0.5);
    color: #ffffff;
    font-size: 30rpx;
    display: flex;
    align-items: center;
    justify-content: center;
    border-bottom-left-radius: 16rpx;
  }

  .upload-add {
    background-color: #f5f6f8;
    border: 2rpx dashed #d9d9d9;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;

    .upload-add-icon {
      font-size: 60rpx;
      color: #cccccc;
      line-height: 1;
    }

    .upload-add-text {
      margin-top: 8rpx;
      font-size: 22rpx;
      color: #999999;
    }
  }
}

.footer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  gap: 20rpx;
  padding: 20rpx 24rpx;
  padding-bottom: calc(20rpx + env(safe-area-inset-bottom));
  background-color: #ffffff;
  box-shadow: 0 -4rpx 16rpx rgba(0, 0, 0, 0.04);
  box-sizing: border-box;

  .footer-btn {
    flex: 1 1 0;
    min-width: 0;

    :deep(.wd-button) {
      width: 100%;
      min-width: 0;
    }
  }
}
</style>
