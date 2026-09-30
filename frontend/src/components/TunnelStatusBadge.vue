<script lang="ts" setup>
// Maps Cloudflare tunnel status to a colored badge.
// healthy → green / degraded → amber / down → red / inactive → gray.
// Unknown or missing status falls back to gray and shows the raw string
// (or "未知" when the field is absent), per issue #4.
defineProps<{ status?: string }>()

const KNOWN: Record<string, { cls: string; label: string }> = {
  healthy: { cls: 'bg-green-100 text-green-700 ring-green-200', label: '正常' },
  degraded: { cls: 'bg-amber-100 text-amber-700 ring-amber-200', label: '部分异常' },
  down: { cls: 'bg-red-100 text-red-700 ring-red-200', label: '断开' },
  inactive: { cls: 'bg-slate-200 text-slate-600 ring-slate-300', label: '未启动' },
}
const GRAY = 'bg-slate-200 text-slate-600 ring-slate-300'
</script>

<template>
  <span
    class="inline-flex min-w-[4rem] items-center justify-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1"
    :class="KNOWN[status ?? '']?.cls ?? GRAY"
  >
    {{ status && KNOWN[status] ? KNOWN[status].label : status || '未知' }}
  </span>
</template>
