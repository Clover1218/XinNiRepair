<template>
  <view class="page">
    <!-- 报修信息 -->
    <view class="form-card">
      <!-- 报修企业 -->
      <picker
        mode="selector"
        :range="enterpriseOptions"
        range-key="name"
        @change="onEnterpriseChange"
      >
        <view class="form-cell">
          <text class="cell-label">报修企业</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.enterprise_id }">
            {{ form.enterprise_name || '请选择企业' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 项目名称 -->
      <view class="form-cell">
        <text class="cell-label">项目名称</text>
        <input
          class="cell-input"
          v-model="form.project_name"
          placeholder="请输入报修项目名称"
          placeholder-class="input-placeholder"
          maxlength="20"
        />
      </view>

      <!-- 项目大类 -->
      <picker mode="selector" :range="categoryOptions" range-key="name" @change="onCategoryChange">
        <view class="form-cell">
          <text class="cell-label">项目大类</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.category }">
            {{ categoryLabel }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 常用故障快捷选择 -->
      <view v-if="commonIssues.length > 0" class="issue-chips">
        <view
          v-for="issue in commonIssues"
          :key="issue"
          class="issue-chip"
          :class="{ active: form.description === issue }"
          @click="selectIssue(issue)"
        >
          {{ issue }}
        </view>
      </view>

      <!-- 项目属性 -->
      <picker mode="selector" :range="propertyOptions" range-key="label" @change="onPropertyChange">
        <view class="form-cell">
          <text class="cell-label">项目属性</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.property }">
            {{ propertyLabel }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <!-- 紧急程度 -->
      <picker mode="selector" :range="urgencyOptions" range-key="label" @change="onUrgencyChange">
        <view class="form-cell">
          <text class="cell-label">紧急程度</text>
          <view class="cell-value" :class="{ 'is-placeholder': !form.urgency }">
            {{ urgencyLabel }}
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

      <!-- 房间号 -->
      <view class="form-cell">
        <text class="cell-label">房间号</text>
        <input
          class="cell-input"
          v-model="form.room"
          placeholder="如：三楼财务科301"
          placeholder-class="input-placeholder"
          maxlength="20"
        />
      </view>

      <!-- 联系人 -->
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
import type { OptionsResult, OrderDetail } from '@/types'

interface ImageItem {
  url: string
  /** 上传前的本地临时路径，用于缩略图即时展示（本地文件在真机上必定可渲染） */
  localPath?: string
}

export default defineComponent({
  data() {
    return {
      orderId: '',
      loading: false,
      submitting: false,
      saving: false,
      options: null as OptionsResult | null,
      enterpriseOptions: [] as { id: string; name: string }[],
      categoryOptions: [] as { id: string; name: string }[],
      propertyOptions: [] as { value: string; label: string }[],
      urgencyOptions: [] as { value: string; label: string }[],
      commonIssues: [] as string[],
      form: {
        enterprise_id: '',
        enterprise_name: '',
        project_name: '',
        category: '',
        property: '',
        description: '',
        urgency: '',
        room: '',
        contact: ''
      },
      imageList: [] as ImageItem[]
    }
  },
  computed: {
    categoryLabel(): string {
      const c = this.categoryOptions.find((o) => o.id === this.form.category)
      return c ? c.name : ''
    },
    propertyLabel(): string {
      const p = this.propertyOptions.find((o) => o.value === this.form.property)
      return p ? p.label : ''
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
        this.categoryOptions = opts.project_categories || []
        this.propertyOptions = opts.properties || []
        this.urgencyOptions = opts.urgent_levels || []
        this.applyDetail(detail)
      } catch (e) {
        console.error('初始化失败', e)
      }
    },
    applyDetail(detail: OrderDetail) {
      this.form.enterprise_id = detail.enterprise_id || ''
      this.form.enterprise_name = detail.enterprise_name || ''
      this.form.project_name = detail.project_name || ''
      this.form.category = detail.category || ''
      this.form.property = detail.property || ''
      this.form.description = detail.description || ''
      this.form.urgency = detail.urgency || ''
      this.form.room = detail.room || ''
      this.form.contact = detail.contact || ''
      this.imageList = (detail.images || []).map((img) => ({ url: img.url }))
      this.syncCommonIssues()
    },
    syncCommonIssues() {
      if (!this.options) return
      this.commonIssues = (this.options.common_issues || {})[this.form.category] || []
    },
    onEnterpriseChange(e: { detail: { value: number } }) {
      const opt = this.enterpriseOptions[e.detail.value]
      if (opt) {
        this.form.enterprise_id = opt.id
        this.form.enterprise_name = opt.name
      }
    },
    onCategoryChange(e: { detail: { value: number } }) {
      const opt = this.categoryOptions[e.detail.value]
      if (opt) {
        this.form.category = opt.id
        this.syncCommonIssues()
      }
    },
    onPropertyChange(e: { detail: { value: number } }) {
      const opt = this.propertyOptions[e.detail.value]
      if (opt) this.form.property = opt.value
    },
    onUrgencyChange(e: { detail: { value: number } }) {
      const opt = this.urgencyOptions[e.detail.value]
      if (opt) this.form.urgency = opt.value
    },
    selectIssue(issue: string) {
      this.form.description = this.form.description === issue ? '' : issue
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
    /** 缩略图加载失败（通常为真机未配置图床 downloadFile 合法域名） */
    onImageError(index: number) {
      const img = this.imageList[index]
      // 本地路径失败时回退到远端 URL
      if (img && img.localPath) {
        img.localPath = undefined
        return
      }
      uni.showToast({ title: '图片加载失败，请检查图床域名配置', icon: 'none' })
    },
    removeImage(index: number) {
      this.imageList.splice(index, 1)
    },
    /** 构造提交参数：images 恒为完整列表，其余字段仅传非空 */
    buildPayload(all = false): Record<string, unknown> {
      const f = this.form
      const payload: Record<string, unknown> = {
        images: this.imageList.map((i) => i.url)
      }
      if (all || f.enterprise_id) payload.enterprise_id = f.enterprise_id
      if (all || f.project_name) payload.project_name = f.project_name
      if (all || f.category) payload.category = f.category
      if (all || f.property) payload.property = f.property
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
      if (!f.enterprise_id) return '请选择报修企业'
      if (!f.project_name || f.project_name.length < 1) return '请输入项目名称'
      if (f.project_name.length > 20) return '项目名称不能超过20字'
      if (!f.category) return '请选择项目大类'
      if (!f.property) return '请选择项目属性'
      if (!f.description || f.description.length < 1) return '请输入报修描述'
      if (f.description.length > 500) return '报修描述不能超过500字'
      if (!f.urgency) return '请选择紧急程度'
      if (!f.room) return '请输入房间号'
      if (!f.contact) return '请输入联系人及电话'
      if (f.contact.length > 40) return '联系人不能超过40字'
      if (this.imageList.length > 9) return '图片最多9张'
      return ''
    },
    /** 一次性请求三条订阅消息授权（工单状态变更/退回/完结）
     *  必须在用户点击动作的同步调用栈内触发 wx.requestSubscribeMessage，
     *  因此放在 submitOrder 最前面调用；授权拒绝不阻塞提交。
     */
    requestSubscribeAuth(): Promise<void> {
      return new Promise((resolve) => {
        // wx 在小程序环境可用，shims-vue.d.ts 已声明为 any；H5/非微信环境跳过
        if (typeof wx === 'undefined' || typeof wx.requestSubscribeMessage !== 'function') {
          resolve()
          return
        }
        wx.requestSubscribeMessage({
          tmplIds: [
            'GzsQVCeBG4ObOgoYuYkeZ5n2jWrcpeGmtzbl2sk4oH8', // 工单状态变更(reported/reviewed/processing)
            '3Gw9MOYxZN9sC8ka02RyrZK1y6guc3wE1H2wcWjNy0w', // 工单退回
            'zj71qQ57GcxS6zzkqc2a4PI9ufJftolzmB-f0ed4f5I'  // 工单完结
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
        // 先完成订阅消息授权，确保后端提交后能即时推送"已上报"通知
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

.issue-chips {
  display: flex;
  flex-wrap: wrap;
  padding: 20rpx 0 28rpx;

  .issue-chip {
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
  }
}
</style>
