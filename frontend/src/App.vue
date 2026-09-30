<script lang="ts" setup>
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { GetAuthState, RetryVerify } from '../wailsjs/go/main/App'
import { EventsOn, EventsOff, EventsEmit } from '../wailsjs/runtime/runtime'
import { auth } from '../wailsjs/go/models'
import AuthView from './views/AuthView.vue'
import MainView from './views/MainView.vue'

const state = ref<auth.State>(new auth.State({}))
const booting = ref(true)

const logVisible = ref(false)
const logTab = ref<'runtime' | 'ops'>('runtime')
const logLines = ref<{ ts: number; level: string; line: string; tunnel_id?: string }[]>([])
const opLogs = ref<{ ts: number; action: string; result: string }[]>([])
const logSearch = ref('')
const logLevelFilter = ref('')
const logHeight = ref(192) // px, adjustable by drag
const logFullscreen = ref(false)

function pushLog(p: any) {
  logLines.value.push({ ts: Date.now(), level: p.level || 'INFO', line: p.line, tunnel_id: p.tunnel_id })
  if (logLines.value.length > 500) logLines.value.splice(0, logLines.value.length - 500)
}

function pushOp(p: any) {
  opLogs.value.push({ ts: Date.now(), action: p.action, result: p.result })
  if (opLogs.value.length > 200) opLogs.value.splice(0, opLogs.value.length - 200)
}

function toggleLog() {
  logVisible.value = !logVisible.value
}

function onKey(e: KeyboardEvent) {
  if (e.key === '`' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    toggleLog()
  }
}

const filteredLogs = computed(() => {
  let out = logLines.value
  if (logLevelFilter.value) out = out.filter((l) => l.level === logLevelFilter.value)
  if (logSearch.value) {
    const q = logSearch.value.toLowerCase()
    out = out.filter((l) => l.line.toLowerCase().includes(q))
  }
  return out
})

function levelColor(level: string) {
  if (level === 'ERROR' || level === 'ERR') return 'text-red-400'
  if (level === 'WARN' || level === 'WRN') return 'text-yellow-400'
  if (level === 'DEBUG' || level === 'DBG') return 'text-slate-500'
  return 'text-blue-400' // INFO
}

function levelBadge(level: string) {
  if (level === 'ERROR' || level === 'ERR') return 'text-red-400'
  if (level === 'WARN' || level === 'WRN') return 'text-yellow-300'
  if (level === 'DEBUG' || level === 'DBG') return 'text-slate-500'
  return 'text-green-400'
}

// drag to resize
let dragStartY = 0
let dragStartH = 0
function onDragStart(e: MouseEvent) {
  dragStartY = e.clientY
  dragStartH = logHeight.value
  document.onmousemove = onDragMove
  document.onmouseup = onDragEnd
}
function onDragMove(e: MouseEvent) {
  logHeight.value = Math.max(80, Math.min(window.innerHeight - 100, dragStartH + (dragStartY - e.clientY)))
}
function onDragEnd() {
  document.onmousemove = null
  document.onmouseup = null
}

async function refresh() {
  state.value = await GetAuthState()
  if (state.value.has_token && !state.value.authenticated && !state.value.offline && !state.value.message) {
    try { await RetryVerify() } catch (e) { console.error('restore failed', e) }
    state.value = await GetAuthState()
  }
  booting.value = false
}

async function onSessionExpired() {
  state.value = await GetAuthState()
  state.value.message = 'Token 已失效，请重新输入'
}

