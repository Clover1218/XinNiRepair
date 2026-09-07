<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import Sortable from 'sortablejs'
import { adminAPI } from '@/api/admin'
import type {
  DictionaryCategoryTree,
  DictionaryProblem,
  DictionaryProperty
} from '@/types'

type SortableInstance = InstanceType<typeof Sortable>

type Kind = 'category' | 'property' | 'problem'

const loading = ref(false)
const saving = ref(false)
const sorting = ref(false)

/** 整棵树（大类 → 属性，各属性下含常见问题） */
const tree = ref<DictionaryCategoryTree[]>([])
const selectedCategoryId = ref('')
const selectedPropertyId = ref('')

const fetchTree = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getCategories()
    tree.value = res.data.list
    // 选中项失效则回退：大类取第一个；属性取第一个有效项
    const catValid = selectedCategoryId.value &&
      tree.value.some(c => c.id === selectedCategoryId.value)
    if (!catValid) {
      selectedCategoryId.value = tree.value[0]?.id ?? ''
      selectedPropertyId.value = ''
    }
    if (selectedCategoryId.value) {
      const props = selectedCategory.value?.properties ?? []
      const propValid = selectedPropertyId.value &&
        props.some(p => p.id === selectedPropertyId.value)
      if (!propValid) selectedPropertyId.value = props[0]?.id ?? ''
    }
  } finally {
    loading.value = false
  }
}

const selectedCategory = computed(() =>
  tree.value.find(c => c.id === selectedCategoryId.value)
)

const propertyOptions = computed(() => selectedCategory.value?.properties ?? [])

const selectedProperty = computed(() =>
  propertyOptions.value.find(p => p.id === selectedPropertyId.value)
)

const selectCategory = (id: string) => {
  selectedCategoryId.value = id
  const props = tree.value.find(c => c.id === id)?.properties ?? []
  selectedPropertyId.value = props[0]?.id ?? ''
}

const selectProperty = (id: string) => {
  selectedPropertyId.value = id
}

/** 选中属性的常见问题 */
const currentProblems = computed<DictionaryProblem[]>(
  () => selectedProperty.value?.problems ?? []
)

// ── 拖拽排序（sortablejs；排序即保存，成功后不回跳数据） ──
const catEl = ref<HTMLElement | null>(null)
const propEl = ref<HTMLElement | null>(null)
const probEl = ref<HTMLElement | null>(null)
const sortables = new Map<string, SortableInstance>()

const moveItem = <T>(arr: T[], from: number, to: number) => {
  if (from === to || from < 0 || to < 0) return false
  const [it] = arr.splice(from, 1)
  arr.splice(to, 0, it)
  return true
}

const destroySortables = () => {
  sortables.forEach(s => s.destroy())
  sortables.clear()
}

const bindSortable = (
  key: string,
  el: HTMLElement | null,
  listGetter: () => Array<{ id: string; sort_order: number }>,
  persist: (items: Array<{ id: string; sort_order: number }>) => Promise<void>
) => {
  if (!el || sortables.has(key)) return
  const s = Sortable.create(el, {
    animation: 150,
    handle: '.drag-handle',
    ghostClass: 'drag-ghost',
    chosenClass: 'drag-chosen',
    onEnd: evt => {
      const items = listGetter()
      if (!moveItem(items, evt.oldIndex ?? -1, evt.newIndex ?? -1)) return
      void persist(items)
    }
  })
  sortables.set(key, s)
}

const rebind = () => {
  destroySortables()
  bindSortable('cat', catEl.value, () => tree.value, persistOrder('大类'))
  bindSortable(
    'prop',
    propEl.value,
    () => (selectedCategory.value?.properties ?? []) as any,
    persistOrder('属性')
  )
  bindSortable(
    'prob',
    probEl.value,
    () => (selectedProperty.value?.problems ?? []) as any,
    persistOrder('问题')
  )
}

/** 拖动结束后按新顺序写回 sort_order（index+1），变更项逐个 PUT */
function persistOrder(label: string) {
  return async (items: Array<{ id: string; sort_order: number }>) => {
    sorting.value = true
    try {
      const tasks: Promise<unknown>[] = []
      items.forEach((it, idx) => {
        const next = idx + 1
        if (it.sort_order !== next) {
          it.sort_order = next // 先本地生效，保证后续 diff 正确
          if (label === '大类')
            tasks.push(adminAPI.updateCategory(it.id, { sort_order: next }))
          else if (label === '属性')
            tasks.push(adminAPI.updateProperty(it.id, { sort_order: next }))
          else tasks.push(adminAPI.updateProblem(it.id, { sort_order: next }))
        }
      })
      if (tasks.length) {
        await Promise.all(tasks)
        ElMessage.success(`${label}顺序已保存`)
      }
    } catch {
      ElMessage.error(`${label}顺序保存失败，已恢复原顺序`)
      await fetchTree() // 回滚为服务端顺序
    } finally {
      sorting.value = false
    }
  }
}

