<script lang="ts" setup>
import { ref } from 'vue'
import { RetryVerify } from '../../wailsjs/go/main/App'
import type { auth } from '../../wailsjs/go/models'
import PermissionBadge from '../components/PermissionBadge.vue'

const props = defineProps<{ state: auth.State }>()
const emit = defineEmits<{ (e: 'refresh'): void }>()

const retrying = ref(false)

async function retry() {
  retrying.value = true
  try {
    await RetryVerify()
  } catch (e) {
    console.error('retry failed', e)
  } finally {
    retrying.value = false
    emit('refresh')
  }
}

const permissionRows: { key: 'tunnel_edit' | 'zone_read' | 'dns_edit'; label: string }[] = [
  { key: 'tunnel_edit', label: 'Tunnel:Edit' },
  { key: 'zone_read', label: 'Zone:Read' },
  { key: 'dns_edit', label: 'DNS:Edit' },
]
</script>

<template>
  <div class="flex h-full flex-col">
    <header class="flex items-center justify-between border-b border-slate-200 bg-white px-6 py-4">
      <div class="flex items-center gap-2">
        <h1 class="text-lg font-bold text-slate-900">CFTunnelKit</h1>
        <span v-if="state.token_info?.account_name" class="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs text-slate-600">
          {{ state.token_info.account_name }}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <span
          v-for="row in permissionRows"
          :key="row.key"
          class="inline-flex items-center gap-1 text-xs"
        >
          <span class="text-slate-500">{{ row.label }}</span>
          <PermissionBadge :status="state.token_info?.permissions?.[row.key] ?? 'unverified'" />
        </span>
      </div>
    </header>

    <div
      v-if="state.offline"
      class="flex items-center justify-between border-b border-amber-200 bg-amber-50 px-6 py-2"
    >
      <p class="text-sm text-amber-700">
        离线，未验证 — {{ state.message || '无法连接 Cloudflare API' }}
      </p>
      <button
        :disabled="retrying"
        class="rounded-md bg-amber-600 px-3 py-1 text-xs font-semibold text-white hover:bg-amber-700 disabled:bg-amber-300"
        @click="retry"
      >
        {{ retrying ? '重试中…' : '重试' }}
      </button>
    </div>

    <main class="flex flex-1 items-center justify-center p-6">
      <div class="w-full max-w-lg rounded-2xl border border-dashed border-slate-300 bg-white p-10 text-center">
        <p class="text-sm font-medium text-slate-600">Tunnel 列表</p>
        <p class="mt-2 text-sm text-slate-400">
          账户 ID：{{ state.token_info?.account_id ?? '—' }}
          <br />
          列表功能将在切片 02 实现，敬请期待。
        </p>
      </div>
    </main>
  </div>
</template>
