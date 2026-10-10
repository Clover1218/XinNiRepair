/**
 * 本地草稿存储（V1.4「先内容后分配 ID」改造）
 *
 * 新建工单时不在服务端创建任何记录，整张表单（含图片）先落本地；
 * 只有在用户显式「保存草稿 / 提交报修」时才向服务端 POST /orders 一次性建单。
 *
 * 约定：
 * - 每个用户只保留最近 1 份本地草稿（key: xn_localDraft_{userId}）
 * - 图片通过 uni.saveFile 持久化为本地文件，跨会话可用
 * - 草稿超过 LOCAL_DRAFT_TTL 天自动丢弃
 */
import { useUserStore } from '@/stores/user'

/** 草稿结构版本号：结构变更时递增，旧版本读取后直接丢弃 */
const DRAFT_VERSION = 1
/** 本地草稿有效期（天） */
const LOCAL_DRAFT_TTL = 7
/** 单草稿最多图片数（与后端 maxFaultImages=9 对齐） */
export const MAX_DRAFT_IMAGES = 9

/** 本地待上传图片项 */
export interface LocalDraftImage {
  /** 本地持久文件路径（已 upload 成功的项为原路径或图床地址） */
  path: string
  /** 可直接用于 image src 的预览路径 */
  preview: string
  name: string
  size: number
  /** 图床 URL（未上传为空） */
  url: string
  /** 是否已随服务端工单上传 */
  uploaded: boolean
}

/** 本地草稿表单内容 */
export interface LocalDraftForm {
  enterprise_id: string
  category_id: string
  property_id: string
  description: string
  urgency: string
  room: string
  contact_name: string
  contact_phone: string
}

/** 本地草稿 */
export interface LocalDraft {
  v: number
  localId: string
  updatedAt: number
  form: LocalDraftForm
  images: LocalDraftImage[]
  /** 已落库时回填；仅保存本地草稿时为 0 长度字符串 */
  orderId: string
}

const KEY_PREFIX = 'xn_localDraft_'

function draftKey(userId: string): string {
  return `${KEY_PREFIX}${userId}`
}

/** 生成草稿用的本地唯一 id（无 crypto 依赖，够用即可） */
export function localId(): string {
  return `ld_${Date.now()}_${Math.floor(Math.random() * 1e6)}`
}

/** 生成图片本地文件名 */
export function localImageName(index: number, ext = 'jpg'): string {
  return `img_${Date.now()}_${index}.${ext}`
}

/**
 * 读取本地草稿：不存在 / 版本不符 / 已过期 均返回 null（并顺手清理过期数据）。
 * @param userId 用户 id，缺省时从 user store 取
 */
export function readLocalDraft(userId?: string): LocalDraft | null {
  const uid = userId || useUserStore().userInfo?.id || ''
  if (!uid) return null
  const raw = uni.getStorageSync(draftKey(uid))
  if (!raw) return null
  let draft: LocalDraft
  try {
    draft = typeof raw === 'string' ? (JSON.parse(raw) as LocalDraft) : (raw as LocalDraft)
  } catch (e) {
    uni.removeStorageSync(draftKey(uid))
    return null
  }
  if (!draft || draft.v !== DRAFT_VERSION) {
    uni.removeStorageSync(draftKey(uid))
    return null
  }
  if (Date.now() - (draft.updatedAt || 0) > LOCAL_DRAFT_TTL * 24 * 60 * 60 * 1000) {
    uni.removeStorageSync(draftKey(uid))
    return null
  }
  return draft
}

/** 写入本地草稿（带 today 时间戳） */
export function writeLocalDraft(draft: LocalDraft, userId?: string): boolean {
  const uid = userId || useUserStore().userInfo?.id || ''
  if (!uid) return false
  try {
    draft.updatedAt = Date.now()
    uni.setStorageSync(draftKey(uid), JSON.stringify(draft))
    return true
  } catch (e) {
    // 配额不足等场景不阻塞编辑，由调用方提示降级
    return false
  }
}

