<script lang="ts" setup>
const props = defineProps<{ status: string }>()

const styles: Record<string, string> = {
  ok: 'bg-green-100 text-green-700',
  missing: 'bg-red-100 text-red-700',
  unverified: 'bg-amber-100 text-amber-700',
}

const labels: Record<string, string> = {
  ok: '可用',
  missing: '缺失',
  unverified: '待验证',
}

const status = props.status || 'unverified'
</script>

<template>
  <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="styles[status] ?? styles.unverified">
    {{ labels[status] ?? labels.unverified }}
  </span>
</template>
