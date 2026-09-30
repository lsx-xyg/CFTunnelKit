<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { GetAuthState, ListTunnels, RetryVerify } from '../../wailsjs/go/main/App'
import type { auth, cloudflare } from '../../wailsjs/go/models'
import PermissionBadge from '../components/PermissionBadge.vue'
import TunnelStatusBadge from '../components/TunnelStatusBadge.vue'

const props = defineProps<{ state: auth.State }>()
const emit = defineEmits<{ (e: 'refresh'): void; (e: 'session-expired'): void }>()

const retrying = ref(false)
const loading = ref(false)
const tunnels = ref<cloudflare.Tunnel[]>([])
const listError = ref('')

// Maps backend error strings to the issue #4 copy for the inline error state.
function friendlyError(e: unknown): string {
  const s = String(e)
  if (s.includes('权限')) return '权限不足，请检查 Token 权限'
  return s
}

async function loadTunnels() {
  loading.value = true
  listError.value = ''
  try {
    tunnels.value = await ListTunnels()
  } catch (e) {
    // The backend clears the persisted config and resets the state on auth
    // failure; detecting it here lets App.vue jump to the auth page.
    const st = await GetAuthState()
    if (!st.authenticated) {
      emit('session-expired')
      return
    }
    listError.value = friendlyError(e)
  } finally {
    loading.value = false
  }
}

async function retry() {
  retrying.value = true
  try {
    await RetryVerify()
  } catch (e) {
    console.error('retry failed', e)
  } finally {
    retrying.value = false
    emit('refresh')
    await loadTunnels()
  }
}

function fmtTime(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString('zh-CN', { hour12: false })
}

onMounted(loadTunnels)

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

    <main class="flex-1 overflow-y-auto p-6">
      <!-- loading: skeleton rows -->
      <div v-if="loading" class="space-y-3" aria-busy="true">
        <div v-for="i in 4" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
      </div>

      <!-- list error: stay on page + retry -->
      <div v-else-if="listError" class="rounded-xl border border-red-200 bg-red-50 p-6 text-center">
        <p class="text-sm font-medium text-red-700">{{ listError }}</p>
        <button
          class="mt-3 rounded-md bg-red-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-red-700"
          @click="loadTunnels"
        >
          重试
        </button>
      </div>

      <!-- empty state: placeholder create button (slice 04) -->
      <div v-else-if="tunnels.length === 0" class="rounded-2xl border border-dashed border-slate-300 bg-white p-10 text-center">
        <p class="text-sm font-medium text-slate-600">此账户还没有 Tunnel</p>
        <button
          disabled
          title="创建功能将在切片 04 实现"
          class="mt-4 cursor-not-allowed rounded-md bg-slate-300 px-4 py-1.5 text-sm font-semibold text-white"
        >
          创建 Tunnel
        </button>
        <p class="mt-2 text-xs text-slate-400">创建功能将在切片 04 实现</p>
      </div>

      <!-- tunnel list -->
      <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white">
        <table class="min-w-full divide-y divide-slate-200 text-sm">
          <thead class="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th class="px-5 py-3 font-medium">名称</th>
              <th class="px-5 py-3 font-medium">状态</th>
              <th class="px-5 py-3 font-medium">Tunnel ID</th>
              <th class="px-5 py-3 font-medium">创建时间</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="t in tunnels" :key="t.id" class="hover:bg-slate-50">
              <td class="px-5 py-3 font-medium text-slate-900">{{ t.name }}</td>
              <td class="px-5 py-3">
                <TunnelStatusBadge :status="t.status" />
              </td>
              <td class="px-5 py-3 font-mono text-xs text-slate-500">{{ t.id }}</td>
              <td class="px-5 py-3 text-slate-500">{{ fmtTime(t.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>
