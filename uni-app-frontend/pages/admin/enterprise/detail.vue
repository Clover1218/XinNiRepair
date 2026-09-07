<template>
  <view class="page">
    <!-- 企业信息 -->
    <view v-if="ent" class="info-card">
      <view class="info-header">
        <text class="info-name">{{ ent.name }}</text>
        <wd-tag v-if="ent.status" :type="enterpriseStatusTagType(ent.status)" round>
          {{ enterpriseStatusLabel(ent.status) }}
        </wd-tag>
        <wd-button size="small" plain round @click="editName">改名</wd-button>
      </view>
      <view class="info-stats">
        <view v-if="ent.member_count !== undefined" class="stat">
          <text class="stat-num">{{ ent.member_count }}</text>
          <text class="stat-label">成员</text>
        </view>
        <view v-if="ent.order_count !== undefined" class="stat">
          <text class="stat-num">{{ ent.order_count }}</text>
          <text class="stat-label">工单</text>
        </view>
        <view v-if="!isStoreStaff && ent.my_role" class="stat-role">
          我的身份：{{ roleLabel(ent.my_role) }}
        </view>
      </view>
      <!-- 免审核开关 -->
      <view class="auto-approve-row">
        <view class="auto-text">
          <text class="auto-title">免审核加入</text>
          <text class="auto-sub">开启后新成员扫码/邀请码加入无需审核</text>
        </view>
        <switch
          :checked="ent.auto_approve === true"
          color="#4d80f0"
          style="transform: scale(0.8);"
          @change="onAutoApproveChange"
        />
      </view>
      <view class="info-time">创建于 {{ formatDate(ent.created_at) }}</view>
    </view>

    <!-- 邀请码 -->
    <view v-if="ent" class="code-card">
      <view class="code-label">邀请码</view>
      <view class="code-row">
        <text class="code-value">{{ ent.invite_code || '--' }}</text>
        <view class="code-actions">
          <wd-button size="small" plain round @click="showQrcode">二维码</wd-button>
          <wd-button size="small" plain round @click="refreshCode">刷新</wd-button>
        </view>
      </view>
      <view class="code-expire">
        {{ ent.invite_code_expires_at ? `有效期至 ${formatDateTime(ent.invite_code_expires_at)}` : '永久有效' }}
      </view>
      <view class="code-tip">点击「二维码」生成邀请二维码，扫码即可加入本企业</view>
    </view>

    <!-- 邀请码二维码 canvas（移出屏幕，仅用于生成图片） -->
    <canvas
      class="qr-canvas"
      canvas-id="inviteQrcode"
      id="inviteQrcode"
      style="width: 200px; height: 200px;"
    ></canvas>

    <!-- 邀请码二维码浮窗 -->
    <view v-if="showQrcodePopup" class="qr-popup" @click="showQrcodePopup = false">
      <view class="qr-popup-card" @click.stop>
        <view class="qr-popup-close" @click="showQrcodePopup = false">×</view>
        <view class="qr-popup-title">邀请二维码</view>
        <view class="qr-popup-img-wrap">
          <image v-if="qrcodeImg" class="qr-popup-img" :src="qrcodeImg" mode="aspectFit"></image>
          <view v-else class="qr-popup-img loading">生成中...</view>
        </view>
        <view class="qr-popup-name">{{ ent?.name || '' }}</view>
        <view class="qr-popup-code">邀请码：{{ ent?.invite_code || '--' }}</view>
        <view class="qr-popup-tip">微信扫码即可加入本企业</view>
      </view>
    </view>

    <!-- 成员列表 -->
    <view class="member-section">
      <view class="member-title">成员列表</view>
      <view class="tabs-wrap">
        <wd-tabs v-model="activeTab" @change="onTabChange">
          <wd-tab v-for="tab in tabs" :key="tab.value" :title="tab.label" :name="tab.value"></wd-tab>
        </wd-tabs>
      </view>

      <view class="member-list">
        <view v-for="m in members" :key="m.membership_id" class="member-card">
          <view class="member-top">
            <image
              v-if="m.avatar_url"
              class="member-avatar"
              :src="m.avatar_url"
              mode="aspectFill"
            ></image>
            <view v-else class="member-avatar placeholder">{{ (m.nickname || '?').slice(0, 1) }}</view>
            <view class="member-info">
              <view class="member-name-row">
                <text class="member-name">{{ m.nickname }}</text>
                <text class="member-role-chip" :class="m.role === 'reviewer' || m.role === 'admin' ? 'reviewer' : 'member'">
                  {{ m.role === 'reviewer' || m.role === 'admin' ? '单位审核员' : '普通成员' }}
                </text>
                <wd-tag :type="memberStatusTagType(m.status)" round>{{ m.status_label }}</wd-tag>
              </view>
              <view class="member-meta">
                <text>{{ m.phone ? maskPhone(m.phone) : '未绑定手机号' }}</text>
                <text class="dot">·</text>
                <text>工单 {{ m.order_count }}</text>
                <text v-if="m.joined_at" class="dot">·</text>
                <text v-if="m.joined_at">{{ formatDate(m.joined_at) }} 加入</text>
              </view>
            </view>
          </view>

          <view class="member-actions">
            <template v-if="m.status === 'pending'">
              <wd-button size="small" type="success" plain round @click="approveMember(m)">通过</wd-button>
              <wd-button size="small" type="danger" plain round @click="rejectMember(m)">拒绝</wd-button>
            </template>
            <template v-else-if="m.status === 'approved'">
              <!-- 审核员身份任免：仅店方（审核员不显示该按钮） -->
              <template v-if="isStoreStaff && m.user_id !== currentUserId">
                <wd-button
                  v-if="m.role === 'reviewer' || m.role === 'admin'"
                  size="small"
                  plain
                  round
                  @click="setMemberRole(m, 'member')"
                >取消审核员</wd-button>
                <wd-button v-else size="small" plain round @click="setMemberRole(m, 'reviewer')">
                  设为单位审核员
                </wd-button>
              </template>
              <wd-button v-if="m.user_id !== currentUserId" size="small" plain round @click="removeMember(m)">
                移除
              </wd-button>
            </template>
          </view>
        </view>

        <view v-if="!loading && members.length === 0" class="empty">
          <view class="empty-text">暂无成员</view>
        </view>

        <view v-if="members.length > 0" class="load-more">
          <text>{{ loading ? '加载中...' : finished ? '没有更多了' : '上拉加载更多' }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { defineComponent } from 'vue'
import { http } from '@/utils/request'
import { isStoreStaff, isReviewerOf } from '@/utils/auth'
import { useUserStore } from '@/stores/user'
import drawQrcode from 'weapp-qrcode'
import {
  formatDate,
  formatDateTime,
  maskPhone,
  roleLabel,
  enterpriseStatusLabel,
  enterpriseStatusTagType,
  memberStatusTagType
} from '@/utils/format'
import type { EnterpriseDetail, MemberItem } from '@/types'

export default defineComponent({
  setup() {
    return {
      userStore: useUserStore(),
      formatDate,
      formatDateTime,
      maskPhone,
      roleLabel,
      enterpriseStatusLabel,
      enterpriseStatusTagType,
      memberStatusTagType
    }
  },
  data() {
    return {
      enterpriseId: '',
      ent: null as EnterpriseDetail | null,
      tabs: [
        { label: '全部', value: '' },
        { label: '已通过', value: 'approved' },
        { label: '待审核', value: 'pending' },
        { label: '已移除', value: 'removed' }
      ],
      activeTab: '',
      statusParam: '',
      members: [] as MemberItem[],
      page: 1,
      pageSize: 20,
      totalPages: 1,
      loading: false,
      finished: false,
      refreshing: false,
      showQrcodePopup: false,
      qrcodeImg: ''
    }
  },
  computed: {
    isStoreStaff(): boolean {
      return isStoreStaff()
    },
    currentUserId(): string {
      return this.userStore.userInfo?.id || ''
    }
  },
  onLoad(options: Record<string, string>) {
    this.enterpriseId = options.id || ''
  },
  async onShow() {
    if (!this.enterpriseId) {
      uni.showToast({ title: '参数错误', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 800)
      return
    }
    // 确保用户信息已加载（审核员身份判定依赖 userInfo.enterprises[].role）
    if (!this.userStore.userInfo) {
      try {
        await this.userStore.fetchUserInfo()
      } catch (e) {
        console.error('获取用户信息失败', e)
      }
    }
    // 守卫：店方任意单位 或 担任审核员的单位（审核员受限）
    if (!isStoreStaff() && !isReviewerOf(this.enterpriseId)) {
      uni.showToast({ title: '无权限查看该单位', icon: 'none' })
      setTimeout(() => uni.navigateBack(), 600)
      return
    }
    this.loadEnterprise()
    this.loadMembers(true)
  },
  onPullDownRefresh() {
    Promise.all([this.loadEnterprise(), this.loadMembers(true)]).finally(() =>
      uni.stopPullDownRefresh()
    )
  },
  onReachBottom() {
    if (!this.finished && !this.loading) this.loadMembers(false)
  },
  methods: {
    async loadEnterprise() {
      try {
        // 店方走管理端详情（含 status/order_count）；审核员走成员端详情（含 my_role）
        if (isStoreStaff()) {
          this.ent = await http.get<EnterpriseDetail>(`/admin/enterprises/${this.enterpriseId}`)
        } else {
          this.ent = await http.get<EnterpriseDetail>(`/enterprises/${this.enterpriseId}`)
        }
      } catch (e) {
        console.error('加载企业详情失败', e)
      }
    },
    async loadMembers(reset = false) {
      if (this.loading) return
      this.loading = true
      try {
        const targetPage = reset ? 1 : this.page + 1
        const params: Record<string, unknown> = {
          page: targetPage,
          page_size: this.pageSize
        }
        if (this.statusParam) params.status = this.statusParam
        const data = await http.get<any>(`/enterprises/${this.enterpriseId}/members`, params)
        const list = (data?.list || []) as MemberItem[]
        const total = Number(data?.total ?? 0) || 0
        const totalPages = Number(data?.total_pages ?? 1) || 1
        this.page = Number(data?.page ?? targetPage) || targetPage
        this.totalPages = totalPages
        this.finished = this.page >= totalPages || total === 0
        this.members = reset ? list : [...this.members, ...list]
      } catch (e) {
        console.error('加载成员列表失败', e)
      } finally {
        this.loading = false
      }
    },
    onTabChange(e: { name: string }) {
      this.activeTab = e.name
      this.statusParam = e.name
      this.loadMembers(true)
    },
    /** 企业改名（PUT /enterprises/:id，名称 2-50，不可重名 4522 已由后端 Toast） */
    editName() {
      const that = this
      uni.showModal({
        title: '修改企业名称',
        content: this.ent?.name || '',
        editable: true,
        placeholderText: '请输入企业名称（2-50字符）',
        success: async (res) => {
          if (!res.confirm) return
          const name = ((res.content || '') as string).trim()
          if (name.length < 2 || name.length > 50) {
            uni.showToast({ title: '企业名称需为2-50字符', icon: 'none' })
            return
          }
          if (name === that.ent?.name) return
          try {
            const data = await http.put<EnterpriseDetail>(`/enterprises/${that.enterpriseId}`, {
              name
            })
            that.ent = data || that.ent
            if (that.ent) that.ent.name = name
            uni.showToast({ title: '已保存', icon: 'success' })
          } catch (e) {
            console.error('修改名称失败', e)
          }
        }
      })
    },
    /** 免审核开关（PUT /enterprises/:id 局部更新 auto_approve） */
    async onAutoApproveChange(e: any) {
      const val = !!(e.detail && e.detail.value)
      try {
        const data = await http.put<EnterpriseDetail>(`/enterprises/${this.enterpriseId}`, {
          auto_approve: val
        })
        if (this.ent) this.ent.auto_approve = val
        uni.showToast({ title: val ? '已开启免审核' : '已关闭免审核', icon: 'success' })
      } catch (err) {
        console.error('更新免审核失败', err)
      }
    },
    /** 生成邀请码二维码并在浮窗中展示（内容：/enterprises/join?code=邀请码） */
    showQrcode() {
      const code = this.ent?.invite_code
      if (!code) {
        uni.showToast({ title: '暂无邀请码', icon: 'none' })
        return
      }
      this.qrcodeImg = ''
      this.showQrcodePopup = true
      try {
        drawQrcode({
          width: 200,
          height: 200,
          canvasId: 'inviteQrcode',
          text: `/enterprises/join?code=${code}`,
          callback: () => {
            uni.canvasToTempFilePath({
              canvasId: 'inviteQrcode',
              success: (res) => {
                this.qrcodeImg = res.tempFilePath
              },
              fail: (err) => {
                console.error('导出二维码失败', err)
                this.showQrcodePopup = false
                uni.showToast({ title: '二维码生成失败', icon: 'none' })
              }
            })
          }
        })
      } catch (e) {
        console.error('生成二维码失败', e)
        this.showQrcodePopup = false
        uni.showToast({ title: '二维码生成失败', icon: 'none' })
      }
    },
    /** 刷新邀请码（默认永久有效） */
    async refreshCode() {
      const res = await uni.showModal({
        title: '刷新邀请码',
        content: '确定重新生成邀请码？原邀请码将失效。',
        confirmText: '确认刷新'
      })
      if (!res.confirm) return
      if (this.refreshing) return
      this.refreshing = true
      try {
        const data = await http.post<{ invite_code: string; expires_at: string | null }>(
          `/enterprises/${this.enterpriseId}/refresh/code`,
          { validity: 'permanent' }
        )
        if (this.ent) {
          this.ent.invite_code = data.invite_code
          this.ent.invite_code_expires_at = data.expires_at
        }
        uni.showToast({ title: '邀请码已刷新', icon: 'success' })
      } catch (e) {
        console.error('刷新邀请码失败', e)
      } finally {
        this.refreshing = false
      }
    },
    async approveMember(m: MemberItem) {
      const res = await uni.showModal({
        title: '审核通过',
        content: `确定通过「${m.nickname}」的加入申请？`
      })
      if (!res.confirm) return
      try {
        await http.put(`/enterprises/${this.enterpriseId}/members/approve`, {
          user_ids: [m.user_id]
        })
        uni.showToast({ title: '已通过', icon: 'success' })
        this.loadMembers(true)
        this.loadEnterprise()
      } catch (e) {
        console.error('审核失败', e)
      }
    },
    async rejectMember(m: MemberItem) {
      const res = await uni.showModal({
        title: '拒绝申请',
        content: `确定拒绝「${m.nickname}」的加入申请？`,
        confirmText: '拒绝',
        confirmColor: '#fa5151'
      })
      if (!res.confirm) return
      try {
        await http.put(`/enterprises/${this.enterpriseId}/members/reject`, {
          user_ids: [m.user_id]
        })
        uni.showToast({ title: '已拒绝', icon: 'success' })
        this.loadMembers(true)
        this.loadEnterprise()
      } catch (e) {
        console.error('拒绝失败', e)
      }
    },
    async removeMember(m: MemberItem) {
      const res = await uni.showModal({
        title: '移除成员',
        content: `确定移除成员「${m.nickname}」？其历史报修记录将保留。`,
        confirmText: '移除',
        confirmColor: '#fa5151'
      })
      if (!res.confirm) return
      try {
        await http.delete(`/enterprises/${this.enterpriseId}/members/${m.user_id}`)
        uni.showToast({ title: '已移除', icon: 'success' })
        this.loadMembers(true)
        this.loadEnterprise()
      } catch (e) {
        console.error('移除成员失败', e)
      }
    },
    /** 任免单位审核员（仅店方；PUT /enterprises/:id/members/:uid/role） */
    setMemberRole(m: MemberItem, role: 'reviewer' | 'member') {
      const that = this
      const isSet = role === 'reviewer'
      uni.showModal({
        title: isSet ? '设为单位审核员' : '取消审核员',
        content: isSet
          ? `确定将「${m.nickname}」设为单位审核员？其将可审核本单位工单、审批成员加入。`
          : `确定取消「${m.nickname}」的单位审核员身份？其将回到普通成员。`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            await http.put(`/enterprises/${that.enterpriseId}/members/${m.user_id}/role`, {
              role
            })
            uni.showToast({ title: '已更新', icon: 'success' })
            that.loadMembers(true)
            // 若目标为微信账号，提示超管可为其设置后台密码
            if (isSet) {
              setTimeout(() => {
                uni.showToast({
                  title: '可提醒超管在 Web【用户管理】为其设置密码以登录后台',
                  icon: 'none',
                  duration: 3000
                })
              }, 600)
            }
          } catch (e) {
            console.error('更新成员身份失败', e)
          }
        }
      })
    }
  }
})
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
}

