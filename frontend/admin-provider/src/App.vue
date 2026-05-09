<template>
  <div id="mu-provider">
    <aside class="sidebar" v-if="isAuth">
      <div class="logo"><h2>MU 服务商后台</h2><small>二级管控 · SaaS 代理</small></div>
      <nav class="menu">
        <router-link to="/dashboard">📊 控制台</router-link>
        <router-link to="/customers">👥 客户管理</router-link>
        <router-link to="/payment">💳 支付配置</router-link>
        <router-link to="/storage">📦 存储管理</router-link>
        <router-link to="/notify">📬 通知管理</router-link>
        <router-link to="/permissions">🛡️ 权限管理</router-link>
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

const store = useAuthStore()
const router = useRouter()
const isAuth = computed(() => store.isAuthenticated)

async function doLogout() { await store.logout(); router.push('/login') }
</script>

<style>
* { margin:0; padding:0; box-sizing:border-box; }
#mu-provider { display:flex; min-height:100vh; font-family:-apple-system,BlinkMacSystemFont,'PingFang SC','Microsoft YaHei',sans-serif; }
.sidebar { width:220px; background:#0f3460; color:#fff; display:flex; flex-direction:column; }
.logo { padding:20px; border-bottom:1px solid #1a4a7a; }
.logo h2 { font-size:16px; } .logo small { color:#88a; font-size:12px; }
.menu { flex:1; padding:16px 0; }
.menu a { display:block; padding:12px 20px; color:#bbb; text-decoration:none; }
.menu a:hover, .menu a.router-link-active { background:#1a4a7a; color:#fff; }
.logout { margin:16px; padding:10px; background:transparent; color:#aaa; border:1px solid #1a4a7a; border-radius:4px; cursor:pointer; }
.content { flex:1; padding:24px; background:#f5f7fa; overflow:auto; }
</style>
