<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { GetIngressConfig, ListZones, SaveIngressConfig } from '../../wailsjs/go/main/App'
import type { cloudflare } from '../../wailsjs/go/models'

// Ingress editor (issue #5): visual editing of 域名 → 本地端口 rules.
// The catch-all 404 rule is maintained invisibly by the backend on save
// (PutIngressConfig appends it); GET strips it, so the user never sees it.
const props = defineProps<{ tunnelId: string; tunnelName: string }>()
const emit = defineEmits<{ (e: 'back'): void }>()

const rules = ref<cloudflare.IngressRule[]>([])
const zones = ref<cloudflare.Zone[]>([])
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const toast = ref('')
let snapshot = '[]' // normalized snapshot of the last saved state
let toastTimer: number | undefined

const SERVICE_RE = /^(https?|tcp|ssh|unix|rdp):\/\/|^http_status:\d+$/

function norm(r: cloudflare.IngressRule): string {
  return `${(r.hostname ?? '').trim().toLowerCase()}|${(r.service ?? '').trim().toLowerCase()}`
}

// zoneMatch: longest suffix zone for a hostname (issue #5/#6).
function zoneMatch(hostname: string): string {
  let h = hostname.trim().toLowerCase()
  if (h.startsWith('*.')) h = h.slice(2)
  let best = ''
  for (const z of zones.value) {
    const zn = z.name.trim().toLowerCase()
    if ((h === zn || h.endsWith('.' + zn)) && zn.length > best.length) best = zn
  }
  return best
}

function rowErrors(i: number): string[] {
  const r = rules.value[i]
  const h = (r.hostname ?? '').trim()
  const s = (r.service ?? '').trim()
  const errs: string[] = []
  if (!h) {
    errs.push('hostname 不能为空')
  } else {
    if (rules.value.filter((x, j) => j !== i && (x.hostname ?? '').trim().toLowerCase() === h.toLowerCase()).length > 0) {
      errs.push('hostname 重复')
    }
    if (h.includes('*') && (!h.startsWith('*.') || h.split('*').length > 2)) {
      errs.push('通配符 * 仅允许出现在最左侧（如 *.example.com）')
    }
    if (zones.value.length > 0 && !zoneMatch(h)) {
      errs.push('域名不在当前账户的 Zone 列表中')
    }
  }
  if (!SERVICE_RE.test(s)) {
    errs.push('service 格式不正确（如 http://localhost:8080 或 http_status:404）')
  }
  return errs
}

const wholeListErrors = computed(() => {
  if (rules.value.length === 0) return ['至少需要一条规则']
  return []
})

const dirty = computed(() => JSON.stringify(rules.value.map(norm)) !== snapshot)

const saveable = computed(() => rules.value.length > 0 && rules.value.every((_, i) => rowErrors(i).length === 0))

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [rs, zs] = await Promise.all([GetIngressConfig(props.tunnelId), ListZones()])
    rules.value = rs
    zones.value = zs
    snapshot = JSON.stringify(rs.map(norm))
  } catch (e) {
    loadError.value = String(e)
  } finally {
    loading.value = false
  }
}

function addRule() {
  rules.value.push({ hostname: '', service: '' })
}

function removeRule(i: number) {
  rules.value.splice(i, 1)
}

function move(i: number, dir: -1 | 1) {
  const j = i + dir
  if (j < 0 || j >= rules.value.length) return
  const arr = rules.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

async function save() {
  if (!saveable.value) return
  saving.value = true
  saveError.value = ''
  try {
    const got = await SaveIngressConfig(props.tunnelId, rules.value)
    rules.value = got
    snapshot = JSON.stringify(got.map(norm))
    showToast('配置已保存')
  } catch (e) {
    // PUT failed or read-back mismatch → keep user input, show the error
    saveError.value = String(e)
  } finally {
    saving.value = false
  }
}

function showToast(msg: string) {
  toast.value = msg
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = ''), 2500)
}

// ---- unsaved-changes guard ----
const confirmLeave = ref(false)

function requestBack() {
  if (dirty.value) confirmLeave.value = true
  else emit('back')
}

function leave() {
  confirmLeave.value = false
  emit('back')
}

