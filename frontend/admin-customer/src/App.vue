<template>
  <div id="mu-customer">
    <aside class="sidebar" v-if="isAuth">
      <div class="logo"><h2>MU 家族后台</h2><small>终端客户 · 族谱管理</small></div>
      <nav class="menu">
        <router-link to="/dashboard">📊 工作台</router-link>
        <router-link to="/genealogy">🌳 族谱可视化</router-link>
        <router-link to="/files">📁 文件管理</router-link>
        <router-link to="/messages">💬 消息中心</router-link>
        <router-link to="/account">👤 账户设置</router-link>
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
#mu-customer { display:flex; min-height:100vh; font-family:-apple-system,BlinkMacSystemFont,'PingFang SC','Microsoft YaHei',sans-serif; }
.sidebar { width:220px; background:#2d3436; color:#fff; display:flex; flex-direction:column; }
.logo { padding:20px; border-bottom:1px solid #444; }
.logo h2 { font-size:16px; } .logo small { color:#888; font-size:12px; }
.menu { flex:1; padding:16px 0; }
.menu a { display:block; padding:12px 20px; color:#bbb; text-decoration:none; }
.menu a:hover, .menu a.router-link-active { background:#444; color:#fff; border-left:3px solid #636e72; }
.logout { margin:16px; padding:10px; background:transparent; color:#bbb; border:1px solid #555; border-radius:4px; cursor:pointer; }
.content { flex:1; padding:24px; background:#f5f7fa; overflow:auto; }
</style>
