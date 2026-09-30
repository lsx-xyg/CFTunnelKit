<script lang="ts" setup>
import { ref } from 'vue'
import { CreateTunnel, GetTunnelToken } from '../../wailsjs/go/main/App'
import type { cloudflare } from '../../wailsjs/go/models'

// Create dialog (issue #6): name validation → CreateTunnel → immediate
// run-token display with copy. A token fetch failure never blocks the
// creation; the dialog then offers "请在详情页重试".
const emit = defineEmits<{ (e: 'close'): void; (e: 'created'): void }>()

const name = ref('')
const error = ref('')
const busy = ref(false)

const phase = ref<'form' | 'token'>('form')
const created = ref<cloudflare.Tunnel | null>(null)
const token = ref('')
const tokenError = ref('')
const showToken = ref(false)
const copied = ref(false)

const NAME_RE = /^[a-zA-Z0-9_-]+$/

function validate(): string {
  const n = name.value.trim()
  if (!n) return '请输入 Tunnel 名称'
  if (n.length > 32) return '名称长度不能超过 32 个字符'
  if (!NAME_RE.test(n)) return '名称只允许字母、数字、- 和 _'
  return ''
}

async function submit() {
  error.value = validate()
  if (error.value) return
  busy.value = true
  try {
    const tun = await CreateTunnel(name.value.trim())
    created.value = tun
    try {
      token.value = await GetTunnelToken(tun.id)
    } catch (e) {
      tokenError.value = 'Token 获取失败，请在详情页重试'
    }
    phase.value = 'token'
    emit('created')
  } catch (e) {
    error.value = String(e)
  } finally {
    busy.value = false
  }
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
</script>

<template>
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40" @click.self="emit('close')">
    <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
      <template v-if="phase === 'form'">
        <h2 class="text-lg font-bold text-slate-900">创建 Tunnel</h2>
        <p class="mt-1 text-xs text-slate-400">创建远程管理型 Tunnel（config_src: cloudflare）</p>

        <label class="mt-4 block text-sm font-medium text-slate-700">
          名称
          <input
            v-model="name"
            maxlength="32"
            placeholder="例如 my-tunnel（字母/数字/-/_，≤32 字符）"
            class="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none"
            @keyup.enter="submit"
          />
        </label>
        <p v-if="error" class="mt-2 text-xs text-red-600">{{ error }}</p>

        <div class="mt-5 flex justify-end gap-2">
          <button
            class="rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            @click="emit('close')"
          >
            取消
          </button>
          <button
            :disabled="busy"
            class="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:bg-blue-300"
            @click="submit"
          >
            {{ busy ? '创建中…' : '创建' }}
          </button>
        </div>
      </template>

      <template v-else>
        <h2 class="text-lg font-bold text-slate-900">Tunnel 创建成功</h2>
        <p class="mt-1 text-sm text-slate-600">
          <span class="font-medium">{{ created?.name }}</span>
          <span class="ml-2 font-mono text-xs text-slate-400">{{ created?.id }}</span>
        </p>

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
          <p class="mt-2 text-xs text-slate-400">该 Token 用于 cloudflared tunnel run，请妥善保存。</p>
        </div>

        <div class="mt-5 flex justify-end">
          <button
            class="rounded-md bg-slate-800 px-4 py-1.5 text-sm font-semibold text-white hover:bg-slate-700"
            @click="emit('close')"
          >
            关闭
          </button>
        </div>
      </template>
    </div>
  </div>
</template>
