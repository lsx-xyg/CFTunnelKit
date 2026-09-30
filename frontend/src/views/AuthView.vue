<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { VerifyAndSaveToken, GetProxy, SetProxy } from '../../wailsjs/go/main/App'
import type { cloudflare } from '../../wailsjs/go/models'
import PermissionBadge from '../components/PermissionBadge.vue'

const props = defineProps<{ initialMessage: string }>()
const emit = defineEmits<{ (e: 'verified'): void }>()

const token = ref('')
const loading = ref(false)
const error = ref('')
const warnings = ref<string[]>([])
const permissions = ref<cloudflare.Permissions | null>(null)
const proxy = ref('')
const showAdvanced = ref(false)

const MIN_TOKEN_LENGTH = 20

onMounted(async () => {
  proxy.value = await GetProxy()
})

async function saveProxy() {
  await SetProxy(proxy.value.trim())
}

function validate(token: string): string {
  const t = token.trim()
  if (t === '') return '请输入 Cloudflare API Token'
  if (t.length < MIN_TOKEN_LENGTH) return 'Token 长度异常，请检查是否完整复制'
  return ''
}

async function verify() {
  const msg = validate(token.value)
  if (msg) {
    error.value = msg
    return
  }
  loading.value = true
  error.value = ''
  warnings.value = []
  permissions.value = null
  try {
    const info = await VerifyAndSaveToken(token.value.trim())
    warnings.value = info.warnings ?? []
    permissions.value = info.permissions
    emit('verified')
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

if (props.initialMessage) {
  error.value = props.initialMessage
}

const permissionRows: { key: keyof cloudflare.Permissions; label: string }[] = [
  { key: 'tunnel_edit', label: 'Tunnel 管理权限 (Tunnel:Edit)' },
  { key: 'zone_read', label: '域名读取权限 (Zone:Read)' },
  { key: 'dns_edit', label: 'DNS 编辑权限 (DNS:Edit)' },
]
</script>

<template>
  <div class="flex h-full items-center justify-center bg-slate-50">
    <div class="w-full max-w-md rounded-2xl bg-white p-8 shadow-sm ring-1 ring-slate-200">
      <h1 class="text-2xl font-bold text-slate-900">CFTunnelKit</h1>
      <p class="mt-1 text-sm text-slate-500">
        Cloudflare Tunnel 桌面管理器 — 请粘贴 API Token 开始使用
      </p>

      <div class="mt-6">
        <label for="token" class="text-sm font-medium text-slate-700">Cloudflare API Token</label>
        <input
          id="token"
          v-model="token"
          type="password"
          autocomplete="off"
          placeholder="粘贴你的 API Token"
          class="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          @keyup.enter="verify"
        />
        <p class="mt-2 text-xs leading-relaxed text-slate-400">
          需要权限：Cloudflare Tunnel:Edit、DNS:Edit、Zone:Read
        </p>
      </div>

      <div class="mt-4">
        <button class="text-xs text-slate-400 hover:text-slate-600" @click="showAdvanced = !showAdvanced">
          {{ showAdvanced ? '▾' : '▸' }} 高级：代理设置（无法连接 Cloudflare API 时填写）
        </button>
        <div v-if="showAdvanced" class="mt-2 rounded-lg border border-slate-200 p-3">
          <input
            v-model="proxy"
            type="text"
            placeholder="http://127.0.0.1:7890"
            class="w-full rounded-md border border-slate-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
            @blur="saveProxy"
          />
          <p class="mt-1 text-xs text-slate-400">
            留空 = 直连。Clash 填 7890，v2rayN 填 10809。填完点其他地方自动保存。
          </p>
        </div>
      </div>

      <button
        :disabled="loading"
        class="mt-4 w-full rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-300"
        @click="verify"
      >
        {{ loading ? '正在校验…' : '验证并保存' }}
      </button>

      <p v-if="error" class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{{ error }}</p>

      <div v-if="permissions" class="mt-4 space-y-2">
        <p class="text-sm font-medium text-slate-700">权限探测结果</p>
        <div v-for="row in permissionRows" :key="row.key" class="flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2">
          <span class="text-sm text-slate-600">{{ row.label }}</span>
          <PermissionBadge :status="permissions[row.key]" />
        </div>
      </div>

      <div v-if="warnings.length" class="mt-4 space-y-1">
        <p v-for="w in warnings" :key="w" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700">{{ w }}</p>
      </div>
    </div>
  </div>
</template>
