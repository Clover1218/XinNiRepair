<template>
  <view class="page">
    <!-- 本地草稿恢复提示条（新建模式） -->
    <view v-if="draftRestored" class="restore-bar">
      <text class="restore-text">已为你恢复上次未完成的报修</text>
      <text class="restore-action" @click.stop="discardRestored">放弃</text>
    </view>

    <!-- ① 发生了什么？ -->
    <view class="section">
      <view class="section-title">① 发生了什么？</view>

      <!-- 设备类型（大类） + 问题类别（属性）：点击打开两级弹层 -->
      <view class="form-cell" @click="openCategoryPopup">
        <view class="cell-label">设备类型</view>
        <view class="cell-value" :class="{ 'is-placeholder': !form.category_id }">
          {{ categorySummary || '请选择设备类型' }}
          <text class="cell-arrow">›</text>
        </view>
      </view>

      <view class="upload-grid">
        <view v-for="(img, index) in imageList" :key="index" class="upload-item">
          <image
            class="upload-img"
            :src="img.preview || img.url"
            mode="aspectFill"
            @click="previewImage(index)"
            @error="onImageError(index)"
          ></image>
          <view class="upload-del" @click.stop="removeImage(index)">×</view>
          <view v-if="img.failed" class="upload-retry" @click.stop="retryImage(index)">重试</view>
        </view>
        <view v-if="imageList.length < 9" class="upload-add" @click="chooseImage">
          <text class="upload-add-icon">+</text>
          <text class="upload-add-text">添加照片</text>
        </view>
      </view>

      <view class="desc-wrap">
        <textarea
          class="desc-textarea"
          v-model="form.description"
          placeholder="描述一下遇到的问题"
          placeholder-class="input-placeholder"
          maxlength="500"
          :show-confirm-bar="false"
        />
        <view class="desc-footer">
          <text
            v-if="form.description"
            class="desc-clear-text"
            @click.stop="clearDescription"
          >
            清空
          </text>
          <text class="desc-count">{{ form.description.length }}/500</text>
        </view>
      </view>

      <!-- 常见问题：点击以“追加”方式写入问题名称，不整体替换 -->
      <scroll-view v-if="problemOptions.length > 0" class="issue-chips" scroll-y>
        <view
          v-for="p in problemOptions"
          :key="p.id"
          class="issue-chip"
          @click="onProblemPick(p)"
        >
          {{ p.name }}
        </view>
      </scroll-view>
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

      <!-- 多单位时的「切换单位」提示（切换后立即同步全局上下文） -->
      <view v-if="enterpriseOptions.length > 1" class="ent-hint">
        <text class="ent-hint-text">默认单位：{{ enterpriseLabel || '未选择' }}，可在下方切换</text>
        <text class="ent-hint-btn" @click.stop="switchEnterprise">切换单位</text>
      </view>

      <!-- 位置：手填 + 常驻 chips + 聚焦历史浮层 -->
      <view class="room-block">
        <view class="form-cell">
          <view class="cell-label">位置</view>
          <input
            class="cell-input"
            v-model="form.room"
            placeholder="如：二楼会议室"
            placeholder-class="input-placeholder"
            maxlength="20"
            @focus="onRoomFocus"
            @blur="onRoomBlur"
            @confirm="onRoomBlur"
          />
          <view v-if="form.room" class="cell-clear" @click.stop="clearCell('room')">×</view>
        </view>

        <!-- 聚焦时弹出历史候选（类似淘宝/美团地址历史），随输入过滤 -->
        <view v-if="roomFocused && roomSuggestions.length > 0" class="room-suggest" @click.stop>
          <view
            v-for="r in roomSuggestions"
            :key="r"
            class="room-suggest-item"
            @mousedown.prevent="pickRoom(r)"
            @click.stop="pickRoom(r)"
          >
            <text class="room-suggest-text">{{ r }}</text>
            <text class="room-suggest-del" @click.stop="removeRoom(r)" @mousedown.stop="removeRoom(r)">
              ×
            </text>
          </view>
        </view>
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
        <view v-if="form.contact_name" class="cell-clear" @click.stop="clearCell('contact_name')">×</view>
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
        <view
          v-if="form.contact_phone"
          class="cell-clear"
          @click.stop="clearCell('contact_phone')"
        >×</view>
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

  <!-- 设备类型 / 问题类别 两级选择弹层 -->
  <wd-popup v-model="catPopupVisible" position="bottom" round custom-style="height: 60vh;">
    <view class="cat-popup">
      <view class="cat-popup-head">
        <text class="cat-popup-cancel" @click.stop="catPopupVisible = false">关闭</text>
        <text class="cat-popup-title">选择设备类型</text>
        <text class="cat-popup-placeholder"></text>
      </view>
      <view class="cat-popup-body">
        <scroll-view class="cat-col" scroll-y :scroll-into-view="catScrollInto">
          <view
            v-for="c in categoryOptions"
            :id="'cat-' + c.id"
            :key="c.id"
            class="cat-col-item"
            :class="{ 'is-active': c.id === catActiveId }"
            @click="onPopupCategory(c)"
          >{{ c.name }}</view>
        </scroll-view>
        <scroll-view class="cat-col cat-col-right" scroll-y>
          <view
            v-for="p in popupPropertyOptions"
            :key="p.id"
            class="cat-col-item"
            :class="{ 'is-active': p.id === catActivePropertyId }"
            @click="onPopupProperty(p)"
          >{{ p.name }}</view>
          <view v-if="popupPropertyOptions.length === 0" class="cat-empty">该类型暂无可选问题类别</view>
        </scroll-view>
      </view>
    </view>
  </wd-popup>

  <!-- 返回拦截：三按钮（保存 / 不保存 / 取消留页）。仅编辑「已落库草稿」时使用 -->
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
import { http, uploadOrderImage, uploadOrderImageFree } from '@/utils/request'
import { splitContact } from '@/utils/format'
import {
  clearLocalDraft,
  localId,
  localImageName,
  MAX_DRAFT_IMAGES,
  persistImage,
  pushRoomHistory,
  readLastRoom,
  readLocalDraft,
  readRoomHistory,
  removeRoomHistory,
  writeLocalDraft
} from '@/utils/localDraft'
import type { LocalDraft, RoomHistoryItem } from '@/utils/localDraft'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUserStore } from '@/stores/user'
import type { CategoryOption, OptionsResult, OrderDetail, PropertyOption, ProblemOption } from '@/types'

