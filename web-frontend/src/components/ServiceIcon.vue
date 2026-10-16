<script setup lang="ts">
/**
 * 线性服务图标（内联 SVG，来源见 src/order/serviceIcons.ts）。
 *
 * 与小程序端 pages/home/icons.ts 同源同形；区别在于 Web 可直接内联
 * `<svg>`（小程序只能用 data-uri 背景图）。
 * 颜色跟随 `currentColor`：首页服务块 = 深灰、左侧深色侧栏 = 白。
 */
import { computed } from 'vue'
import { serviceIconPaths } from '@/order/serviceIcons'

const props = withDefaults(defineProps<{ name: string; size?: number }>(), {
  size: 22
})

const paths = computed(() => serviceIconPaths(props.name))
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    aria-hidden="true"
    class="svc-svg"
  >
    <path
      v-for="(d, i) in paths"
      :key="i"
      :d="d"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>

<style scoped>
.svc-svg {
  display: block;
  flex-shrink: 0;
}
</style>