onMounted(load)
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- header -->
    <header class="flex items-center justify-between border-b border-slate-200 bg-white px-5 py-3">
      <div class="flex items-center gap-3">
        <button
          class="rounded-md border border-slate-300 px-3 py-1 text-sm font-medium text-slate-600 hover:bg-slate-50"
          @click="requestBack"
        >
          ← 返回列表
        </button>
        <div>
          <h1 class="text-sm font-bold text-slate-900">Ingress 规则</h1>
          <p class="text-xs text-slate-400">{{ tunnelName }} · {{ tunnelId }}</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="rounded-md bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700">
          规则从上到下匹配，第一条生效
        </span>
        <button
          :disabled="!saveable || saving"
          class="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:bg-blue-300"
          @click="save"
        >
          {{ saving ? '保存中…' : dirty ? '保存' : '已保存' }}
        </button>
      </div>
    </header>

    <main class="flex-1 overflow-y-auto p-5">
      <p v-if="loadError" class="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{{ loadError }}</p>

      <div v-else-if="loading" class="space-y-2">
        <div class="h-12 animate-pulse rounded-lg bg-slate-100" />
        <div class="h-12 animate-pulse rounded-lg bg-slate-100" />
        <div class="h-12 animate-pulse rounded-lg bg-slate-100" />
      </div>

      <template v-else>
        <div v-if="zones.length === 0" class="mb-4 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700">
          当前账户下无 Zone，请先在 Cloudflare 添加域名。hostname 校验将跳过。
        </div>

        <div v-if="rules.length === 0 && !wholeListErrors.length" class="rounded-2xl border border-dashed border-slate-300 p-8 text-center">
          <p class="text-sm text-slate-500">尚未配置规则（Tunnel 未配置或仅有兜底规则）</p>
        </div>

        <div class="space-y-2">
          <div
            v-for="(r, i) in rules"
            :key="i"
            class="rounded-xl border border-slate-200 bg-white p-3"
            :class="{ 'border-red-300': rowErrors(i).length > 0 }"
          >
            <div class="flex items-center gap-2">
              <span class="w-6 text-center text-xs font-semibold text-slate-400">{{ i + 1 }}</span>
              <input
                v-model="r.hostname"
                placeholder="hostname，如 nas.example.com 或 *.example.com"
                class="flex-1 rounded-md border border-slate-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none"
              />
              <span class="text-slate-400">→</span>
              <input
                v-model="r.service"
                placeholder="http://localhost:8080"
                class="flex-1 rounded-md border border-slate-300 px-3 py-1.5 font-mono text-sm focus:border-blue-500 focus:outline-none"
              />
              <div class="flex flex-col gap-0.5">
                <button
                  class="rounded border border-slate-300 px-1.5 text-xs text-slate-500 hover:bg-slate-50 disabled:opacity-30"
                  :disabled="i === 0"
                  @click="move(i, -1)"
                >
                  ↑
                </button>
                <button
                  class="rounded border border-slate-300 px-1.5 text-xs text-slate-500 hover:bg-slate-50 disabled:opacity-30"
                  :disabled="i === rules.length - 1"
                  @click="move(i, 1)"
                >
                  ↓
                </button>
              </div>
              <button
                class="rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50"
                @click="removeRule(i)"
              >
                删除
              </button>
            </div>
            <ul v-if="rowErrors(i).length" class="mt-1.5 ml-8 space-y-0.5">
              <li v-for="(e, k) in rowErrors(i)" :key="k" class="text-xs text-red-600">{{ e }}</li>
            </ul>
          </div>
        </div>

        <p v-if="wholeListErrors.length" class="mt-3 text-xs text-red-600">{{ wholeListErrors[0] }}</p>
        <p v-if="saveError" class="mt-3 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{{ saveError }}</p>

        <button
          class="mt-4 rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
          @click="addRule"
        >
          + 添加规则
        </button>
        <p class="mt-2 text-xs text-slate-400">
          保存时自动追加末位 404 兜底规则（http_status:404），无需手动维护。
        </p>
      </template>
    </main>

    <!-- unsaved-changes confirm -->
    <div v-if="confirmLeave" class="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/40">
      <div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
        <h2 class="text-base font-bold text-slate-900">有未保存的修改</h2>
        <p class="mt-2 text-sm text-slate-600">离开后修改将丢失，确定返回列表吗？</p>
        <div class="mt-5 flex justify-end gap-2">
          <button
            class="rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            @click="confirmLeave = false"
          >
            继续编辑
          </button>
          <button
            class="rounded-md bg-red-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-red-700"
            @click="leave"
          >
            放弃修改
          </button>
        </div>
      </div>
    </div>

    <!-- toast -->
    <div
      v-if="toast"
      class="fixed right-6 top-6 z-50 max-w-sm rounded-lg bg-slate-800 px-4 py-2.5 text-sm text-white shadow-lg"
    >
      {{ toast }}
    </div>
  </div>
</template>