.info-card,
.code-card {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 28rpx;
  margin-bottom: 20rpx;

  .info-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    .info-name {
      flex: 1;
      min-width: 0;
      font-size: 34rpx;
      font-weight: 600;
      color: #1a1a1a;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    wd-button {
      margin-left: 16rpx;
      flex-shrink: 0;
    }
  }

  .info-stats {
    display: flex;
    align-items: baseline;
    margin-top: 24rpx;

    .stat {
      display: flex;
      align-items: baseline;
      margin-right: 40rpx;

      .stat-num {
        font-size: 40rpx;
        font-weight: 700;
        color: #4d80f0;
      }

      .stat-label {
        margin-left: 8rpx;
        font-size: 24rpx;
        color: #999999;
      }
    }

    .stat-role {
      font-size: 24rpx;
      color: #999999;
    }
  }

  .auto-approve-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 24rpx;
    padding-top: 24rpx;
    border-top: 1rpx solid #f2f3f5;

    .auto-text {
      .auto-title {
        display: block;
        font-size: 28rpx;
        color: #333333;
      }

      .auto-sub {
        display: block;
        margin-top: 6rpx;
        font-size: 22rpx;
        color: #999999;
      }
    }
  }

  .info-time {
    margin-top: 16rpx;
    font-size: 24rpx;
    color: #bbbbbb;
  }
}