/** 清除本地草稿，并尝试删除其占用的本地图片文件 */
export function clearLocalDraft(userId?: string): void {
  const uid = userId || useUserStore().userInfo?.id || ''
  if (!uid) return
  const draft = readLocalDraft(uid)
  if (draft && draft.images && draft.images.length) {
    draft.images.forEach((img) => {
      if (img.path && img.path.indexOf('http') !== 0) {
        removeSavedFile(img.path)
      }
    })
  }
  uni.removeStorageSync(draftKey(uid))
}

/** 删除本地持久文件（失败静默，属清理动作） */
export function removeSavedFile(path: string): void {
  if (!path || path.indexOf('http') === 0) return
  try {
    uni.getSavedFileList({
      success: (res) => {
        const hit = (res.fileList || []).find((f: { filePath: string }) => f.filePath === path)
        if (hit) uni.removeSavedFile({ filePath: path })
      }
    })
  } catch (e) {
    // 清理失败不影响主流程
  }
}

/**
 * 将临时图片持久化为本地文件，返回可直接渲染的路径。
 * 持久化失败时回退原临时路径（本次会话内仍可用）。
 */
export function persistImage(tempPath: string): Promise<string> {
  return new Promise((resolve) => {
    if (!tempPath || tempPath.indexOf('http') === 0) {
      resolve(tempPath)
      return
    }
    uni.saveFile({
      tempFilePath: tempPath,
      success: (res) => resolve(res.savedFilePath || tempPath),
      fail: () => resolve(tempPath)
    })
  })
}

/* ---------- 位置历史（最近填写的地点） ---------- */

/** 位置历史最多保留条数 */
export const ROOM_HISTORY_MAX = 8

/** 位置历史项 */
export interface RoomHistoryItem {
  room: string
  /** 最近一次使用时间戳（ms） */
  ts: number
}

function roomKey(userId: string, enterpriseId: string): string {
  return `xn_roomHist_${userId}_${enterpriseId}`
}

/**
 * 读取位置历史：按「用户 + 单位」维度，最近使用在前。
 * 只读不写，不存在 / 数据损坏均返回空数组。
 */
export function readRoomHistory(userId: string, enterpriseId: string): RoomHistoryItem[] {
  if (!userId || !enterpriseId) return []
  const raw = uni.getStorageSync(roomKey(userId, enterpriseId))
  if (!raw) return []
  let list: RoomHistoryItem[] = []
  try {
    const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    if (Array.isArray(parsed)) list = parsed
  } catch {
    return []
  }
  return list
    .filter((it) => it && typeof it.room === 'string' && it.room.trim())
    .map((it) => ({ room: it.room.trim(), ts: Number(it.ts) || 0 }))
    .sort((a, b) => b.ts - a.ts)
    .slice(0, ROOM_HISTORY_MAX)
}

/** 读取最近一次填写的地点（无历史时返回空串） */
export function readLastRoom(userId: string, enterpriseId: string): string {
  const list = readRoomHistory(userId, enterpriseId)
  return list.length ? list[0].room : ''
}

/**
 * 写入位置历史：已存在则提到队首并刷新时间，否则插入队首，超出上限丢弃最旧的。
 * 空值 / 缺 userId / 缺 enterpriseId 直接忽略。
 */
export function pushRoomHistory(userId: string, enterpriseId: string, room: string): void {
  if (!userId || !enterpriseId) return
  const v = (room || '').trim()
  if (!v) return
  const list = readRoomHistory(userId, enterpriseId).filter((it) => it.room !== v)
  list.unshift({ room: v, ts: Date.now() })
  uni.setStorageSync(roomKey(userId, enterpriseId), list.slice(0, ROOM_HISTORY_MAX))
}

/** 删除单条位置历史 */
export function removeRoomHistory(userId: string, enterpriseId: string, room: string): void {
  if (!userId || !enterpriseId) return
  const v = (room || '').trim()
  if (!v) return
  const list = readRoomHistory(userId, enterpriseId).filter((it) => it.room !== v)
  uni.setStorageSync(roomKey(userId, enterpriseId), list)
}

/** 清空某单位下的位置历史 */
export function clearRoomHistory(userId: string, enterpriseId: string): void {
  if (!userId || !enterpriseId) return
  uni.removeStorageSync(roomKey(userId, enterpriseId))
}