// 数据(整树替换)或选中变化后重绑拖拽（避免深监听在拖动保存逐条更新时反复重建实例）
watch(
  [tree, selectedCategoryId, selectedPropertyId],
  async () => {
    await nextTick()
    rebind()
  }
)

// ── 新增 / 编辑（排序改为拖拽维护，弹窗不再提供排序输入） ──
const dialogVisible = ref(false)
const dialogKind = ref<Kind>('category')
const dialogTitle = computed(() => {
  const map = { category: '项目大类', property: '项目属性', problem: '常见问题' }
  return `${form.value.id ? '编辑' : '新增'}${map[dialogKind.value]}`
})

const form = ref<{
  id: string
  categoryId: string
  propertyId: string
  name: string
  description: string
  commonSolutions: string[]
}>({
  id: '',
  categoryId: '',
  propertyId: '',
  name: '',
  description: '',
  commonSolutions: []
})

const kindName = (k: Kind) =>
  k === 'category' ? '项目大类' : k === 'property' ? '项目属性' : '常见问题'

const nextSort = (items: Array<{ sort_order: number }>) =>
  items.length ? Math.max(...items.map(i => i.sort_order)) + 1 : 1

const openAdd = (kind: Kind) => {
  dialogKind.value = kind
  if (kind === 'problem' && !selectedPropertyId.value) {
    ElMessage.warning('请先在左侧选择某个项目属性，再添加该属性的常见问题')
    return
  }
  form.value = {
    id: '',
    categoryId: selectedCategoryId.value,
    propertyId: kind === 'problem' ? selectedPropertyId.value : selectedPropertyId.value,
    name: '',
    description: '',
    commonSolutions: []
  }
  dialogVisible.value = true
}

const openEditCategory = (row: DictionaryCategoryTree) => {
  dialogKind.value = 'category'
  form.value = {
    id: row.id,
    categoryId: row.id,
    propertyId: '',
    name: row.name,
    description: row.description,
    commonSolutions: []
  }
  dialogVisible.value = true
}

const openEditProperty = (row: DictionaryProperty) => {
  dialogKind.value = 'property'
  form.value = {
    id: row.id,
    categoryId: selectedCategoryId.value,
    propertyId: row.id,
    name: row.name,
    description: row.description,
    commonSolutions: []
  }
  dialogVisible.value = true
}

const openEditProblem = (row: DictionaryProblem) => {
  dialogKind.value = 'problem'
  form.value = {
    id: row.id,
    categoryId: selectedCategoryId.value,
    propertyId: row.property_id || selectedPropertyId.value,
    name: row.name,
    description: row.description,
    commonSolutions: [...(row.common_solutions ?? [])]
  }
  dialogVisible.value = true
}

const maxNameLen = computed(() => (dialogKind.value === 'problem' ? 100 : 50))

