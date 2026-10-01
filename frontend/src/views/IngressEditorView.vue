<script lang="ts" setup>
import { computed, onMounted, ref, type Ref } from 'vue'
import { api, type cloudflare } from '@/api'
import { friendlyError } from '@/utils/error'
import { useEscape } from '@/composables/useEscape'

// Ingress editor (issue #5): visual editing of 域名 → 本地端口 rules.
// The catch-all 404 rule is maintained invisibly by the backend on save
// (PutIngressConfig appends it); GET strips it, so the user never sees it.
const props = defineProps<{ tunnelId: string; tunnelName: string }>()
const emit = defineEmits<{ (e: 'back'): void }>()

interface Rule extends cloudflare.IngressRule { locked?: boolean; _id?: string }
const rules = ref<Rule[]>([])
const zones = ref<cloudflare.Zone[]>([])
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const toast = ref('')
let snapshot = '[]' // normalized snapshot of the last saved state
let savedHosts: string[] = [] // hostnames from last successful save (for DNS diff)
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

function zoneFor(hostname: string): cloudflare.Zone | undefined {
  const zn = zoneMatch(hostname)
  return zones.value.find((z) => z.name.trim().toLowerCase() === zn)
}

// relName converts a full hostname to the zone-relative DNS record name
// (issue #6): nas.example.com → nas, *.example.com → *, a.b.example.com →
// a.b, apex → @.
function relName(hostname: string, zoneName: string): string {
  let h = hostname.trim().toLowerCase().replace(/\.$/, '')
  const z = zoneName.trim().toLowerCase().replace(/\.$/, '')
  if (h === z) return '@'
  if (h.endsWith('.' + z)) return h.slice(0, h.length - z.length - 1)
  return h
}

// ---- slice 06: DNS link dialogs ----
interface DNSPrompt {
  create: string[] // hostnames to create CNAMEs for
  remove: string[] // hostnames to delete DNS for
}
const dnsPrompt = ref<DNSPrompt | null>(null)
useEscape(() => (dnsPrompt.value = null), dnsPrompt as unknown as Ref<boolean>)
const dnsCreateChecked = ref(true)
const dnsRemoveChecked = ref(true)
const dnsRunning = ref(false)
const dnsResults = ref<string[]>([])

// delete-link prompt when a rule is removed in the editor
const deletePrompt = ref<string[] | null>(null)
useEscape(() => (deletePrompt.value = null), deletePrompt as unknown as Ref<boolean>)
const deleteDNSChecked = ref(true)

function hostnamesOf(rulesList: cloudflare.IngressRule[]): string[] {
  const set: string[] = []
  for (const r of rulesList) {
    const h = (r.hostname ?? '').trim().toLowerCase()
    if (h && !set.includes(h)) set.push(h)
  }
  return set
}

async function runDNSPrompt() {
  if (!dnsPrompt.value) return
  const p = dnsPrompt.value
  const target = props.tunnelId + '.cfargotunnel.com'
  dnsRunning.value = true
  dnsResults.value = []
  try {
    if (dnsCreateChecked.value) {
      for (const h of p.create) {
        const z = zoneFor(h)
        if (!z) {
          dnsResults.value.push(`${h}: 无匹配 Zone，跳过`)
          continue
        }
        try {
          const res = await api.dns.ensure(z.id, relName(h, z.name), target)
          dnsResults.value.push(`${h}: ${res.created ? '已创建 CNAME' : 'CNAME 已存在（指向当前 Tunnel）'}`)
        } catch (e) {
          dnsResults.value.push(`${h}: ${friendlyError(e)}`)
        }
      }
    }
    if (dnsRemoveChecked.value) {
      for (const h of p.remove) {
        const z = zoneFor(h)
        if (!z) continue
        try {
          await api.dns.remove(z.id, relName(h, z.name))
          dnsResults.value.push(`${h}: DNS 记录已删除`)
        } catch (e) {
          dnsResults.value.push(`${h}: ${friendlyError(e)}（请手动处理）`)
        }
      }
    }
    if (dnsResults.value.length === 0) dnsResults.value.push('没有需要执行的 DNS 操作')
  } finally {
    dnsPrompt.value = null
    dnsRunning.value = false
  }
}

function closeDNSResults() {
  dnsResults.value = []
}

