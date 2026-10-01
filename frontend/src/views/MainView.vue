<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { GetAuthState, GetRunStates, ListTunnels, OpenLogDir, RetryVerify, StartTunnel, StopTunnel, WriteOpLog } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime/runtime'
import type { auth, cloudflare } from '../../wailsjs/go/models'
import PermissionBadge from '../components/PermissionBadge.vue'
import TunnelStatusBadge from '../components/TunnelStatusBadge.vue'
import CreateTunnelDialog from '../components/CreateTunnelDialog.vue'
import TunnelDetailDialog from '../components/TunnelDetailDialog.vue'
import IngressEditorView from './IngressEditorView.vue'

interface LogPayload {
  timestamp: number
  stream: 'stdout' | 'stderr'
  level: 'INFO' | 'WARN' | 'ERROR' | 'DEBUG'
  line: string
  tunnel_id: string
}
interface StatusPayload {
  tunnel_id: string
  state: 'running' | 'stopped' | 'error'
  exit_code?: number
}
interface DownloadPayload {
  phase: 'downloading' | 'extracting' | 'done'
  downloaded: number
  total: number
}

const props = defineProps<{ state: auth.State }>()
const emit = defineEmits<{ (e: 'refresh'): void; (e: 'session-expired'): void; (e: 'toggle-log'): void }>()

const retrying = ref(false)
const loading = ref(false)
const pollInterval = ref(15)
const tunnels = ref<cloudflare.Tunnel[]>([])
const listError = ref('')
const moreOpen = ref(false)

// --- slice 03: process management state ---
const runStates = ref<Record<string, string>>({})
const busy = ref<Record<string, boolean>>({})
const download = ref<{ active: boolean; phase: string; downloaded: number; total: number }>({
  active: false,
  phase: '',
  downloaded: 0,
  total: 0,
})
const toast = ref<{ msg: string; type: 'success' | 'error' | 'info' } | null>(null)
let toastTimer: number | undefined

// --- slice 04: create / detail dialogs ---
const showCreate = ref(false)
const detailTunnelId = ref<string | null>(null)

// --- slice 05: ingress editor (full-screen overlay) ---
const ingressTunnelId = ref<string | null>(null)
const ingressTunnelName = ref('')

// --- UI polish: log panel fullscreen + operation log ---
const logFullscreen = ref(false)
const logTab = ref<'runtime' | 'ops'>('runtime')
interface OpLog {
  timestamp: number
  action: string
  result: 'ok' | 'fail'
}
const opLogs = ref<OpLog[]>([])

function pushOp(action: string, result: 'ok' | 'fail') {
  opLogs.value.push({ timestamp: Date.now(), action, result })
  if (opLogs.value.length > 500) opLogs.value.splice(0, opLogs.value.length - 500)
  WriteOpLog(action, result)
  EventsEmit('app:op', { action, result })
}

function openIngress(t: cloudflare.Tunnel) {
  ingressTunnelId.value = t.id
  ingressTunnelName.value = t.name
}

function onCreated() {
  showCreate.value = false
  showToast('Tunnel 创建成功', 'success')
  pushOp('创建 Tunnel', 'ok')
  loadTunnels()
}

function onDeleted() {
  detailTunnelId.value = null
  showToast('Tunnel 已删除', 'success')
  pushOp('删除 Tunnel', 'ok')
  loadTunnels()
}

// Log panel: global ring buffer capped at 5000 lines (issue #5), oldest
// dropped first.
const logLines = ref<LogPayload[]>([])
const logFilter = ref('')
const logSearch = ref('')
const logLevelFilter = ref('')
const logBox = ref<HTMLElement | null>(null)

const runningTunnels = computed(() => tunnels.value.filter((t) => runStates.value[t.id] === 'running'))

const runningCount = computed(() => tunnels.value.filter((t) => runStates.value[t.id] === 'running').length)
const stoppedCount = computed(() => tunnels.value.filter((t) => runStates.value[t.id] !== 'running').length)
const errorCount = computed(() => tunnels.value.filter((t) => runStates.value[t.id] === 'error').length)

