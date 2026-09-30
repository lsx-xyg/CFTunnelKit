<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { GetAuthState, RetryVerify } from '../wailsjs/go/main/App'
import { auth } from '../wailsjs/go/models'
import AuthView from './views/AuthView.vue'
import MainView from './views/MainView.vue'

const state = ref<auth.State>(new auth.State({}))
const booting = ref(true)

async function refresh() {
  state.value = await GetAuthState()
  // A persisted token that is neither verified nor offline needs a restore
  // attempt (startup path). RetryVerify is user-triggerable and safe to call.
  if (
    state.value.has_token &&
    !state.value.authenticated &&
    !state.value.offline &&
    !state.value.message
  ) {
    try {
      await RetryVerify()
    } catch (e) {
      // Restore failure surfaces via the state / auth view message.
      console.error('restore failed', e)
    }
    state.value = await GetAuthState()
  }
  booting.value = false
}

onMounted(refresh)
</script>

<template>
  <div class="h-full">
    <div v-if="booting" class="flex h-full items-center justify-center">
      <p class="text-sm text-slate-400">正在启动…</p>
    </div>
    <AuthView v-else-if="!state.authenticated" :initial-message="state.message" @verified="refresh" />
    <MainView v-else :state="state" @refresh="refresh" />
  </div>
</template>