async function confirmDeleteDNS() {
  const hs = deletePrompt.value ?? []
  deletePrompt.value = null
  if (!deleteDNSChecked.value || hs.length === 0) return
  const failed: string[] = []
  for (const h of hs) {
    const z = zoneFor(h)
    if (!z) continue
    try {
      await api.dns.remove(z.id, relName(h, z.name))
    } catch {
      failed.push(h)
    }
  }
  if (failed.length) showToast('DNS 记录删除失败，请手动处理：' + failed.join('、'))
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
  const minDelay = new Promise((r) => setTimeout(r, 600))
  try {
    const [rs, zs] = await Promise.all([api.ingress.get(props.tunnelId), api.dns.zones()])
    rules.value = (rs ?? []).map(r => ({ ...r, locked: true, _id: crypto.randomUUID() }))
    zones.value = zs
    snapshot = JSON.stringify((rs ?? []).map(norm))
    savedHosts = hostnamesOf(rs)
  } catch (e) {
    loadError.value = friendlyError(e)
  } finally {
    await minDelay
    loading.value = false
  }
}

function addRule() {
  rules.value.push({ hostname: '', service: '', _id: crypto.randomUUID() })
}

function removeRule(i: number) {
  const h = (rules.value[i].hostname ?? '').trim()
  rules.value.splice(i, 1)
  // immediate save: PUT the new list
  save()
  // issue #6 delete link: ask whether to remove the DNS record too
  if (h) {
    deletePrompt.value = [h]
    deleteDNSChecked.value = true
  }
}

function move(i: number, dir: -1 | 1) {
  const j = i + dir
  if (j < 0 || j >= rules.value.length) return
  const arr = rules.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
  save()
}