onMounted(() => {
  refresh()
  EventsOn('cloudflared:log', pushLog)
  EventsOn('app:op', pushOp)
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  EventsOff('cloudflared:log')
  EventsOff('app:op')
  window.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div class="h-full">
    <div v-if="booting" class="flex h-full items-center justify-center">
      <p class="text-sm text-slate-400">正在启动…</p>
    </div>
    <AuthView v-else-if="!state.authenticated" :initial-message="state.message" @verified="refresh" />
    <MainView v-else :state="state" @refresh="refresh" @session-expired="onSessionExpired" @toggle-log="toggleLog" />

    <Transition name="slide-up">
      <div v-if="logVisible"
        class="fixed bottom-0 left-0 right-0 z-50 flex flex-col border-t border-slate-700 bg-slate-900 shadow-2xl"
        :class="logFullscreen ? 'top-10' : ''">
        <!-- drag handle -->
        <div class="flex justify-center pt-0.5 cursor-row-resize select-none" @mousedown="onDragStart">
          <div class="h-1 w-16 rounded-full bg-slate-600"></div>
        </div>
        <div class="flex items-center justify-between gap-2 border-b border-slate-700 px-3 py-1">
          <div class="flex items-center gap-1">
            <button class="rounded px-2 py-0.5 text-xs font-medium" :class="logTab === 'runtime' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'" @click="logTab = 'runtime'">运行日志</button>
            <button class="rounded px-2 py-0.5 text-xs font-medium" :class="logTab === 'ops' ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'" @click="logTab = 'ops'">操作记录</button>
          </div>
          <div class="flex items-center gap-2">
            <input v-if="logTab === 'runtime'" v-model="logSearch" type="text" placeholder="搜索..." class="w-32 rounded border border-slate-600 bg-slate-800 px-2 py-0.5 text-xs text-slate-200 placeholder-slate-500" />
            <select v-if="logTab === 'runtime'" v-model="logLevelFilter" class="rounded border border-slate-600 bg-slate-800 px-1 py-0.5 text-xs text-slate-300">
              <option value="">全部级别</option>
              <option value="ERROR">ERROR</option>
              <option value="WARN">WARN</option>
              <option value="INFO">INFO</option>
              <option value="DEBUG">DEBUG</option>
            </select>
            <span class="text-xs text-slate-500">{{ logTab === 'runtime' ? filteredLogs.length : opLogs.length }} 行</span>
            <button class="text-xs text-slate-400 hover:text-white" @click="logFullscreen = !logFullscreen">{{ logFullscreen ? '还原' : '全屏' }}</button>
            <button class="text-xs text-slate-400 hover:text-white" @click="toggleLog">收起</button>
          </div>
        </div>
        <div class="overflow-y-auto px-3 py-2 font-mono text-xs leading-5 text-slate-100 slim-scroll"
          :style="logFullscreen ? 'flex-1' : 'height:' + logHeight + 'px'">
          <template v-if="logTab === 'runtime'">
            <p v-if="filteredLogs.length === 0" class="text-slate-500">暂无日志 — 启动 Tunnel 后显示</p>
            <p v-for="(l, i) in filteredLogs" :key="i" class="whitespace-pre-wrap break-all">
              <span class="mr-2 font-bold" :class="levelBadge(l.level)">{{ l.level }}</span>
              <span :class="levelColor(l.level)">{{ l.line }}</span>
            </p>
          </template>
          <template v-else>
            <p v-if="opLogs.length === 0" class="text-slate-500">暂无操作记录</p>
            <p v-for="(o, i) in opLogs" :key="i" class="whitespace-pre-wrap break-all">
              <span class="mr-2 text-slate-500">{{ new Date(o.ts).toLocaleTimeString() }}</span>
              <span class="mr-2" :class="o.result === 'ok' ? 'text-green-400' : 'text-red-400'">{{ o.result === 'ok' ? 'OK' : 'FAIL' }}</span>
              <span>{{ o.action }}</span>
            </p>
          </template>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style>
.slide-up-enter-active, .slide-up-leave-active { transition: transform 0.2s ease, opacity 0.2s ease; }
.slide-up-enter-from, .slide-up-leave-to { transform: translateY(100%); opacity: 0; }
.slim-scroll::-webkit-scrollbar { width: 4px; }
.slim-scroll::-webkit-scrollbar-track { background: transparent; }
.slim-scroll::-webkit-scrollbar-thumb { background: #475569; border-radius: 2px; }
</style>