const visibleLogs = computed(() => {
  let out = logLines.value
  if (logFilter.value) out = out.filter((l) => l.tunnel_id === logFilter.value)
  if (logLevelFilter.value) out = out.filter((l) => l.level === logLevelFilter.value)
  if (logSearch.value) {
    const q = logSearch.value.toLowerCase()
    out = out.filter((l) => l.line.toLowerCase().includes(q))
  }
  return out
})

function pushLog(p: LogPayload) {
  logLines.value.push(p)
  if (logLines.value.length > 5000) {
    logLines.value.splice(0, logLines.value.length - 5000)
  }
}

function showToast(msg: string, type: 'success' | 'error' | 'info' = 'info') {
  toast.value = { msg, type }
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = null), 3500)
}

// Maps backend error strings to the issue copy for the inline error state.
function friendlyError(e: unknown): string {
  const s = String(e)
  if (s.includes('权限')) return '权限不足，请检查 Token 权限'
  return s
}

async function loadTunnels(silent = false) {
  if (!silent) {
    loading.value = true
    listError.value = ''
  }
  try {
    tunnels.value = await ListTunnels()
  } catch (e) {
    if (!silent) {
      const st = await GetAuthState()
      if (!st.authenticated) {
        emit('session-expired')
        return
      }
      listError.value = friendlyError(e)
    }
  } finally {
    if (!silent) {
      await new Promise((r) => setTimeout(r, 400))
      loading.value = false
    }
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

// --- slice 03: start / stop ---
async function startTunnel(t: cloudflare.Tunnel) {
  busy.value[t.id] = true
  try {
    await StartTunnel(t.id)
    runStates.value[t.id] = 'running'
    const row = tunnels.value.find((x) => x.id === t.id)
    if (row) row.status = 'healthy'
    showToast(`已启动 ${t.name}`, 'success')
    pushOp(`启动隧道 ${t.name}（${t.id.slice(0,8)}）`, 'ok')
  } catch (e) {
    const st = await GetAuthState()
    if (!st.authenticated) { emit('session-expired'); return }
    const msg = friendlyError(e)
    showToast(msg, 'error')
    pushOp(`启动隧道 ${t.name} 失败：${msg}`, 'fail')
  } finally {
    busy.value[t.id] = false
  }
}

async function stopTunnel(t: cloudflare.Tunnel) {
  busy.value[t.id] = true
  try {
    await StopTunnel(t.id)
    runStates.value[t.id] = 'stopped'
    const row = tunnels.value.find((x) => x.id === t.id)
    if (row) row.status = 'down'
    showToast(`已停止 ${t.name}`, 'success')
    pushOp(`停止隧道 ${t.name}（${t.id.slice(0,8)}）`, 'ok')
  } catch (e) {
    const msg = friendlyError(e)
    showToast(msg, 'error')
    pushOp(`停止隧道 ${t.name} 失败：${msg}`, 'fail')
  } finally {
    busy.value[t.id] = false
  }
}

function tunnelName(id: string): string {
  return tunnels.value.find((t) => t.id === id)?.name ?? id
}

function fmtTime(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getFullYear() < 2020) return '—'
  return d.toLocaleString('zh-CN', { hour12: false })
}

function logTime(ts: number): string {
  const d = new Date(ts)
  return d.toLocaleTimeString('zh-CN', { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

function logLevelLabel(level: string): string {
  switch (level) {
    case 'ERROR': return 'ERR'
    case 'WARN': return 'WRN'
    case 'DEBUG': return 'DBG'
    default: return 'INF'
  }
}

function logLevelBadgeClass(level: string): string {
  switch (level) {
    case 'ERROR': return 'bg-red-500/30 text-red-300'
    case 'WARN': return 'bg-amber-500/30 text-amber-300'
    case 'DEBUG': return 'bg-slate-600/30 text-slate-500'
    default: return 'bg-slate-700/50 text-slate-400'
  }
}

watch(
  () => logLines.value.length,
  async () => {
    await nextTick()
    const el = logBox.value
    if (el) el.scrollTop = el.scrollHeight
  },
)

let pollTimer: number | undefined

function restartPoll() {
  if (pollTimer) clearInterval(pollTimer)
  if (pollInterval.value > 0) {
    pollTimer = window.setInterval(() => loadTunnels(true), pollInterval.value * 1000)
  }
}

onMounted(async () => {
  await loadTunnels()
  runStates.value = await GetRunStates()
  EventsOn('cloudflared:log', (p: LogPayload) => {
    pushLog({ ...p, tunnel_id: p.tunnel_id ?? '' })
  })
  EventsOn('cloudflared:status', (p: StatusPayload) => {
    runStates.value[p.tunnel_id] = p.state
    if (p.state === 'error') {
      showToast(`Tunnel ${tunnelName(p.tunnel_id)} 意外退出${p.exit_code != null ? `（退出码 ${p.exit_code}）` : ''}`, 'error')
      pushOp(`意外退出 ${tunnelName(p.tunnel_id)}`, 'fail')
    }
  })
  EventsOn('cloudflared:download', (p: DownloadPayload) => {
    download.value = { active: true, phase: p.phase, downloaded: p.downloaded ?? 0, total: p.total ?? 0 }
    if (p.phase === 'done') {
      window.setTimeout(() => (download.value.active = false), 1000)
    }
  })
  // background poll every 15s
  restartPoll()
})

onUnmounted(() => {
  EventsOff('cloudflared:log')
  EventsOff('cloudflared:status')
  EventsOff('cloudflared:download')
  if (pollTimer) clearInterval(pollTimer)
  if (toastTimer) window.clearTimeout(toastTimer)
})

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
      <div class="flex items-center gap-3">
        <!-- status dot with hover tooltip -->
        <div class="relative group py-2">
          <div class="h-2.5 w-2.5 rounded-full cursor-default bg-slate-300" />
          <div class="absolute right-0 top-full z-[60] w-60 rounded-lg border border-slate-200 bg-white p-3 shadow-xl opacity-0 translate-y-1 transition-all duration-150 group-hover:opacity-100 group-hover:translate-y-0 pointer-events-none group-hover:pointer-events-auto">
            <p class="text-xs font-semibold text-slate-700 mb-1">API 权限</p>
            <div v-for="row in permissionRows" :key="row.key" class="flex items-center justify-between py-0.5">
              <span class="text-xs text-slate-500">{{ row.label }}</span>
              <PermissionBadge :status="state.token_info?.permissions?.[row.key] ?? 'unverified'" />
            </div>
          </div>
        </div>
        <!-- hover dropdown -->
        <div class="relative group py-2">
          <button class="rounded-md border border-slate-300 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-50">☰ 更多</button>
          <div class="absolute right-0 top-full z-[60] w-48 rounded-lg border border-slate-200 bg-white py-1 shadow-xl opacity-0 translate-y-1 transition-all duration-150 group-hover:opacity-100 group-hover:translate-y-0 pointer-events-none group-hover:pointer-events-auto">
            <button class="block w-full px-4 py-1.5 text-left text-xs text-slate-700 hover:bg-slate-50" @click="emit('toggle-log')">终端日志</button>
            <button class="block w-full px-4 py-1.5 text-left text-xs text-slate-700 hover:bg-slate-50" @click="OpenLogDir()">打开日志目录</button>
          </div>
        </div>
        <button class="rounded-md bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white shadow-sm hover:bg-blue-700 transition-shadow" @click="showCreate = true">
          + 创建 Tunnel
        </button>
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

    <!-- slice 03: cloudflared download progress -->
    <div
      v-if="download.active"
      class="flex items-center justify-between gap-4 border-b border-blue-200 bg-blue-50 px-6 py-2"
    >
      <p class="text-sm text-blue-700">
        {{ download.phase === 'extracting' ? '正在解压 cloudflared…' : download.phase === 'done' ? 'cloudflared 就绪' : '正在下载 cloudflared…' }}
      </p>
      <div class="h-2 flex-1 overflow-hidden rounded-full bg-blue-100">
        <div
          class="h-2 rounded-full bg-blue-500 transition-all"
          :style="{
            width:
              download.total > 0
                ? Math.min(100, Math.round((download.downloaded / download.total) * 100)) + '%'
                : '40%',
          }"
        />
      </div>
      <span class="text-xs text-blue-500">
        {{ download.total > 0 ? Math.round((download.downloaded / download.total) * 100) + '%' : '…' }}
      </span>
    </div>

    <main class="flex flex-1 flex-col gap-4 overflow-hidden p-6">
      <!-- loading: skeleton -->
      <div v-if="loading" class="space-y-4" aria-busy="true">
        <p class="text-center text-xs text-slate-400">加载隧道列表…</p>
        <div class="grid grid-cols-4 gap-3">
          <div v-for="i in 4" :key="i" class="h-16 animate-pulse rounded-xl bg-slate-200" />
        </div>
        <div class="space-y-2">
          <div v-for="i in 3" :key="i" class="h-12 animate-pulse rounded-xl bg-slate-200" />
        </div>
      </div>

      <!-- list error: stay on page + retry -->
      <div v-else-if="listError" class="rounded-xl border border-red-200 bg-red-50 p-6 text-center">
        <p class="text-sm font-medium text-red-700">{{ listError }}</p>
        <button
          class="mt-3 rounded-md bg-red-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-red-700"
          @click="retry"
        >
          重试
        </button>
      </div>

      <!-- empty state: create -->
      <div v-else-if="tunnels.length === 0" class="rounded-2xl border border-dashed border-slate-300 bg-white p-10 text-center">
        <p class="text-sm font-medium text-slate-600">此账户还没有 Tunnel</p>
        <button
          class="mt-4 rounded-md bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700"
          @click="showCreate = true"
        >
          创建 Tunnel
        </button>
      </div>

      <!-- stats cards -->
      <div v-if="tunnels.length > 0" class="grid grid-cols-4 gap-3 mb-4">
        <div class="rounded-xl border border-slate-100 bg-white p-5 shadow-sm hover:shadow-md transition-shadow">
          <p class="text-xs text-slate-400">总隧道</p>
          <p class="text-xl font-bold text-slate-800">{{ tunnels.length }}</p>
        </div>
        <div class="rounded-xl border border-slate-100 bg-white p-5 shadow-sm hover:shadow-md transition-shadow">
          <p class="text-xs text-slate-400">运行中</p>
          <p class="text-xl font-bold text-green-600">{{ runningCount }}</p>
        </div>
        <div class="rounded-xl border border-slate-100 bg-white p-5 shadow-sm hover:shadow-md transition-shadow">
          <p class="text-xs text-slate-400">已停止</p>
          <p class="text-xl font-bold text-slate-500">{{ stoppedCount }}</p>
        </div>
        <div class="rounded-xl border border-slate-100 bg-white p-5 shadow-sm hover:shadow-md transition-shadow">
          <p class="text-xs text-slate-400">异常</p>
          <p class="text-xl font-bold text-red-500">{{ errorCount }}</p>
        </div>
      </div>

      <!-- tunnel list -->
      <div v-if="tunnels.length > 0" class="overflow-hidden rounded-xl border border-slate-100 bg-white shadow-sm">
        <div class="flex items-center justify-end border-b border-slate-100 px-4 py-1.5">
          <select v-model.number="pollInterval" class="rounded border border-slate-200 px-1.5 py-0.5 text-xs" @change="restartPoll" title="轮询间隔">
            <option :value="5">5s</option>
            <option :value="15">15s</option>
            <option :value="30">30s</option>
            <option :value="60">60s</option>
            <option :value="0">关闭</option>
          </select>
        </div>
        <table class="min-w-full divide-y divide-slate-200 text-xs">
          <thead class="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th class="px-4 py-2.5 font-medium">名称</th>
              <th class="px-4 py-2.5 font-medium">状态</th>
              <th class="px-4 py-2.5 font-medium">Tunnel ID</th>
              <th class="px-4 py-2.5 font-medium">创建时间</th>
              <th class="px-4 py-2.5 font-medium">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="t in tunnels" :key="t.id" class="hover:bg-blue-50/50 transition-colors">
              <td class="px-4 py-2.5 font-medium text-slate-900">{{ t.name }}</td>
              <td class="px-4 py-2.5 whitespace-nowrap">
                <span
                  class="inline-flex min-w-[4rem] items-center justify-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1"
                  :class="runStates[t.id] === 'running' ? 'bg-green-100 text-green-700 ring-green-200' : runStates[t.id] === 'error' ? 'bg-red-100 text-red-700 ring-red-200' : 'bg-slate-200 text-slate-600 ring-slate-300'"
                >
                  {{ runStates[t.id] === 'running' ? '正常' : runStates[t.id] === 'error' ? '异常' : '断开' }}
                </span>
              </td>
              <td class="px-4 py-2.5 font-mono text-xs text-slate-500">{{ t.id }}</td>
              <td class="px-4 py-2.5 text-slate-500">{{ fmtTime(t.created_at) }}</td>
              <td class="px-4 py-2.5 whitespace-nowrap">
                <button
                  class="mr-2 rounded-md border border-slate-300 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-50"
                  @click="openIngress(t)"
                >
                  Ingress
                </button>
                <button
                  class="mr-2 rounded-md border border-slate-300 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-50"
                  @click="detailTunnelId = t.id"
                >
                  详情
                </button>
                <button
                  v-if="runStates[t.id] !== 'running'"
                  :disabled="busy[t.id]"
                  class="inline-block w-[72px] rounded-md bg-green-600 px-3 py-1 text-xs font-semibold text-white hover:bg-green-700 disabled:bg-green-300"
                  @click="startTunnel(t)"
                >
                  {{ busy[t.id] ? '启动中' : '启动' }}
                </button>
                <button
                  v-else
                  :disabled="busy[t.id]"
                  class="inline-block w-[72px] rounded-md bg-red-600 px-3 py-1 text-xs font-semibold text-white hover:bg-red-700 disabled:bg-red-300"
                  @click="stopTunnel(t)"
                >
                  {{ busy[t.id] ? '停止中' : '停止' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

    </main>

    <!-- slice 04: dialogs -->
    <CreateTunnelDialog v-if="showCreate" @close="showCreate = false" @created="onCreated" />
    <TunnelDetailDialog
      v-if="detailTunnelId"
      :tunnel-id="detailTunnelId"
      :run-state="runStates[detailTunnelId] ?? 'stopped'"
      @close="detailTunnelId = null"
      @deleted="onDeleted"
    />

    <!-- slice 05: ingress editor overlay -->
    <Transition name="page">
      <div v-if="ingressTunnelId" class="fixed inset-0 z-[60] bg-slate-100">
        <IngressEditorView
          :tunnel-id="ingressTunnelId"
          :tunnel-name="ingressTunnelName"
          @back="ingressTunnelId = null"
          @toggle-log="emit('toggle-log')"
        />
      </div>
    </Transition>

    <!-- toast -->
    <div
      v-if="toast"
      class="fixed right-6 top-6 z-[60] max-w-md rounded-lg px-4 py-2.5 text-sm text-white shadow-lg break-words"
      :class="toast.type === 'success' ? 'bg-green-600' : toast.type === 'error' ? 'bg-red-600' : 'bg-slate-800'"
    >
      {{ toast.msg }}
    </div>
  </div>
</template>

<style>
.page-enter-active, .page-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.page-enter-from { opacity: 0; transform: translateX(20px); }
.page-leave-to { opacity: 0; transform: translateX(-20px); }
</style>
