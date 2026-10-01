<script lang="ts" setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api'
import { friendlyError } from '@/utils/error'

const props = defineProps<{ current: string; latest: string; releaseUrl: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

function goDownload() {
  api.system.openRelease(props.releaseUrl)
  emit('close')
}

function escHandler(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => document.addEventListener('keydown', escHandler))
onUnmounted(() => document.removeEventListener('keydown', escHandler))
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 animate-fade-in" @click.self="emit('close')">
    <div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl animate-zoom-in">
      <h2 class="text-lg font-bold text-slate-900">发现新版本</h2>
      <p class="mt-2 text-sm text-slate-600">
        当前版本 <span class="font-mono">{{ current || 'dev' }}</span>
      </p>
      <p class="mt-1 text-sm text-slate-600">
        最新版本 <span class="font-mono font-semibold text-blue-600">{{ latest }}</span>
      </p>
      <div class="mt-5 flex justify-end gap-2">
        <button
          class="rounded-lg border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50 transition-colors"
          @click="emit('close')"
        >
          稍后
        </button>
        <button
          class="rounded-lg bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 transition-colors"
          @click="goDownload"
        >
          前往下载
        </button>
      </div>
    </div>
  </div>
</template>