.code-card {
  .code-label {
    font-size: 26rpx;
    color: #999999;
  }

  .code-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 16rpx;

    .code-value {
      font-size: 44rpx;
      font-weight: 700;
      letter-spacing: 6rpx;
      color: #4d80f0;
    }

    .code-actions {
      display: flex;
      align-items: center;

      wd-button {
        margin-left: 16rpx;
      }
    }
  }

  .code-expire {
    margin-top: 12rpx;
    font-size: 24rpx;
    color: #bbbbbb;
  }

  .code-tip {
    margin-top: 8rpx;
    font-size: 24rpx;
    color: #aaaaaa;
  }
}

/* 二维码 canvas：移出屏幕，仅用于生成图片（不可 display:none） */
.qr-canvas {
  position: fixed;
  top: 0;
  left: 9999px;
}

/* 二维码浮窗 */
.qr-popup {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;

  .qr-popup-card {
    position: relative;
    width: 560rpx;
    background-color: #ffffff;
    border-radius: 24rpx;
    padding: 48rpx 40rpx 40rpx;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;

    .qr-popup-close {
      position: absolute;
      top: 20rpx;
      right: 24rpx;
      width: 48rpx;
      height: 48rpx;
      line-height: 44rpx;
      text-align: center;
      font-size: 44rpx;
      color: #999999;
    }

    .qr-popup-title {
      font-size: 34rpx;
      font-weight: 600;
      color: #1a1a1a;
    }

    .qr-popup-img-wrap {
      margin-top: 32rpx;
      padding: 20rpx;
      border: 2rpx solid #f0f1f3;
      border-radius: 20rpx;

      .qr-popup-img {
        width: 400rpx;
        height: 400rpx;

        &.loading {
          display: flex;
          align-items: center;
          justify-content: center;
          color: #999999;
          font-size: 26rpx;
        }
      }
    }

    .qr-popup-name {
      margin-top: 24rpx;
      font-size: 30rpx;
      font-weight: 600;
      color: #333333;
    }

    .qr-popup-code {
      margin-top: 12rpx;
      font-size: 28rpx;
      color: #4d80f0;
      letter-spacing: 2rpx;
    }

    .qr-popup-tip {
      margin-top: 16rpx;
      font-size: 24rpx;
      color: #aaaaaa;
    }
  }
}

