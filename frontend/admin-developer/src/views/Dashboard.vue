<template>
  <div class="dashboard">
    <h1>控制台</h1>
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-value">{{ stats.tenants }}</div>
        <div class="stat-label">租户总数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.providers }}</div>
        <div class="stat-label">服务商数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.customers }}</div>
        <div class="stat-label">终端客户数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.plugins }}</div>
        <div class="stat-label">运行插件数</div>
      </div>
    </div>

    <div class="panels">
      <div class="panel">
        <h3>平台总览</h3>
        <div class="panel-content">
          <p>开发商：<b>MU平台（mu-platform）</b></p>
          <p>框架版本：<b>{{ info.version || '1.0.0' }}</b></p>
          <p>Runtime：<b>{{ info.runtime || 'Go 1.26.1' }}</b></p>
          <p>数据库：<b>{{ info.database || 'PostgreSQL 18.3' }}</b></p>
        </div>
      </div>
      <div class="panel">
        <h3>快捷操作</h3>
        <div class="quick">
          <router-link to="/tenants">+ 新建服务商</router-link>
          <router-link to="/payment">配置支付渠道</router-link>
          <router-link to="/storage">配置存储源</router-link>
          <router-link to="/notify">管理通知模板</router-link>
          <router-link to="/plugins">插件管理</router-link>
          <router-link to="/agent">查看引擎状态</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const stats = ref({ tenants: 0, providers: 0, customers: 0, plugins: 0 })
const info = ref<any>({})

onMounted(async () => {
  try {
    const r = await api.get('/admin/developer/dashboard/stats')
    stats.value = r.data?.data || stats.value
  } catch {}
  try {
    const r = await api.get('/version')
    info.value = r.data?.data || {}
  } catch {}
})
</script>

<style scoped>
.dashboard h1 { margin-bottom: 24px; color: #333; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
.stat-card { background: #fff; border-radius: 8px; padding: 24px; text-align: center; box-shadow: 0 2px 8px rgba(0,0,0,.06); }
.stat-value { font-size: 32px; font-weight: bold; color: #4a9eff; }
.stat-label { margin-top: 8px; color: #666; }
.panels { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.panel { background: #fff; border-radius: 8px; padding: 20px; box-shadow: 0 2px 8px rgba(0,0,0,.06); }
.panel h3 { margin-bottom: 16px; color: #333; }
.panel-content p { margin: 8px 0; color: #555; }
.quick { display: flex; flex-direction: column; gap: 8px; }
.quick a { padding: 10px 14px; background: #f0f5ff; color: #4a9eff; border-radius: 6px; text-decoration: none; }
.quick a:hover { background: #e0ebff; }
</style>
