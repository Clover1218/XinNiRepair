<template>
  <view class="page">
    <!-- ① 发生了什么？ -->
    <view class="section">
      <view class="section-title">① 发生了什么？</view>

      <!-- 设备类型（大类） + 问题类别（属性）：同一行对半分 -->
      <view class="cat-row">
        <picker
          class="cat-picker"
          mode="selector"
          :range="categoryOptions"
          range-key="name"
          :value="categoryIndex"
          @change="onCategoryChange"
        >
          <view class="form-cell">
            <view class="cell-label">设备类型</view>
            <view class="cell-value" :class="{ 'is-placeholder': !form.category_id }">
              {{ categoryLabel || '请选择' }}
              <text class="cell-arrow">›</text>
            </view>
          </view>
        </picker>

        <picker
          class="cat-picker"
          mode="selector"
          :range="propertyOptions"
          range-key="name"
          :value="propertyIndex"
          :disabled="propertyOptions.length === 0"
          @change="onPropertyChange"
        >
          <view class="form-cell">
            <view class="cell-label">问题类别</view>
            <view class="cell-value" :class="{ 'is-placeholder': !form.property_id }">
              {{ propertyLabel || (propertyOptions.length === 0 ? '先选类型' : '请选择') }}
              <text class="cell-arrow">›</text>
            </view>
          </view>
        </picker>
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
          <text class="upload-add-text">添加照片</text>
        </view>
      </view>

      <textarea
        class="desc-textarea"
        v-model="form.description"
        placeholder="描述一下遇到的问题"
        placeholder-class="input-placeholder"
        maxlength="500"
        :show-confirm-bar="false"
      />

      <!-- 常见问题：点击以“追加”方式写入问题名称，不整体替换 -->
      <view v-if="problemOptions.length > 0" class="issue-chips">
        <view
          v-for="p in problemOptions"
          :key="p.id"
          class="issue-chip"
          @click="onProblemPick(p)"
        >
          {{ p.name }}
        </view>
      </view>
    </view>

    <!-- ② 在哪里？ -->
    <view class="section">
      <view class="section-title">② 在哪里？</view>

      <picker
        mode="selector"
        :range="enterpriseOptions"
        range-key="name"
        :value="enterpriseIndex"
        @change="onEnterpriseChange"
      >
        <view class="form-cell">
          <view class="cell-label">报修单位</view>
          <view class="cell-value" :class="{ 'is-placeholder': !form.enterprise_id }">
            {{ enterpriseLabel || '请选择单位' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>

      <view class="form-cell">
        <view class="cell-label">位置</view>
        <input
          class="cell-input"
          v-model="form.room"
          placeholder="如：二楼会议室"
          placeholder-class="input-placeholder"
          maxlength="20"
        />
      </view>

      <view class="form-cell">
        <view class="cell-label">联系人</view>
        <input
          class="cell-input"
          v-model="form.contact_name"
          placeholder="如：张三"
          placeholder-class="input-placeholder"
          maxlength="20"
        />
      </view>

      <view class="form-cell">
        <view class="cell-label">联系电话</view>
        <input
          class="cell-input"
          v-model="form.contact_phone"
          type="number"
          placeholder="如：13800138000"
          placeholder-class="input-placeholder"
          maxlength="11"
        />
      </view>
    </view>

    <!-- ③ 帮我们分类 -->
    <view class="section">
      <view class="section-title">③ 帮我们分类</view>

      <picker mode="selector" :range="urgencyOptions" range-key="label" @change="onUrgencyChange">
        <view class="form-cell">
          <view class="cell-label">紧急程度</view>
          <view class="cell-value" :class="{ 'is-placeholder': !form.urgency }">
            {{ urgencyLabel || '请选择' }}
            <text class="cell-arrow">›</text>
          </view>
        </view>
      </picker>
    </view>

    <!-- 底部操作栏 -->
    <view class="footer">
      <view class="footer-btn">
        <wd-button plain round block @click="deleteDraft()">删除</wd-button>
      </view>
      <view class="footer-btn">
        <wd-button round block @click="saveDraft()">保存草稿</wd-button>
      </view>
      <view class="footer-btn">
        <wd-button type="primary" round block :loading="submitting" @click="submitOrder">
          提交报修
        </wd-button>
      </view>
    </view>
  </view>
  <!-- #ifdef MP-WEIXIN -->
  <!-- 空 page-container：仅用于拦截左上角“<”返回手势，不包裹内容（避免 show=false 时主页面变空白） -->
  <page-container :show="containerShow" :overlay="false" @beforeleave="onContainerLeave"></page-container>
  <!-- #endif -->

  <!-- 返回拦截：三按钮（保存 / 不保存 / 取消留页）。禁用遮罩关闭，仅按钮可关，避免误触丢失草稿 -->
  <wd-popup v-model="showExitDialog" position="center" round :close-on-click-modal="false" custom-style="width: 86%;">
    <view class="exit-popup">
      <view class="exit-title">是否保存草稿</view>
      <view class="exit-tip">当前草稿尚未保存，退出前可选择保存或丢弃。</view>
      <view class="exit-actions">
        <wd-button plain size="small" @click="onExitCancel">取消</wd-button>
        <wd-button type="error" plain size="small" @click="onExitDiscard">不保存</wd-button>
        <wd-button type="primary" size="small" :loading="saving" @click="onExitSave">保存</wd-button>
      </view>
    </view>
  </wd-popup>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http, uploadOrderImage } from '@/utils/request'
import { splitContact } from '@/utils/format'
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
      /** 防止返回键弹窗与 navigateBack 互相触发的死循环 */
      exiting: false,
      /** 草稿是否已加载完成（加载完成前不拦截返回） */
      loaded: false,
      /** 微信小程序 page-container 显示态：用于拦截左上角返回键 */
      containerShow: true,
      /** 加载完成后的表单快照，用于脏检查 */
      snapshot: null as null | { form: Record<string, unknown>; images: string[] },
      /** 草稿是否已“保存”过：重新打开已有内容的草稿=true；本会话点过“保存草稿”后=true；新建空草稿未保存=false */
      hasSaved: false,
      /** 返回拦截三按钮弹窗（保存 / 不保存 / 取消留页）显隐 */
      showExitDialog: false,
      /** 初始化未完成时用户已按返回（已拦截待命），加载完成后补弹询问 */
      backPending: false,
      options: null as OptionsResult | null,
      enterpriseOptions: [] as { id: string; name: string }[],
      categoryOptions: [] as CategoryOption[],
      propertyOptions: [] as PropertyOption[],
      urgencyOptions: [] as { value: string; label: string }[],
      form: {
        enterprise_id: '',
        category_id: '',
        property_id: '',
        description: '',
        urgency: '',
        room: '',
        /** V1.3：联系人与联系电话分栏（后端 contact 单字段过渡期内合并提交） */
        contact_name: '',
        contact_phone: ''
      },
      imageList: [] as ImageItem[],
      /** 通用常见问题（未选大类/属性时展示；接口待后端提供，本期为空） */
      generalProblems: [] as ProblemOption[]
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
    },
    /**
     * 常见问题数据源：
     * - 已选大类 + 属性 → 该属性下 problems
     * - 未选 → 通用常见问题（后端接口待提供，本期为空）
     */
    problemOptions(): ProblemOption[] {
      if (this.form.property_id) {
        const prop = this.propertyOptions.find((p) => p.id === this.form.property_id)
        return (prop && prop.problems) || []
      }
      return this.generalProblems
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
    if (!this.orderId) {
      uni.showToast({ title: '参数错误', icon: 'none' })
      this.exiting = true
      setTimeout(() => uni.navigateBack(), 800)
      return
    }
    this.init()
  },
  /**
   * 返回键拦截（H5 / App 端生效）。
   * 注意：微信小程序端 onBackPress 只能监听、无法阻止返回，左上角“<”由 page-container 的
   * @beforeleave 负责拦截，故此处对 MP-WEIXIN 直接放行，避免弹窗出现在已离开的页面上。
   */
  onBackPress() {
    // #ifdef MP-WEIXIN
    return false
    // #endif
    // 退出流程中放行
    if (this.exiting) return false
    this.decideExit()
    return true
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
        // 通用常见问题：后端接口待提供（文档第十一章 #5），若 options 返回则直接使用
        this.generalProblems = ((opts as unknown as Record<string, unknown>).general_problems ||
          []) as ProblemOption[]
        this.applyDetail(detail)
        // 基于后端原始草稿判断：已有内容的草稿视为“已保存”（重新打开时无须再问）；新建空草稿=false（在自动选企业之前，避免自动选择干扰判定）
        this.hasSaved = !this.isBlankDraft(detail)
        // 仅一个可选单位时自动选中
        if (!this.form.enterprise_id && this.enterpriseOptions.length === 1) {
          this.form.enterprise_id = this.enterpriseOptions[0].id
        }
        // 快照/基线在自动选企业之后拍摄：自动选择不计入“用户改动”，避免单企业账号误判为已编辑
        this.captureSnapshot()
        this.loaded = true
        // 初始化期间用户已按过返回（被拦截待命）：加载完成后补一次去留决策
        if (this.backPending && !this.exiting) {
          this.backPending = false
          this.decideExit()
        }
      } catch (e) {
        console.error('初始化失败', e)
        // 加载失败：标记为“已保存”，返回时静默退出，避免对残缺表单误存/误删
        this.loaded = true
        this.hasSaved = true
        this.backPending = false
      }
    },
    /** 回填草稿：contact 单字段按“尾部手机号”拆分为联系人/电话 */
    applyDetail(detail: OrderDetail) {
      const { name, phone } = splitContact(detail.contact)
      this.form.enterprise_id = detail.enterprise_id || ''
      this.form.category_id = detail.category_id || ''
      this.form.property_id = detail.property_id || ''
      this.form.description = detail.description || ''
      this.form.urgency = detail.urgency || ''
      this.form.room = detail.room || ''
      this.form.contact_name = name
      this.form.contact_phone = phone
      this.imageList = (detail.images || []).map((img) => ({ url: img.url }))

      if (this.form.category_id) {
        const cat = this.categoryOptions.find((c) => c.id === this.form.category_id)
        this.propertyOptions = cat ? cat.properties || [] : []
      }
    },
    onEnterpriseChange(e: { detail: { value: number } }) {
      const opt = this.enterpriseOptions[e.detail.value]
      if (opt) this.form.enterprise_id = opt.id
    },
    /** 切换设备类型（大类）：重置问题类别（属性） */
    onCategoryChange(e: { detail: { value: number } }) {
      const opt = this.categoryOptions[e.detail.value]
      if (!opt) return
      this.form.category_id = opt.id
      this.form.property_id = ''
      this.propertyOptions = opt.properties || []
    },
    onPropertyChange(e: { detail: { value: number } }) {
      const opt = this.propertyOptions[e.detail.value]
      if (!opt) return
      this.form.property_id = opt.id
    },
    /** 常见问题：以“追加”方式写入问题名称本身（不整体替换，用户可继续编辑） */
    onProblemPick(p: ProblemOption) {
      const text = (p.name || '').trim()
      if (!text) return
      const cur = this.form.description.trim()
      this.form.description = cur ? `${cur} ${text}` : text
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
    /** 联系人 + 电话 → 单字段 contact（后端拆分前过渡方案，见文档第十一章 #1） */
    contactValue(): string {
      return `${this.form.contact_name.trim()} ${this.form.contact_phone.trim()}`.trim()
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
      const contact = this.contactValue()
      if (all || contact) payload.contact = contact
      return payload
    },
    async saveDraft(exitAfter?: boolean) {
      if (this.saving) return
      this.saving = true
      try {
        await http.put(`/orders/${this.orderId}`, this.buildPayload(false))
        uni.showToast({ title: '已保存草稿', icon: 'success' })
        // 标记为已保存，并以当前内容重置快照基线（之后无改动则不再询问）
        this.hasSaved = true
        this.captureSnapshot()
        // 返回键场景下：保存后退出
        if (exitAfter === true) this.doExit()
      } catch (e) {
        console.error('保存草稿失败', e)
      } finally {
        this.saving = false
      }
    },
    /**
     * 删除草稿。
     * - 来自底部“删除”按钮（fromBack=false）：先二次确认再删。
     * - 来自返回键拦截（fromBack=true）：外层弹窗已确认“不保存”，不再二次确认。
     */
    async deleteDraft(fromBack = false) {
      if (fromBack !== true) {
        const res = await uni.showModal({ title: '提示', content: '确定删除该草稿吗？删除后不可恢复' })
        if (!res.confirm) return false
      }
      try {
        await http.delete(`/orders/${this.orderId}`)
        uni.showToast({ title: '已删除', icon: 'success' })
        this.doExit()
        return true
      } catch (e) {
        console.error('删除草稿失败', e)
        return false
      }
    },
    /** 统一退出：解除返回拦截后返回上一页（避免 navigateBack 被 page-container 拦下） */
    doExit() {
      this.exiting = true
      this.containerShow = false
      setTimeout(() => uni.navigateBack(), 500)
    },
    /** 重新武装 page-container：先收起(已 false)再延迟一帧恢复，避免同帧 show 翻转不被微信识别导致后续返回不再拦截 */
    rearmContainer() {
      setTimeout(() => {
        if (!this.exiting) this.containerShow = true
      }, 80)
    },
    /** 加载完成后拍摄表单快照，供脏检查使用 */
    captureSnapshot() {
      this.snapshot = {
        form: JSON.parse(JSON.stringify(this.form)) as Record<string, unknown>,
        images: this.imageList.map((i) => i.url)
      }
    },
    /** 当前内容相对加载快照是否发生改动 */
    isDirty(): boolean {
      if (!this.snapshot) return false
      const cur = this.imageList.map((i) => i.url)
      if (cur.length !== this.snapshot.images.length) return true
      for (let i = 0; i < cur.length; i++) {
        if (cur[i] !== this.snapshot.images[i]) return true
      }
      return JSON.stringify(this.form) !== JSON.stringify(this.snapshot.form)
    },
    /** 后端返回的草稿是否为“空白草稿”：所有可填字段均无内容时才算空白（不依赖前端自动选择，判定最准确） */
    isBlankDraft(d: OrderDetail): boolean {
      const blank = (v: unknown) => v === '' || v === null || v === undefined
      return (
        blank(d.enterprise_id) &&
        blank(d.category_id) &&
        blank(d.property_id) &&
        blank(d.description) &&
        blank(d.urgency) &&
        blank(d.room) &&
        blank(d.contact) &&
        (!d.images || d.images.length === 0)
      )
    },
    /**
     * 返回去留决策（H5/App onBackPress 与小程序 @beforeleave 共用）。
     * 规则（以“是否已保存”为核心）：
     * - 已保存且未改动（含重新打开的已填草稿、或本会话点过“保存草稿”）→ 直接退出，不询问。
     * - 初始化未完成 → 拦截但不弹（避免与 init 回填竞态），记 backPending，加载完成后自动补决策。
     * - 其余（新建空草稿未保存 / 已填未保存 / 保存后又改了内容）→ 弹三按钮询问：
     *   保存=走保存逻辑并退出；不保存=走删除逻辑并退出；取消=留在页面。
     */
    decideExit() {
      // 初始化未完成：不静默退出、也不弹（表单尚未回填），待命后由 init 补弹
      if (!this.loaded) {
        this.backPending = true
        this.rearmContainer()
        return
      }
      // 弹窗已打开：仅拦截本次返回手势（重武装），不重复弹窗，避免直接离开丢草稿
      if (this.showExitDialog) {
        this.rearmContainer()
        return
      }
      // 已保存且未改动：直接退出，不询问
      if (this.hasSaved && !this.isDirty()) {
        this.doExit()
        return
      }
      this.showExitDialog = true
      this.rearmContainer()
    },
    /** 返回弹窗：保存并退出（先解除拦截再执行，避免保存期间被返回打断） */
    async onExitSave() {
      this.showExitDialog = false
      this.containerShow = false
      await this.saveDraft(true)
    },
    /** 返回弹窗：不保存（删除草稿）并退出 */
    async onExitDiscard() {
      this.showExitDialog = false
      this.containerShow = false
      await this.deleteDraft(true)
    },
    /** 返回弹窗：取消，留在页面（恢复 page-container 拦截） */
    onExitCancel() {
      this.showExitDialog = false
      this.containerShow = true
    },
    /** 微信小程序：左上角“<”返回键触发（page-container @beforeleave） */
    onContainerLeave() {
      // 无论去留都先收起容器，消费本次返回手势（微信要求 beforeleave 内将 show 置 false）
      this.containerShow = false
      // 已在退出流程（保存/不保存/提交/删除后）：放行，等 navigateBack
      if (this.exiting) return
      this.decideExit()
    },
    validateForm(): string {
      const f = this.form
      if (!f.enterprise_id) return '请选择报修单位'
      if (!f.description || f.description.trim().length < 1) return '请描述遇到的问题'
      if (f.description.length > 500) return '报修描述不能超过500字'
      if (!f.room || f.room.trim().length < 1) return '请输入位置'
      if (!f.contact_name.trim()) return '请输入联系人'
      if (!/^1\d{10}$/.test(f.contact_phone.trim())) return '请输入正确的11位手机号'
      if (!f.category_id) return '请选择设备类型'
      if (!f.property_id) return '请选择问题类别'
      if (!f.urgency) return '请选择紧急程度'
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
        // V1.3：提交后返回我的工单列表（onShow 自动刷新）
        this.exiting = true
        this.containerShow = false
        setTimeout(() => uni.navigateBack(), 600)
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
  background-color: #f5f6f8;
}

.section {
  background-color: #ffffff;
  margin: 20rpx 24rpx;
  border-radius: 20rpx;
  padding: 28rpx;

  .section-title {
    font-size: 30rpx;
    font-weight: 600;
    color: #1a1a1a;
    margin-bottom: 24rpx;
  }
}

/* ① 图片 + 描述 + 常见问题 */
.upload-grid {
  display: flex;
  flex-wrap: wrap;

  .upload-item,
  .upload-add {
    width: 180rpx;
    height: 180rpx;
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
      font-size: 56rpx;
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

.desc-textarea {
  width: 100%;
  min-height: 180rpx;
  margin-top: 8rpx;
  padding: 20rpx;
  box-sizing: border-box;
  background-color: #f7f8fa;
  border-radius: 16rpx;
  font-size: 28rpx;
  color: #333333;
  line-height: 1.6;
}

.input-placeholder {
  color: #bbbbbb;
}

/* 常见问题 chips：次要样式，不喧宾夺主 */
.issue-chips {
  display: flex;
  flex-wrap: wrap;
  margin-top: 20rpx;

  .issue-chip {
    padding: 8rpx 20rpx;
    margin-right: 16rpx;
    margin-bottom: 16rpx;
    background-color: #f5f6f8;
    border-radius: 999rpx;
    font-size: 22rpx;
    color: #888888;
  }
}

/* ② ③ 表单行 */
.form-cell {
  display: flex;
  align-items: center;
  min-height: 96rpx;
  border-bottom: 1rpx solid #f2f3f5;

  &:last-child {
    border-bottom: none;
  }

  .cell-label {
    width: 160rpx;
    font-size: 28rpx;
    color: #333333;
    flex-shrink: 0;
    text-align: right;
    padding-right: 24rpx;
  }

  .cell-input {
    flex: 1;
    font-size: 28rpx;
    color: #333333;
    padding: 20rpx 0;
    text-align: left;
  }

  .cell-value {
    flex: 1;
    text-align: left;
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

/* 设备类型 + 问题类别：同一行对半分 */
.cat-row {
  display: flex;
  align-items: stretch;
  gap: 20rpx;

  .cat-picker {
    flex: 1 1 0;
    min-width: 0;
  }

  .form-cell {
    .cell-label {
      width: 140rpx;
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

/* 返回拦截三按钮弹窗 */
.exit-popup {
  padding: 40rpx 32rpx 32rpx;
  width: 100%;
  box-sizing: border-box;

  .exit-title {
    font-size: 34rpx;
    font-weight: 600;
    color: #1a1a1a;
    text-align: center;
  }

  .exit-tip {
    margin-top: 16rpx;
    font-size: 26rpx;
    color: #8a8f99;
    text-align: center;
    line-height: 1.6;
  }

  .exit-actions {
    display: flex;
    gap: 20rpx;
    margin-top: 40rpx;

    :deep(.wd-button) {
      flex: 1 1 0;
      min-width: 0;
    }
  }
}
</style>