.member-section {
  .member-title {
    margin: 24rpx 8rpx 16rpx;
    font-size: 30rpx;
    font-weight: 600;
    color: #333333;
  }
}

.member-list {
  .member-card {
    background-color: #ffffff;
    border-radius: 20rpx;
    padding: 24rpx 28rpx;
    margin-bottom: 20rpx;

    .member-top {
      display: flex;
      align-items: center;

      .member-avatar {
        width: 80rpx;
        height: 80rpx;
        border-radius: 50%;
        background-color: #f0f2f5;
        flex-shrink: 0;

        &.placeholder {
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 32rpx;
          color: #999999;
        }
      }

      .member-info {
        flex: 1;
        min-width: 0;
        margin-left: 20rpx;

        .member-name-row {
          display: flex;
          align-items: center;

          .member-name {
            font-size: 30rpx;
            font-weight: 600;
            color: #1a1a1a;
            max-width: 240rpx;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          }

          .member-role-chip {
            margin: 0 12rpx;
            font-size: 20rpx;
            padding: 2rpx 10rpx;
            border-radius: 999rpx;

            &.reviewer {
              color: #b26a00;
              background-color: rgba(255, 193, 7, 0.16);
            }

            &.member {
              color: #666666;
              background-color: #f0f2f5;
            }
          }
        }

        .member-meta {
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          margin-top: 12rpx;
          font-size: 24rpx;
          color: #999999;

          .dot {
            margin: 0 10rpx;
            color: #dddddd;
          }
        }
      }
    }

    .member-actions {
      display: flex;
      justify-content: flex-end;
      margin-top: 20rpx;

      wd-button {
        margin-left: 16rpx;
      }
    }
  }
}

.empty {
  background-color: #ffffff;
  border-radius: 20rpx;
  padding: 60rpx 0;
  text-align: center;

  .empty-text {
    font-size: 28rpx;
    color: #999999;
  }
}

.load-more {
  padding: 16rpx 0 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: #bbbbbb;
}
</style>
