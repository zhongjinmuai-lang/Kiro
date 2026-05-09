<template>
  <div id="mu-admin">
    <aside class="sidebar" v-if="isAuthenticated">
      <div class="logo">
        <h2>MU 开发商后台</h2>
        <small>顶层集权 · 全局管控</small>
      </div>
      <nav class="menu">
        <router-link to="/dashboard">📊 控制台</router-link>
        <router-link to="/tenants">🏢 租户管理</router-link>
        <router-link to="/payment">💰 支付中台</router-link>
        <router-link to="/storage">📦 存储中台</router-link>
        <router-link to="/notify">📮 通知中台</router-link>
        <router-link to="/plugins">🧩 插件管理</router-link>
        <router-link to="/agent">🤖 智能体引擎</router-link>
        <router-link to="/settings">⚙️ 系统设置</router-link>
      </nav>
      <button class="logout" @click="doLogout">退出登录</button>
    </aside>
    <main class="content"><router-view /></main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

const authStore = useAuthStore()
const router = useRouter()
const isAuthenticated = computed(() => authStore.isAuthenticated)

async function doLogout() { await authStore.logout(); router.push('/login') }
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
#mu-admin { display: flex; min-height: 100vh; font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif; }
.sidebar { width: 240px; background: #1a1a2e; color: #fff; display: flex; flex-direction: column; }
.logo { padding: 20px; border-bottom: 1px solid #333; }
.logo h2 { font-size: 18px; } .logo small { color: #888; font-size: 12px; }
.menu { flex: 1; padding: 16px 0; }
.menu a { display: block; padding: 12px 20px; color: #ccc; text-decoration: none; transition: all .2s; }
.menu a:hover, .menu a.router-link-active { background: #16213e; color: #fff; border-left: 3px solid #4a9eff; }
.logout { margin: 16px; padding: 10px; background: transparent; color: #aaa; border: 1px solid #333; border-radius: 4px; cursor: pointer; }
.logout:hover { background: #16213e; color: #fff; }
.content { flex: 1; padding: 24px; background: #f5f7fa; overflow: auto; }
</style>
