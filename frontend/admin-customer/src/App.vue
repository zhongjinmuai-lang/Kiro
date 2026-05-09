<template>
  <div id="mu-customer">
    <aside class="sidebar" v-if="isAuthenticated">
      <div class="logo">
        <h2>MU 业务后台</h2>
      </div>
      <nav class="menu">
        <router-link to="/dashboard">工作台</router-link>
        <router-link to="/orders">订单管理</router-link>
        <router-link to="/files">文件管理</router-link>
        <router-link to="/messages">消息中心</router-link>
        <router-link to="/account">账户设置</router-link>
      </nav>
    </aside>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
#mu-customer { display: flex; min-height: 100vh; font-family: -apple-system, sans-serif; }
.sidebar { width: 200px; background: #2d3436; color: #fff; padding: 20px 0; }
.logo { padding: 0 20px 20px; border-bottom: 1px solid #444; }
.logo h2 { font-size: 16px; }
.menu { padding: 20px 0; }
.menu a { display: block; padding: 12px 20px; color: #bbb; text-decoration: none; }
.menu a:hover, .menu a.router-link-active { background: #444; color: #fff; }
.content { flex: 1; padding: 24px; background: #f5f7fa; }
</style>
