<template>
  <div class="login-page">
    <div class="card">
      <h2>MU 服务商 SaaS 后台</h2>
      <p class="sub">二级管控 · 经营客户 · 品牌定制</p>
      <form @submit.prevent="onSubmit">
        <input v-model="form.tenantCode" placeholder="服务商编码（默认 demo-provider）" required />
        <input v-model="form.username" placeholder="用户名（默认 admin）" required />
        <input v-model="form.password" placeholder="密码（默认 mu_admin_2026）" type="password" required />
        <button :disabled="loading">{{ loading ? '登录中…' : '登录' }}</button>
        <p v-if="err" class="err">{{ err }}</p>
        <p class="hint">默认账号：<code>demo-provider / admin / mu_admin_2026</code></p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const store = useAuthStore()
const form = ref({ tenantCode: 'demo-provider', username: 'admin', password: '' })
const loading = ref(false)
const err = ref('')

async function onSubmit() {
  loading.value = true; err.value = ''
  try {
    await store.login(form.value.tenantCode, form.value.username, form.value.password)
    router.push('/dashboard')
  } catch (e: any) {
    err.value = e.response?.data?.message || '登录失败'
  } finally { loading.value = false }
}
</script>

<style scoped>
.login-page { display:flex; align-items:center; justify-content:center; min-height:100vh; background:linear-gradient(135deg,#0f3460,#16213e); }
.card { background:#fff; border-radius:12px; padding:40px; width:420px; box-shadow:0 10px 40px rgba(0,0,0,.3); }
h2 { text-align:center; color:#333; margin-bottom:4px; }
.sub { text-align:center; color:#888; margin-bottom:24px; font-size:13px; }
input { width:100%; padding:10px 12px; margin-bottom:14px; border:1px solid #ddd; border-radius:6px; font-size:14px; }
button { width:100%; padding:12px; background:#0f3460; color:#fff; border:none; border-radius:6px; font-size:16px; cursor:pointer; }
button:disabled { opacity:.6; }
.err { color:#f5222d; text-align:center; margin-top:10px; }
.hint { text-align:center; color:#999; font-size:12px; margin-top:14px; }
.hint code { background:#f0f5ff; color:#0f3460; padding:2px 8px; border-radius:4px; }
</style>