const handleSubmit = async () => {
  const name = form.value.name.trim()
  if (!name) {
    ElMessage.warning('请输入名称')
    return
  }
  if (name.length > maxNameLen.value) {
    ElMessage.warning(`名称不能超过 ${maxNameLen.value} 字`)
    return
  }
  if (form.value.description.length > 200) {
    ElMessage.warning('描述不能超过 200 字')
    return
  }
  const base = {
    name,
    description: form.value.description.trim() || undefined
  }
  saving.value = true
  try {
    if (dialogKind.value === 'category') {
      // 新大类排到末尾
      const payload = form.value.id
        ? base
        : { ...base, sort_order: nextSort(tree.value) }
      if (form.value.id) await adminAPI.updateCategory(form.value.id, payload as any)
      else await adminAPI.createCategory(payload as any)
    } else if (dialogKind.value === 'property') {
      const categoryId = form.value.categoryId || selectedCategoryId.value
      if (!categoryId) {
        ElMessage.warning('请先选择左侧项目大类')
        return
      }
      const list = tree.value.find(c => c.id === categoryId)?.properties ?? []
      const payload = {
        ...base,
        category_id: categoryId,
        ...(form.value.id ? {} : { sort_order: nextSort(list) })
      }
      if (form.value.id) await adminAPI.updateProperty(form.value.id, payload)
      else await adminAPI.createProperty(payload as any)
    } else {
      const propertyId = form.value.propertyId || selectedPropertyId.value
      if (!propertyId) {
        ElMessage.warning('请先选择左侧项目属性')
        return
      }
      const list = selectedProperty.value?.problems ?? []
      const payload = {
        ...base,
        property_id: propertyId,
        common_solutions: form.value.commonSolutions,
        ...(form.value.id ? {} : { sort_order: nextSort(list) })
      }
      if (form.value.id) await adminAPI.updateProblem(form.value.id, payload)
      else await adminAPI.createProblem(payload as any)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await fetchTree()
  } finally {
    saving.value = false
  }
}

// ── 删除（软删除；属性/大类级联其下问题） ──
const removeKind = async (kind: Kind, row: { id: string; name: string }) => {
  const confirmText =
    kind === 'category'
      ? `确定删除大类「${row.name}」？删除后其下属性及属性下常见问题将一并隐藏（软删除，历史工单不受影响）。`
      : kind === 'property'
        ? `确定删除属性「${row.name}」？其下常见问题将一并隐藏（软删除，历史工单不受影响）。`
        : `确定删除常见问题「${row.name}」？删除为软删除，不影响历史工单。`
  try {
    await ElMessageBox.confirm(confirmText, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  try {
    if (kind === 'category') await adminAPI.deleteCategory(row.id)
    else if (kind === 'property') await adminAPI.deleteProperty(row.id)
    else await adminAPI.deleteProblem(row.id)
    ElMessage.success('已删除')
    await fetchTree()
  } catch {
    /* 拦截器已提示 */
  }
}

onMounted(async () => {
  await fetchTree()
  await nextTick()
  rebind()
})

onBeforeUnmount(destroySortables)
</script>

<template>
  <div v-loading="loading || sorting" class="dict-page">
    <el-card shadow="never" class="dict-card">
      <!-- 三栏布局：大类 | 属性 | 常见问题，均可点击选中、拖动手柄排序 -->
      <div class="cols">
        <!-- 第 1 栏：项目大类 -->
        <section class="col cat-col">
          <div class="panel-head">
            <span class="panel-title">项目大类</span>
            <el-button type="primary" plain size="small" @click="openAdd('category')">
              <el-icon><Plus /></el-icon>
              新增
            </el-button>
          </div>
          <ul ref="catEl" class="drop-list">
            <li
              v-for="c in tree"
              :key="c.id"
              class="item"
              :class="{ active: c.id === selectedCategoryId }"
              @click="selectCategory(c.id)"
            >
              <div class="item-main">
                <div class="item-line">
                  <span class="item-name">{{ c.name }}</span>
                  <span class="item-count">{{ c.properties.length }} 属性</span>
                </div>
              </div>
              <div class="item-ops" @click.stop>
                <el-button link type="primary" size="small" @click="openEditCategory(c)">编辑</el-button>
                <el-button link type="danger" size="small" @click="removeKind('category', c)">删除</el-button>
                <el-icon class="drag-handle" title="拖动排序"><Rank /></el-icon>
              </div>
            </li>
          </ul>
          <el-empty v-if="!tree.length" description="暂无项目大类" :image-size="60" />
          <div class="drag-tip">按住 <el-icon><Rank /></el-icon> 拖动调整大类顺序</div>
        </section>

        <!-- 第 2 栏：项目属性（选中大类后展示，可点选、可拖） -->
        <section class="col prop-col">
          <div class="panel-head">
            <span class="panel-title">项目属性</span>
            <el-button
              type="primary"
              plain
              size="small"
              :disabled="!selectedCategoryId"
              @click="openAdd('property')"
            >
              <el-icon><Plus /></el-icon>
              新增
            </el-button>
          </div>
          <template v-if="selectedCategory">
            <ul ref="propEl" class="drop-list">
              <li
                v-for="p in propertyOptions"
                :key="p.id"
                class="item"
                :class="{ active: p.id === selectedPropertyId }"
                @click="selectProperty(p.id)"
              >
                <div class="item-main">
                  <div class="item-line">
                    <span class="item-name">{{ p.name }}</span>
                    <span class="item-count">{{ p.problems?.length ?? 0 }} 问题</span>
                  </div>
                  <div v-if="p.description" class="item-desc">{{ p.description }}</div>
                </div>
                <div class="item-ops" @click.stop>
                  <el-button link type="primary" size="small" @click="openEditProperty(p)">编辑</el-button>
                  <el-button link type="danger" size="small" @click="removeKind('property', p)">删除</el-button>
                  <el-icon class="drag-handle" title="拖动排序"><Rank /></el-icon>
                </div>
              </li>
            </ul>
            <el-empty v-if="!propertyOptions.length" description="暂无属性，请新增" :image-size="60" />
            <div class="drag-tip">按住 <el-icon><Rank /></el-icon> 拖动调整属性顺序</div>
          </template>
          <el-empty v-else description="请先选择左侧项目大类" :image-size="60" />
        </section>

        <!-- 第 3 栏：常见问题（选中属性后联动展示，可拖） -->
        <section class="col prob-col">
          <div class="panel-head">
            <span class="panel-title">常见问题</span>
            <el-button
              type="primary"
              plain
              size="small"
              :disabled="!selectedPropertyId"
              @click="openAdd('problem')"
            >
              <el-icon><Plus /></el-icon>
              新增
            </el-button>
          </div>
          <template v-if="selectedProperty">
            <div class="scope-tip">
              当前属性：
              <span class="scope-name">{{ selectedProperty.name }}</span>
              （问题直接隶属项目属性）
            </div>
            <ul ref="probEl" class="drop-list">
              <li
                v-for="q in currentProblems"
                :key="q.id"
                class="item prob-item"
              >
                <div class="item-main">
                  <div class="item-line">
                    <span class="item-name">{{ q.name }}</span>
                  </div>
                  <div v-if="q.description" class="item-desc">{{ q.description }}</div>
                  <div v-if="q.common_solutions?.length" class="item-tags">
                    <el-tag
                      v-for="tag in q.common_solutions"
                      :key="tag"
                      size="small"
                      type="info"
                      effect="plain"
                      class="tag"
                    >{{ tag }}</el-tag>
                  </div>
                </div>
                <div class="item-ops" @click.stop>
                  <el-button link type="primary" size="small" @click="openEditProblem(q)">编辑</el-button>
                  <el-button link type="danger" size="small" @click="removeKind('problem', q)">删除</el-button>
                  <el-icon class="drag-handle" title="拖动排序"><Rank /></el-icon>
                </div>
              </li>
            </ul>
            <el-empty v-if="!currentProblems.length" description="该属性下暂无常见问题" :image-size="60" />
            <div class="drag-tip">按住 <el-icon><Rank /></el-icon> 拖动调整问题顺序</div>
          </template>
          <el-empty v-else description="请先选择左侧某个项目属性" :image-size="60" />
        </section>
      </div>
    </el-card>

    <!-- 新增/编辑弹窗（排序不再手工录入，改为拖拽维护） -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px">
      <el-form label-position="top">
        <el-form-item :label="`${kindName(dialogKind)}名称（必填，≤${maxNameLen} 字）`">
          <el-input
            v-model="form.name"
            :maxlength="maxNameLen"
            show-word-limit
            placeholder="请输入名称"
          />
        </el-form-item>
        <el-form-item v-if="dialogKind === 'property'" label="所属项目大类">
          <el-select v-model="form.categoryId" class="full-width" disabled>
            <el-option
              v-for="c in tree"
              :key="c.id"
              :label="c.name"
              :value="c.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="dialogKind === 'problem'" label="所属项目属性">
          <el-select v-model="form.propertyId" class="full-width" filterable>
            <el-option
              v-for="p in propertyOptions"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="描述（≤200 字，选填）">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            :maxlength="200"
            show-word-limit
            placeholder="选填"
          />
        </el-form-item>
        <el-form-item v-if="dialogKind === 'problem'" label="常见解决建议（回车添加，可多条）">
          <el-select
            v-model="form.commonSolutions"
            multiple
            filterable
            allow-create
            default-first-option
            :reserve-keyword="false"
            placeholder="输入建议后回车添加"
            class="full-width"
          >
            <el-option
              v-for="tag in form.commonSolutions"
              :key="tag"
              :label="tag"
              :value="tag"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSubmit">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.dict-card {
  border-radius: 8px;
}

.cols {
  display: grid;
  grid-template-columns: 24% 30% 1fr;
  gap: 14px;
  align-items: start;
}

.col {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 12px 14px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  white-space: nowrap;
}

.drop-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 560px;
  overflow-y: auto;
  flex: 1;
}

.item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 8px 10px;
  cursor: pointer;
  transition: all 0.2s;
}

.item:hover {
  border-color: #409eff;
}

.item.active {
  border-color: #409eff;
  background-color: #ecf5ff;
}

.item-main {
  min-width: 0;
}

.item-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.item-name {
  font-weight: 600;
  color: #303133;
  word-break: break-all;
}

.item-count {
  font-size: 12px;
  color: #909399;
  white-space: nowrap;
  flex-shrink: 0;
}

.item-desc {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
  word-break: break-all;
}

.item-tags {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.item-ops {
  margin-top: 6px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

.drag-handle {
  margin-left: 6px;
  color: #b0b6be;
  cursor: grab;
  font-size: 16px;
}

.drag-handle:active {
  cursor: grabbing;
}

.drag-ghost {
  opacity: 0.4;
}

.drag-chosen {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.12);
}

.drag-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #c0c4cc;
  padding-top: 4px;
}

.scope-tip {
  font-size: 13px;
  color: #909399;
}

.scope-name {
  color: #409eff;
  font-weight: 600;
}

.prob-item .item-main {
  flex: 1;
}

.full-width {
  width: 100%;
}
</style>
