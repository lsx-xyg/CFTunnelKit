<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { DeleteTunnel, GetTunnelDetail, GetTunnelToken } from '../../wailsjs/go/main/App'
import type { cloudflare } from '../../wailsjs/go/models'
import TunnelStatusBadge from './TunnelStatusBadge.vue'

// Detail dialog (issue #6): metadata + connection count + run token
// (hidden by default, reveal + copy) + delete flow (type the tunnel name
// to confirm; active-connection errors surface inline).
const props = defineProps<{ tunnelId: string; runState?: string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'deleted'): void }>()

const loading = ref(true)
const error = ref('')
const detail = ref<cloudflare.TunnelDetail | null>(null)

const token = ref('')
const tokenError = ref('')
const showToken = ref(false)
const copied = ref(false)

const phase = ref<'detail' | 'confirm-delete'>('detail')
const confirmName = ref('')
const deleteBusy = ref(false)
const deleteError = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    detail.value = await GetTunnelDetail(props.tunnelId)
  } catch (e) {
    error.value = String(e)
    loading.value = false
    return
  }
  try {
    token.value = await GetTunnelToken(props.tunnelId)
  } catch (e) {
    tokenError.value = 'Token 获取失败' + (String(e).includes('权限') ? '，请检查 Token 权限' : '，请稍后重试')
  }
  loading.value = false
}

async function copyToken() {
  try {
    await navigator.clipboard.writeText(token.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    error.value = '复制失败，请手动选择复制'
  }
}

function openDelete() {
  phase.value = 'confirm-delete'
  confirmName.value = ''
  deleteError.value = ''
}

async function doDelete() {
  if (!detail.value) return
  if (confirmName.value.trim() !== detail.value.name) {
    deleteError.value = '输入的名称与 Tunnel 名称不一致'
    return
  }
  deleteBusy.value = true
  deleteError.value = ''
  try {
    await DeleteTunnel(props.tunnelId)
    emit('deleted')
  } catch (e) {
    deleteError.value = String(e)
    deleteBusy.value = false
  }
}

function fmtTime(iso: string | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString('zh-CN', { hour12: false })
}

onMounted(load)
</script>

<template>
  <div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-900/40 animate-fade-in" @click.self="phase === 'confirm-delete' ? null : emit('close')">
    <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl animate-zoom-in">
      <template v-if="phase === 'detail'">
        <div class="flex items-start justify-between">
          <div>
            <h2 class="text-lg font-bold text-slate-900">{{ detail?.name ?? '…' }}</h2>
            <p class="mt-0.5 font-mono text-xs text-slate-400">{{ detail?.id }}</p>
          </div>
          <button class="text-slate-400 hover:text-slate-600" @click="emit('close')">✕</button>
        </div>

        <div v-if="loading" class="mt-4 space-y-2">
          <div class="h-4 animate-pulse rounded bg-slate-100" />
          <div class="h-4 animate-pulse rounded bg-slate-100" />
          <div class="h-4 animate-pulse rounded bg-slate-100" />
        </div>
        <p v-else-if="error" class="mt-4 text-sm text-red-600">{{ error }}</p>

        <template v-else-if="detail">
          <dl class="mt-4 space-y-2 text-sm">
            <div class="flex justify-between">
              <dt class="text-slate-500">状态</dt>
              <dd><span class="inline-flex min-w-[4rem] items-center justify-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1"
                :class="runState === 'running' ? 'bg-green-100 text-green-700 ring-green-200' : runState === 'error' ? 'bg-red-100 text-red-700 ring-red-200' : 'bg-slate-200 text-slate-600 ring-slate-300'">
                {{ runState === 'running' ? '运行中' : runState === 'error' ? '异常' : '未运行' }}
              </span></dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-500">创建时间</dt>
              <dd class="text-slate-700">{{ fmtTime(detail.created_at) }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-500">连接数</dt>
              <dd class="text-slate-700">{{ detail.connections }}</dd>
            </div>
          </dl>

          <div class="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-4">
            <p class="text-sm font-medium text-slate-700">运行 Token</p>
            <p v-if="tokenError" class="mt-1 text-xs text-amber-700">{{ tokenError }}</p>
            <template v-else>
              <p class="mt-1 break-all font-mono text-xs text-slate-500">
                {{ showToken ? token : '••••••••••••••••••••••••••••' }}
              </p>
              <div class="mt-3 flex gap-2">
                <button
                  class="rounded-md border border-slate-300 px-3 py-1 text-xs font-medium text-slate-600 hover:bg-white"
                  @click="showToken = !showToken"
                >
                  {{ showToken ? '隐藏' : '显示' }}
                </button>
                <button
                  class="rounded-md bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-700"
                  @click="copyToken"
                >
                  {{ copied ? '已复制' : '复制' }}
                </button>
              </div>
            </template>
          </div>

          <div class="mt-5 flex justify-between">
            <button
              class="rounded-md border border-red-300 px-4 py-1.5 text-sm font-semibold text-red-600 hover:bg-red-50"
              @click="openDelete"
            >
              删除 Tunnel
            </button>
            <button
              class="rounded-md bg-slate-800 px-4 py-1.5 text-sm font-semibold text-white hover:bg-slate-700"
              @click="emit('close')"
            >
              关闭
            </button>
          </div>
        </template>
      </template>

      <template v-else>
        <h2 class="text-lg font-bold text-red-600">删除 Tunnel</h2>
        <p class="mt-2 text-sm text-slate-600">
          此操作不可撤销。请输入 Tunnel 名称
          <span class="font-mono font-medium text-slate-900">{{ detail?.name }}</span>
          以确认删除。
        </p>
        <input
          v-model="confirmName"
          :placeholder="detail?.name ?? '输入 Tunnel 名称'"
          class="mt-3 w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-red-500 focus:outline-none"
          @keyup.enter="doDelete"
        />
        <p v-if="deleteError" class="mt-2 text-xs text-red-600">{{ deleteError }}</p>
        <div class="mt-5 flex justify-end gap-2">
          <button
            class="rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            @click="phase = 'detail'"
          >
            取消
          </button>
          <button
            :disabled="deleteBusy || confirmName.trim() !== (detail?.name ?? '')"
            class="rounded-md bg-red-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-red-700 disabled:bg-red-300"
            @click="doDelete"
          >
            {{ deleteBusy ? '删除中…' : '确认删除' }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>
