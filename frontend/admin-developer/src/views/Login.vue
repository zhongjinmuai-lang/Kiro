<template>
  <div class="login-page">
    <div class="login-card">
      <h2>MU 开发商总后台</h2>
      <p class="subtitle">自研全能智能体主体框架 · 顶层集权</p>
      <form @submit.prevent="handleLogin">
        <div class="form-group">
          <label>开发商编码</label>
          <input v-model="form.tenantCode" type="text" placeholder="默认 mu-platform" required />
        </div>
        <div class="form-group">
          <label>用户名</label>
          <input v-model="form.username" type="text" placeholder="默认 admin" required />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input v-model="form.password" type="password" placeholder="默认 mu_admin_2026" required />
        </div>
        <button type="submit" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
        <p v-if="error" class="error">{{ error }}</p>
        <p class="hint">
          默认账号：<code>mu-platform / admin / mu_admin_2026</code>
        </p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const form = ref({ tenantCode: 'mu-platform', username: 'admin', password: '' })
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    await authStore.login(form.value.tenantCode, form.value.username, form.value.password)
    router.push('/dashboard')
  } catch (e: any) {
    error.value = e.response?.data?.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
}
.login-card {
  background: #fff;
  border-radius: 12px;
  padding: 40px;
  width: 420px;
  box-shadow: 0 10px 40px rgba(0,0,0,.3);
}
.login-card h2 { text-align: center; color: #333; margin-bottom: 4px; }
.subtitle { text-align: center; color: #999; margin-bottom: 28px; font-size: 13px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; color: #555; font-size: 13px; }
.form-group input {
  width: 100%; padding: 10px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px;
}
button {
  width: 100%; padding: 12px; background: #4a9eff; color: #fff; border: none;
  border-radius: 6px; font-size: 16px; cursor: pointer; margin-top: 8px;
}
button:hover { background: #3a8eef; }
button:disabled { background: #ccc; cursor: not-allowed; }
.error { color: #f5222d; text-align: center; margin-top: 12px; }
.hint { text-align: center; margin-top: 16px; color: #999; font-size: 12px; }
.hint code { background: #f0f5ff; color: #4a9eff; padding: 2px 8px; border-radius: 4px; }
</style>