/** 图片项：本地队列与已上传项统一用这一个结构 */
interface ImageItem {
  /** 服务端图片记录 ID（预上传返回的 temp_id；未上传为空） */
  id?: string
  /** 图床 URL（未上传为空） */
  url: string
  /** 本地预览路径（优先展示；上传成功后保留以免闪烁） */
  preview: string
  /** 本地持久文件路径，上传时使用 */
  path: string
  name: string
  size: number
  /** 是否已在服务端落库 */
  uploaded: boolean
  /** 上传失败标记（展示「重试」角标） */
  failed: boolean
}

export default defineComponent({
  data() {
    return {
      /** 服务端工单 ID：空串代表「新建未落库」 */
      orderId: '',
      submitting: false,
      saving: false,
      /** 防止返回键弹窗与 navigateBack 互相触发的死循环 */
      exiting: false,
      /** 草稿是否已加载完成（加载完成前不拦截返回） */
      loaded: false,
      /** 加载完成后的表单快照，用于脏检查 */
      snapshot: null as null | { form: Record<string, unknown>; images: string[] },
      /** 草稿是否已“保存”过 */
      hasSaved: false,
      /** 返回拦截三按钮弹窗（仅服务端草稿模式）显隐 */
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
      generalProblems: [] as ProblemOption[],
      /** 本地草稿是否已恢复展示（顶部提示条） */
      draftRestored: false,
      /** 本地草稿写入防抖定时器 */
      draftTimer: null as ReturnType<typeof setTimeout> | null,
      /** 两级选择弹层显隐 */
      catPopupVisible: false,
      /** 弹层当前选中的大类 / 属性 */
      catActiveId: '',
      catActivePropertyId: '',
      catScrollInto: '',
      /** 是否正在补传图片队列 */
      flushing: false,
      /** 位置历史（当前单位维度，最近使用在前） */
      roomHistory: [] as RoomHistoryItem[],
      /** 位置输入框聚焦中：展示完整历史候选浮层 */
      roomFocused: false
    }
  },
  computed: {
    /** 是否新建模式（尚未分配服务端 ID） */
    isNewDraft(): boolean {
      return !this.orderId
    },
    enterpriseIndex(): number {
      return this.enterpriseOptions.findIndex((o) => o.id === this.form.enterprise_id)
    },
    enterpriseLabel(): string {
      const o = this.enterpriseOptions.find((x) => x.id === this.form.enterprise_id)
      return o ? o.name : ''
    },
    categoryLabel(): string {
      const c = this.categoryOptions.find((x) => x.id === this.form.category_id)
      return c ? c.name : ''
    },
    propertyLabel(): string {
      const p = this.propertyOptions.find((x) => x.id === this.form.property_id)
      return p ? p.name : ''
    },
    /** 「大类 · 属性」合并展示 */
    categorySummary(): string {
      if (!this.form.category_id) return ''
      return this.form.property_id
        ? `${this.categoryLabel} · ${this.propertyLabel}`
        : this.categoryLabel
    },
    urgencyLabel(): string {
      const u = this.urgencyOptions.find((o) => o.value === this.form.urgency)
      return u ? u.label : ''
    },
    /** 弹层右侧属性列表 */
    popupPropertyOptions(): PropertyOption[] {
      const cat = this.categoryOptions.find((c) => c.id === this.catActiveId)
      return (cat && cat.properties) || []
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
    },
    /** 聚焦浮层候选：随输入过滤（空输入时给全部历史） */
    roomSuggestions(): string[] {
      const kw = this.form.room.trim()
      return this.roomHistory
        .map((it) => it.room)
        .filter((r) => (kw ? r.indexOf(kw) >= 0 : true))
        .slice(0, 8)
    }
  },
  watch: {
    /** 表单任何变化都做一次本地暂存（新建模式；500ms 防抖） */
    form: {
      handler() {
        this.scheduleLocalSave()
      },
      deep: true
    },
    /** 单位切换后位置历史要跟着换（历史按「用户 + 单位」隔离） */
    'form.enterprise_id'() {
      this.loadRoomHistory()
    }
  },
  onLoad(options: Record<string, string>) {
    this.orderId = options.id || ''
    if (this.orderId) {
      this.init()
    } else {
      this.initNew()
    }
  },
  onUnload() {
    // 页面销毁：立即落盘一次并解绑返回拦截，避免强杀丢失内容
    this.flushLocalDraft()
    this.disarmReturnGuard()
  },
  /**
   * 返回键拦截（H5 / App 端生效）。
   * 微信小程序端左上角“<”由 wx.enableAlertBeforeUnload 原生弹窗拦截。
   */
  onBackPress() {
    // #ifdef MP-WEIXIN
    return false
    // #endif
    if (this.exiting) return false
    this.decideExit()
    return true
  },
  methods: {
    /** 编辑已有草稿：加载字典 + 回填详情 */
    async init() {
      await this.ensureUser()
      try {
        const [opts, detail] = await Promise.all([
          http.get<OptionsResult>('/orders/options'),
          http.get<OrderDetail>(`/orders/${this.orderId}`)
        ])
        this.options = opts
        this.enterpriseOptions = opts.enterprises || []
        this.categoryOptions = opts.categories || []
        this.urgencyOptions = opts.urgent_levels || []
        this.generalProblems = ((opts as unknown as Record<string, unknown>).general_problems ||
          []) as ProblemOption[]
        this.applyDetail(detail)
        this.hasSaved = !this.isBlankDraft(detail)
        // 仅一个可选单位时自动选中
        if (!this.form.enterprise_id && this.enterpriseOptions.length === 1) {
          this.form.enterprise_id = this.enterpriseOptions[0].id
        }
        this.captureSnapshot()
        this.loaded = true
        this.armReturnGuard()
        if (this.backPending && !this.exiting) {
          this.backPending = false
          this.decideExit()
        }
      } catch (e) {
        console.error('初始化失败', e)
        this.loaded = true
        this.hasSaved = true
        this.backPending = false
      }
    },
    /**
     * 确保本地已有用户信息（默认联系人、位置记忆、本地草稿 key 均依赖它）。
     * 冷启动/分享直达等场景下 userStore 尚无数据，需在此补齐。
     */
    async ensureUser() {
      const store = useUserStore()
      if (store.userInfo) return
      try {
        const ok = await store.ensureLoggedIn()
        if (ok) await store.fetchUserInfo()
      } catch (e) {
        console.error('加载用户信息失败', e)
      }
    },
    /**
     * 新建：不发任何建单请求。
     * 优先恢复本地未完成的草稿，否则按「我的企业上下文 + 报修人信息 + 位置记忆」填默认值。
     */
    async initNew() {
      await this.ensureUser()
      try {
        const opts = await http.get<OptionsResult>('/orders/options')
        this.options = opts
        this.enterpriseOptions = opts.enterprises || []
        this.categoryOptions = opts.categories || []
        this.urgencyOptions = opts.urgent_levels || []
        this.generalProblems = ((opts as unknown as Record<string, unknown>).general_problems ||
          []) as ProblemOption[]
      } catch (e) {
        console.error('加载字典失败', e)
      }

      const restored = readLocalDraft()
      if (restored && !restored.orderId) {
        this.applyLocalDraft(restored)
        this.draftRestored = true
        this.hasSaved = false
      } else {
        this.applyDefaultValues()
        this.hasSaved = false
      }
      this.loaded = true
      this.captureSnapshot()
    },
    /** 默认值：报修单位取当前「我的企业」上下文，联系人/电话取报修人，位置取最近一次填写 */
    applyDefaultValues() {
      const user = useUserStore().userInfo
      const entStore = useEnterpriseStore()
      this.form.enterprise_id = entStore.currentEnterpriseId || ''
      // 上下文单位不在可选列表中时不强行套用
      if (this.form.enterprise_id && !this.enterpriseOptions.some((o) => o.id === this.form.enterprise_id)) {
        this.form.enterprise_id = ''
      }
      // 单单位时自动选中，多单位时保持未选让用户主动确认
      if (!this.form.enterprise_id && this.enterpriseOptions.length === 1) {
        this.form.enterprise_id = this.enterpriseOptions[0].id
      }
      this.form.contact_name = user?.nickname || ''
      this.form.contact_phone = /^1\d{10}$/.test(user?.phone || '') ? String(user?.phone) : ''
      if (user && this.form.enterprise_id) {
        this.form.room = readLastRoom(user.id, this.form.enterprise_id)
      }
      this.loadRoomHistory()
    },
    /** 回填服务端草稿详情 */
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
      this.imageList = (detail.images || []).map((img) => ({
        id: img.id,
        url: img.url,
        preview: img.url,
        path: img.url,
        name: '',
        size: 0,
        uploaded: true,
        failed: false
      }))

      if (this.form.category_id) {
        const cat = this.categoryOptions.find((c) => c.id === this.form.category_id)
        this.propertyOptions = cat ? cat.properties || [] : []
      }
    },
    /** 回填本地草稿 */
    applyLocalDraft(draft: LocalDraft) {
      this.form.enterprise_id = draft.form.enterprise_id || ''
      this.form.category_id = draft.form.category_id || ''
      this.form.property_id = draft.form.property_id || ''
      this.form.description = draft.form.description || ''
      this.form.urgency = draft.form.urgency || ''
      this.form.room = draft.form.room || ''
      this.form.contact_name = draft.form.contact_name || ''
      this.form.contact_phone = draft.form.contact_phone || ''
      this.imageList = (draft.images || []).map((img) => ({
        id: img.id || '',
        url: img.url || '',
        preview: img.preview || img.path || img.url,
        path: img.path || img.preview || '',
        name: img.name || '',
        size: img.size || 0,
        uploaded: !!img.uploaded,
        failed: false
      }))
      if (this.form.category_id) {
        const cat = this.categoryOptions.find((c) => c.id === this.form.category_id)
        this.propertyOptions = cat ? cat.properties || [] : []
      }
      // 单位已失效（用户已退出该企业）时清空，避免带出无法提交的数据
      if (this.form.enterprise_id && !this.enterpriseOptions.some((o) => o.id === this.form.enterprise_id)) {
        this.form.enterprise_id = ''
      }
    },
    /** 放弃恢复的本地草稿：清空后回到默认值 */
    discardRestored() {
      clearLocalDraft()
      this.resetForm()
      this.applyDefaultValues()
      this.draftRestored = false
      this.hasSaved = false
      this.captureSnapshot()
    },
    resetForm() {
      this.form.enterprise_id = ''
      this.form.category_id = ''
      this.form.property_id = ''
      this.form.description = ''
      this.form.urgency = ''
      this.form.room = ''
      this.form.contact_name = ''
      this.form.contact_phone = ''
      this.imageList = []
      this.propertyOptions = []
    },
    /* ---------- 字典选择 ---------- */
    onEnterpriseChange(e: { detail: { value: number } }) {
      const opt = this.enterpriseOptions[e.detail.value]
      if (opt) this.form.enterprise_id = opt.id
    },
    /** 多单位切换：同时更新本单与「我的企业」全局上下文 */
    switchEnterprise() {
      const names = this.enterpriseOptions.map((o) => o.name)
      uni.showActionSheet({
        itemList: names,
        success: (res) => {
          const opt = this.enterpriseOptions[res.tapIndex]
          if (!opt) return
          this.form.enterprise_id = opt.id
          useEnterpriseStore().setCurrent(opt.id)
          const user = useUserStore().userInfo
          if (user) {
            const remembered = readLastRoom(user.id, opt.id)
            if (remembered) this.form.room = remembered
          }
          this.loadRoomHistory()
          uni.showToast({ title: `已切换默认单位：${opt.name}`, icon: 'none' })
        },
        fail: () => {}
      })
    },
    /* ---------- 位置历史 ---------- */
    /** 载入当前单位维度的位置历史 */
    loadRoomHistory() {
      const user = useUserStore().userInfo
      if (!user || !this.form.enterprise_id) {
        this.roomHistory = []
        return
      }
      this.roomHistory = readRoomHistory(user.id, this.form.enterprise_id)
    },
    onRoomFocus() {
      this.loadRoomHistory()
      this.roomFocused = true
    },
    /**
     * blur 晚一拍收起：小程序端点候选时 blur 先于 tap 触发，
     * 留 250ms 让点击落到浮层上（H5 端另有 mousedown.prevent 兜底）。
     */
    onRoomBlur() {
      setTimeout(() => {
        this.roomFocused = false
      }, 250)
    },
    /** 选中历史位置 */
    pickRoom(room: string) {
      this.form.room = room
      this.roomFocused = false
    },
    /** 删除单条历史 */
    removeRoom(room: string) {
      const user = useUserStore().userInfo
      if (!user || !this.form.enterprise_id) return
      removeRoomHistory(user.id, this.form.enterprise_id, room)
      this.loadRoomHistory()
    },
    /** 打开两级选择弹层，并定位到当前已选项 */
    openCategoryPopup() {
      this.catActiveId = this.form.category_id || (this.categoryOptions[0] && this.categoryOptions[0].id) || ''
      this.catActivePropertyId = this.form.property_id || ''
      this.catScrollInto = this.form.category_id ? `cat-${this.form.category_id}` : ''
      this.catPopupVisible = true
    },
    /** 选中大类：属性列表自动切换，实现“选完直接跳到问题类别” */
    onPopupCategory(c: CategoryOption) {
      this.catActiveId = c.id
      this.catActivePropertyId = ''
    },
    /** 选中属性：立即回填并关闭弹层 */
    onPopupProperty(p: PropertyOption) {
      this.form.category_id = this.catActiveId
      this.form.property_id = p.id
      const cat = this.categoryOptions.find((c) => c.id === this.catActiveId)
      this.propertyOptions = cat ? cat.properties || [] : []
      this.catPopupVisible = false
    },
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
    /* ---------- 描述区 ---------- */
    clearDescription() {
      this.form.description = ''
    },
    clearCell(field: 'room' | 'contact_name' | 'contact_phone') {
      this.form[field] = ''
    },
    /* ---------- 图片 ---------- */
    chooseImage() {
      const remain = MAX_DRAFT_IMAGES - this.imageList.length
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
          await this.addImages(validFiles.map((f) => ({ path: f.path, name: f.name, size: f.size })))
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
            name: localImageName(this.imageList.length),
            size: (f as any).size || 0
          }))
          if (files.length === 0) return
          const validFiles = files.filter((f) => f.size <= 5 * 1024 * 1024)
          if (validFiles.length === 0) {
            uni.showToast({ title: '单张图片不能超过5MB', icon: 'none' })
            return
          }
          await this.addImages(validFiles)
        },
        fail: (err) => {
          console.error('选择图片失败', err)
        }
      })
    },
    /**
     * 选择图片后「立即预上传」：本地落盘做预览，同时调用上传接口写入 temporary 记录（未绑定哨兵）。
     * 上传成功拿到服务端 id/url；失败标记 failed 供用户重试。
     * 真正的「计入工单」发生在保存/提交时（persistOrder 传递 image_ids，后端 diff 关联）。
     */
    async addImages(files: { path: string; name?: string; size: number }[]) {
      uni.showLoading({ title: '处理中...' })
      for (const file of files) {
        const saved = await persistImage(file.path)
        const item: ImageItem = {
          id: '',
          url: '',
          preview: saved || file.path,
          path: saved || file.path,
          name: file.name || localImageName(this.imageList.length),
          size: file.size || 0,
          uploaded: false,
          failed: false
        }
        this.imageList.push(item)
        await this.uploadOne(item)
      }
      uni.hideLoading()
      this.scheduleLocalSave()
    },
    /**
     * 上传单张图片（带 order_id 走普通接口，无则走预上传接口）。
     * 成功填充 id/url/preview 并置 uploaded；失败置 failed。
     */
    async uploadOne(item: ImageItem) {
      if (!item.path || item.path.indexOf('http') === 0) return
      try {
        const data = this.orderId
          ? await uploadOrderImage(this.orderId, item.path)
          : await uploadOrderImageFree(item.path)
        item.id = data.id
        item.url = data.url
        item.preview = data.url
        item.uploaded = true
        item.failed = false
        this.scheduleLocalSave()
      } catch (e) {
        item.failed = true
        console.error('图片上传失败', e)
      }
    },
    /** 单张重试 */
    async retryImage(index: number) {
      const item = this.imageList[index]
      if (!item || item.uploaded) return
      item.failed = false
      await this.uploadOne(item)
    },
    previewImage(index: number) {
      const urls = this.imageList.map((i) => i.url || i.preview).filter(Boolean)
      uni.previewImage({
        current: this.imageList[index].url || this.imageList[index].preview,
        urls
      })
    },
    onImageError(index: number) {
      const img = this.imageList[index]
      // 本地预览失败：回退到图床地址；都没有则提示
      if (img && img.url && img.preview !== img.url) {
        img.preview = img.url
        return
      }
      uni.showToast({ title: '图片加载失败，请重新选择', icon: 'none' })
    },
    removeImage(index: number) {
      this.imageList.splice(index, 1)
      this.scheduleLocalSave()
    },
    /* ---------- 本地草稿 ---------- */
    scheduleLocalSave() {
      if (!this.loaded) return
      if (this.draftTimer) clearTimeout(this.draftTimer)
      this.draftTimer = setTimeout(() => this.flushLocalDraft(), 500)
    },
    /** 立即写入本地草稿（仅新建模式；已落库的由服务端负责） */
    flushLocalDraft() {
      if (this.draftTimer) {
        clearTimeout(this.draftTimer)
        this.draftTimer = null
      }
      // 注意：退出流程也需要落盘（doExit 先置 exiting 再调用此处），故不能按 exiting 提前返回
      if (!this.loaded) return
      // 仅「新建未落库」模式写本地草稿。
      // 编辑已有草稿时不读写本地草稿：那份本地草稿属于“另一张未提交的新单”，
      // 若在此清理会误删用户尚未提交的内容（它会在新单保存/放弃/过期时才被处理）。
      if (!this.isNewDraft) return
      const empty =
        !this.form.enterprise_id &&
        !this.form.category_id &&
        !this.form.property_id &&
        !this.form.description &&
        !this.form.urgency &&
        !this.form.room &&
        !this.form.contact_name &&
        !this.form.contact_phone &&
        this.imageList.length === 0
      if (empty) return
      // 仅在用户真正改动过之后才落盘：默认值/恢复内容本身不算草稿，
      // 否则一进页面就生成“未完成草稿”，下次会被无谓地提示续填
      if (!this.isDirty()) return
      writeLocalDraft({
        v: 1,
        localId: localId(),
        updatedAt: Date.now(),
        form: JSON.parse(JSON.stringify(this.form)),
        images: this.imageList.map((i) => ({
          id: i.id,
          path: i.path,
          preview: i.preview,
          name: i.name,
          size: i.size,
          url: i.url,
          uploaded: i.uploaded
        })),
        orderId: this.orderId
      })
    },
    /* ---------- 提交 / 保存 ---------- */
    /** 联系人 + 电话 → 单字段 contact（后端拆分前过渡方案） */
    contactValue(): string {
      return `${this.form.contact_name.trim()} ${this.form.contact_phone.trim()}`.trim()
    },
    /** 构造提交参数：仅传已上传图片的 image_ids（预上传记录），其余字段按需传 */
    buildPayload(all = false): Record<string, unknown> {
      const f = this.form
      const payload: Record<string, unknown> = {
        image_ids: this.imageList.filter((i) => i.uploaded && i.id).map((i) => i.id as string)
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
    /**
     * 持久化到服务端。
     * 未分配 ID 时先用完整内容一次性建单（"先内容后分配 ID"），拿到 ID 后再补传本地图片队列。
     */
    async persistOrder(all = false): Promise<boolean> {
      const payload = this.buildPayload(all)
      try {
        if (!this.orderId) {
          const data = await http.post<{ order_id: string }>('/orders', payload)
          this.orderId = data.order_id
        } else {
          await http.put(`/orders/${this.orderId}`, payload)
        }
        await this.rememberRoom()
        clearLocalDraft()
        this.draftRestored = false
        this.hasSaved = true
        this.captureSnapshot()
        this.armReturnGuard()
        return true
      } catch (e) {
        console.error('保存失败', e)
        return false
      }
    },
    /** 位置记忆：保存/提交成功后写入 */
    async rememberRoom() {
      const user = useUserStore().userInfo
      if (user && this.form.enterprise_id) {
        pushRoomHistory(user.id, this.form.enterprise_id, this.form.room)
        this.loadRoomHistory()
      }
    },
    async saveDraft(exitAfter?: boolean) {
      if (this.saving) return
      this.saving = true
      const ok = await this.persistOrder(false)
      this.saving = false
      if (!ok) return
      uni.showToast({ title: '已保存草稿', icon: 'success' })
      if (exitAfter === true) this.doExit()
    },
    /**
     * 删除草稿。
     * - 新建未落库：仅清本地草稿（服务端无记录，天然干净）。
     * - 已落库：调用服务端删除接口 + 清本地。
     */
    async deleteDraft(fromBack = false) {
      if (fromBack !== true) {
        const res = await uni.showModal({ title: '提示', content: '确定删除该草稿吗？删除后不可恢复' })
        if (!res.confirm) return false
      }
      try {
        if (this.orderId) {
          await http.delete(`/orders/${this.orderId}`)
        }
        clearLocalDraft()
        uni.showToast({ title: '已删除', icon: 'success' })
        this.doExit()
        return true
      } catch (e) {
        console.error('删除草稿失败', e)
        return false
      }
    },
    /* ---------- 退出流程 ---------- */
    /** 统一退出：先落盘本地草稿再返回 */
    doExit() {
      this.exiting = true
      if (this.isNewDraft) this.flushLocalDraft()
      this.disarmReturnGuard()
      setTimeout(() => uni.navigateBack(), 500)
    },
    armReturnGuard() {
      // #ifdef MP-WEIXIN
      // 用原生弹窗拦截左上角返回：不引入 page-container 覆盖层，避免其拦截页面滚动
      const w = typeof wx !== 'undefined' ? (wx as any) : null
      if (w && typeof w.enableAlertBeforeUnload === 'function') {
        w.enableAlertBeforeUnload({ message: '离开后未保存的修改将不会保留，确定离开吗？' })
      }
      // #endif
    },
    disarmReturnGuard() {
      // #ifdef MP-WEIXIN
      const w = typeof wx !== 'undefined' ? (wx as any) : null
      if (w && typeof w.disableAlertBeforeUnload === 'function') {
        w.disableAlertBeforeUnload()
      }
      // #endif
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
    /** 后端返回的草稿是否为“空白草稿” */
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
     * 返回去留决策。
     * - 新建未落库：内容已自动暂存本地，静默退出（符合“回来可续填”设计）。
     * - 已落库草稿且未改动：直接退出。
     * - 已落库草稿有改动：弹三按钮询问。
     */
    decideExit() {
      if (!this.loaded) {
        this.backPending = true
        return
      }
      if (this.showExitDialog) return
      if (this.isNewDraft) {
        this.doExit()
        return
      }
      if (this.hasSaved && !this.isDirty()) {
        this.doExit()
        return
      }
      this.showExitDialog = true
    },
    async onExitSave() {
      this.showExitDialog = false
      await this.saveDraft(true)
    },
    async onExitDiscard() {
      this.showExitDialog = false
      await this.deleteDraft(true)
    },
    onExitCancel() {
      this.showExitDialog = false
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
    /** 提交：授权 → 落库（必要时先建单） → 上传图片 → submit */
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
        const ok = await this.persistOrder(true)
        if (!ok) return
        await http.post(`/orders/${this.orderId}/submit`, {})
        uni.showToast({ title: '提交成功', icon: 'success' })
        this.exiting = true
        this.disarmReturnGuard()
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
  padding-bottom: 220rpx;
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

/* 本地草稿恢复提示条 */
.restore-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16rpx 24rpx;
  background-color: #fff7e6;
  color: #ad6800;
  font-size: 24rpx;

  .restore-action {
    color: #d4380d;
    padding-left: 24rpx;
  }
}

/* ① 图片 + 描述 + 常见问题 */
.upload-grid {
  display: flex;
  flex-wrap: wrap;
  margin-top: 8rpx;

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

  .upload-retry {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    padding: 8rpx 0;
    background-color: rgba(212, 56, 13, 0.8);
    color: #ffffff;
    font-size: 22rpx;
    text-align: center;
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

.desc-wrap {
  position: relative;
  margin-top: 8rpx;
}

.desc-textarea {
  width: 100%;
  min-height: 180rpx;
  padding: 20rpx;
  padding-bottom: 48rpx;
  box-sizing: border-box;
  background-color: #f7f8fa;
  border-radius: 16rpx;
  font-size: 28rpx;
  color: #333333;
  line-height: 1.6;
}

.desc-footer {
  position: absolute;
  left: 20rpx;
  right: 20rpx;
  bottom: 12rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.desc-clear-text {
  font-size: 24rpx;
  color: #e64340;
  line-height: 1;
}

.desc-count {
  margin-left: auto;
  font-size: 22rpx;
  color: #bbbbbb;
  line-height: 1;
}

.input-placeholder {
  color: #bbbbbb;
}

/* 常见问题 chips：限高内部滚动，避免撑爆页面导致整页无法滚动 */
.issue-chips {
  max-height: 220rpx;
  margin-top: 20rpx;

  .issue-chip {
    display: inline-block;
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

  .cell-clear {
    width: 44rpx;
    height: 44rpx;
    border-radius: 50%;
    background-color: #e6e6e6;
    color: #ffffff;
    font-size: 30rpx;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
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

/* 位置：手填 + 历史候选 */
.room-block {
  position: relative;
}

/* 聚焦时弹出的历史候选浮层（对齐输入框，覆盖下方内容） */
.room-suggest {
  position: absolute;
  left: 184rpx;
  right: 0;
  top: 88rpx;
  z-index: 30;
  background-color: #ffffff;
  border: 1rpx solid #ebedf0;
  border-radius: 12rpx;
  overflow: hidden;
}

.room-suggest-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 20rpx;
  border-bottom: 1rpx solid #f5f6f7;

  &:last-child {
    border-bottom: none;
  }
}

.room-suggest-text {
  flex: 1;
  font-size: 28rpx;
  color: #333333;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.room-suggest-del {
  flex-shrink: 0;
  width: 40rpx;
  text-align: center;
  font-size: 30rpx;
  color: #c8c9cc;
  margin-left: 12rpx;
}

/* 多单位切换提示 */
.ent-hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16rpx 0 8rpx;
  font-size: 24rpx;
  color: #8a8f99;

  .ent-hint-btn {
    color: #4a7cf7;
    flex-shrink: 0;
  }
}

/* 设备类型 / 问题类别 两级弹层 */
.cat-popup {
  display: flex;
  flex-direction: column;
  height: 100%;
  box-sizing: border-box;

  .cat-popup-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 28rpx 32rpx;
    font-size: 30rpx;
    color: #1a1a1a;
    border-bottom: 1rpx solid #f2f3f5;

    .cat-popup-cancel {
      color: #8a8f99;
      font-size: 28rpx;
    }

    .cat-popup-placeholder {
      width: 56rpx;
    }
  }

  .cat-popup-body {
    flex: 1;
    display: flex;
    min-height: 0;

    .cat-col {
      width: 45%;
      height: 100%;
      background-color: #f7f8fa;

      .cat-col-item {
        padding: 28rpx 32rpx;
        font-size: 28rpx;
        color: #333333;
        border-bottom: 1rpx solid #f2f3f5;

        &.is-active {
          background-color: #ffffff;
          color: #4a7cf7;
        }
      }

      .cat-empty {
        padding: 40rpx 32rpx;
        font-size: 26rpx;
        color: #bbbbbb;
      }
    }

    .cat-col-right {
      width: 55%;
      background-color: #ffffff;
    }
  }
}

.footer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
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