async function save() {
  if (!saveable.value) return
  saving.value = true
  saveError.value = ''
  try {
    const oldHosts = savedHosts
    const clean = rules.value.map(({ _id, locked, ...rest }) => rest)
    const got = await api.ingress.save(props.tunnelId, clean)
    const newHosts = hostnamesOf(got)
    rules.value = (got ?? []).map(r => ({ ...r, locked: true, _id: crypto.randomUUID() }))
    snapshot = JSON.stringify((got ?? []).map(norm))
    savedHosts = newHosts
    showToast('配置已保存')
    // issue #6: offer DNS link for added / removed hostnames.
    // Always prompt (even if zones failed to load) so the user knows.
    const create = newHosts.filter((h) => !oldHosts.includes(h))
    const remove = oldHosts.filter((h) => !newHosts.includes(h))
    if (create.length || remove.length) {
      dnsPrompt.value = { create, remove }
      dnsCreateChecked.value = create.length > 0
      dnsRemoveChecked.value = remove.length > 0
    }
    console.log('[Ingress save] oldHosts=', oldHosts, 'newHosts=', newHosts, 'create=', create, 'remove=', remove, 'zones=', zones.value)
  } catch (e) {
    // PUT failed or read-back mismatch → keep user input, show the error
    saveError.value = friendlyError(e)
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
useEscape(() => (confirmLeave.value = false), confirmLeave)

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
          <p class="text-xs text-slate-500">{{ tunnelName }} · {{ tunnelId }}</p>
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

      <div v-else-if="loading" class="space-y-3">
        <p class="text-xs text-slate-400">加载 Ingress 规则…</p>
        <div class="h-12 animate-pulse rounded-xl bg-slate-200" />
        <div class="h-12 animate-pulse rounded-xl bg-slate-200" />
        <div class="h-12 animate-pulse rounded-xl bg-slate-200" />
      </div>

      <template v-else>
        <div v-if="zones.length === 0" class="mb-4 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700">
          当前账户下无 Zone，请先在 Cloudflare 添加域名。hostname 校验将跳过。
        </div>

        <div v-if="rules.length === 0 && !wholeListErrors.length" class="rounded-2xl border border-dashed border-slate-300 p-8 text-center">
          <p class="text-sm text-slate-500">尚未配置规则（Tunnel 未配置或仅有兜底规则）</p>
          <button
            class="mt-4 rounded-lg bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 transition-colors"
            @click="addRule"
          >
            + 添加规则
          </button>
        </div>

        <div class="space-y-2">
          <div
            v-for="(r, i) in rules"
            :key="r._id ?? i"
            class="rounded-2xl border border-slate-200 bg-white p-3"
            :class="{ 'border-red-300': rowErrors(i).length > 0 }"
          >
            <div class="flex items-center gap-2">
              <span class="w-6 text-center text-xs font-semibold text-slate-400">{{ i + 1 }}</span>
              <input
                v-model="r.hostname"
                placeholder="hostname，如 nas.example.com"
                :disabled="r.locked"
                class="flex-1 rounded-md border border-slate-200 px-3 py-1.5 text-sm disabled:bg-slate-100 disabled:text-slate-500 disabled:cursor-not-allowed"
                :title="r.hostname ? '已有域名不可修改，请删除后重新添加' : ''"
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
          class="mt-4 rounded-lg border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50 transition-colors"
          @click="addRule"
        >
          + 添加规则
        </button>
        <p class="mt-2 text-xs text-slate-500">
          保存时自动追加末位 404 兜底规则（http_status:404），无需手动维护。
        </p>
      </template>
    </main>

    <!-- unsaved-changes confirm -->
    <Teleport to="body">
    <div v-if="confirmLeave" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40">
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
    </Teleport>

    <!-- slice 06: DNS link prompt after save -->
    <Teleport to="body">
    <div v-if="dnsPrompt" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40">
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h2 class="text-base font-bold text-slate-900">DNS 联动</h2>
        <p class="mt-1 text-sm text-slate-500">是否同步处理以下域名的 DNS 记录？</p>

        <div v-if="dnsPrompt.create.length" class="mt-4 rounded-lg border border-slate-200 p-3">
          <label class="flex items-center gap-2 text-sm font-medium text-slate-700">
            <input v-model="dnsCreateChecked" type="checkbox" class="accent-blue-600" />
            创建 CNAME（指向 {{ tunnelId }}.cfargotunnel.com）
          </label>
          <ul class="mt-1.5 space-y-0.5">
            <li v-for="h in dnsPrompt.create" :key="'c' + h" class="pl-6 font-mono text-xs text-slate-600">{{ h }}</li>
          </ul>
        </div>

        <div v-if="dnsPrompt.remove.length" class="mt-3 rounded-lg border border-slate-200 p-3">
          <label class="flex items-center gap-2 text-sm font-medium text-slate-700">
            <input v-model="dnsRemoveChecked" type="checkbox" class="accent-blue-600" />
            删除 DNS 记录
          </label>
          <ul class="mt-1.5 space-y-0.5">
            <li v-for="h in dnsPrompt.remove" :key="'r' + h" class="pl-6 font-mono text-xs text-slate-600">{{ h }}</li>
          </ul>
        </div>

        <div class="mt-5 flex justify-end gap-2">
          <button
            :disabled="dnsRunning"
            class="rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            @click="dnsPrompt = null"
          >
            取消
          </button>
          <button
            :disabled="dnsRunning"
            class="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:bg-blue-300"
            @click="runDNSPrompt"
          >
            {{ dnsRunning ? '处理中…' : '确认' }}
          </button>
        </div>
      </div>
    </div>
    </Teleport>

    <!-- DNS operation results -->
    <Teleport to="body">
    <div v-if="dnsResults.length" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40">
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h2 class="text-base font-bold text-slate-900">DNS 操作结果</h2>
        <ul class="mt-3 max-h-64 space-y-1 overflow-y-auto">
          <li v-for="(r, i) in dnsResults" :key="i" class="text-xs text-slate-700">{{ r }}</li>
        </ul>
        <div class="mt-5 flex justify-end">
          <button
            class="rounded-md bg-slate-800 px-4 py-1.5 text-sm font-semibold text-white hover:bg-slate-700"
            @click="closeDNSResults"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
    </Teleport>

    <!-- slice 06: delete-link prompt -->
    <Teleport to="body">
    <div v-if="deletePrompt" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40">
      <div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
        <h2 class="text-base font-bold text-slate-900">同时删除 DNS 记录？</h2>
        <ul class="mt-3 space-y-1">
          <li v-for="h in deletePrompt" :key="h" class="font-mono text-xs text-slate-600">{{ h }}</li>
        </ul>
        <label class="mt-3 flex items-center gap-2 text-sm text-slate-700">
          <input v-model="deleteDNSChecked" type="checkbox" class="accent-blue-600" />
          同时删除对应 DNS 记录
        </label>
        <div class="mt-5 flex justify-end gap-2">
          <button
            class="rounded-md border border-slate-300 px-4 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            @click="deletePrompt = null"
          >
            取消
          </button>
          <button
            class="rounded-md bg-red-600 px-4 py-1.5 text-sm font-semibold text-white hover:bg-red-700"
            @click="confirmDeleteDNS"
          >
            确认
          </button>
        </div>
      </div>
    </div>
    </Teleport>

    <!-- toast -->
    <Teleport to="body">
    <div
      v-if="toast"
      class="fixed right-6 top-6 z-50 max-w-sm rounded-lg bg-slate-800 px-4 py-2.5 text-sm text-white shadow-lg"
    >
      {{ toast }}
    </div>
    </Teleport>
  </div>
</template>
