<template>
  <div id="mu-provider">
    <aside class="sidebar" v-if="isAuthenticated">
      <div class="logo">
        <h2>MU 服务商后台</h2>
      </div>
      <nav class="menu">
        <router-link to="/dashboard">控制台</router-link>
        <router-link to="/customers">客户管理</router-link>
        <router-link to="/payment">支付配置</router-link>
        <router-link to="/storage">存储管理</router-link>
        <router-link to="/notify">通知管理</router-link>
        <router-link to="/permissions">权限管理</router-link>
        <router-link to="/settings">系统设置</router-link>
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
#mu-provider { display: flex; min-height: 100vh; font-family: -apple-system, sans-serif; }
.sidebar { width: 220px; background: #0f3460; color: #fff; padding: 20px 0; }
.logo { padding: 0 20px 20px; border-bottom: 1px solid #1a4a7a; }
.logo h2 { font-size: 16px; }
.menu { padding: 20px 0; }
.menu a { display: block; padding: 12px 20px; color: #bbb; text-decoration: none; }
.menu a:hover, .menu a.router-link-active { background: #1a4a7a; color: #fff; }
.content { flex: 1; padding: 24px; background: #f5f7fa; }
</style>
