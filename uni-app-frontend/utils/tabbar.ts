/**
 * 自定义 TabBar 选中态同步工具（配合 custom-tab-bar 原生组件）。
 *
 * 微信自定义 tabBar 下，uni-app 会【原样拷贝】根目录 custom-tab-bar/ 到小程序，
 * 因此该目录必须是原生 wxml/wxss/js/json，不能写 .vue、不能 import 业务 TS。
 * 标签集合为：0 首页 / 1 统计 / 2 我的；其中「统计」仅企业审核员与维修业务员可见，
 * 必须在【页面上下文】里用 auth 工具算好，通过 setData 注入原生组件的自定义字段
 * statsVisible；原生组件据此决定渲染 3 项还是 2 项。
 *
 * 注意：不要通过 setData 覆盖框架注入的 `list`（微信会按 pages.json 回写），
 * 故这里只传 selected + statsVisible 两个字段。
 * 框架通过 getCurrentPages()[last].getTabBar() 暴露组件实例。
 * 非 tab 页 / 未启用 custom tabBar 时静默忽略。
 *
 * ⚠️ V1.7：工单列表已从 tabBar 移出（改为普通页面，首页用 navigateTo 带参跳转），
 * 因此只有 home / stats / profile 三个页面需要调用本工具。
 */
import { isStoreStaff, isUnitReviewer } from '@/utils/auth'

/** 中间「统计」标签：维修业务员(role=1) 与单位审核员可见；普通用户隐藏 */
function statsVisible(): boolean {
  return isStoreStaff() || isUnitReviewer()
}

/**
 * 同步底栏选中态。
 * @param index 该页在完整 3 项列表中的索引：0 首页 / 1 统计 / 2 我的
 */
export function setTabBarSelected(index: number): void {
  try {
    const pages = getCurrentPages() as {
      getTabBar?: () => { setData: (o: Record<string, unknown>) => void }
    }[]
    const page = pages[pages.length - 1]
    const tabBar = page && typeof page.getTabBar === 'function' ? page.getTabBar() : null
    if (tabBar && typeof tabBar.setData === 'function') {
      tabBar.setData({ selected: index, statsVisible: statsVisible() })
    }
  } catch (e) {
    /* 非 tab 页或自定义 tabBar 未启用：忽略 */
  }
}
