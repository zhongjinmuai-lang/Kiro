<template>
  <div class="dashboard">
    <h1>服务商控制台</h1>
    <div class="stats">
      <div class="card"><div class="v">{{ stats.customers }}</div><div class="l">下属客户</div></div>
      <div class="card"><div class="v">¥{{ (stats.revenue || 0).toFixed(2) }}</div><div class="l">本月收入</div></div>
      <div class="card"><div class="v">{{ stats.orders }}</div><div class="l">订单总数</div></div>
      <div class="card"><div class="v">{{ stats.messages }}</div><div class="l">消息推送</div></div>
    </div>
    <div class="panel">
      <h3>快捷操作</h3>
      <div class="quick">
        <router-link to="/customers">新增客户</router-link>
        <router-link to="/payment">绑定商户号</router-link>
        <router-link to="/storage">分配存储配额</router-link>
        <router-link to="/notify">配置通知模板</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const stats = ref<any>({ customers: 0, revenue: 0, orders: 0, messages: 0 })
onMounted(async () => {
  try { const r = await api.get('/admin/provider/dashboard/stats'); if (r.data?.data) stats.value = r.data.data } catch {}
})
</script>

<style scoped>
.dashboard h1 { margin-bottom:24px; color:#333; }
.stats { display:grid; grid-template-columns:repeat(4,1fr); gap:16px; margin-bottom:24px; }
.card { background:#fff; border-radius:8px; padding:24px; text-align:center; box-shadow:0 2px 8px rgba(0,0,0,.06); }
.v { font-size:28px; font-weight:bold; color:#0f3460; } .l { margin-top:8px; color:#666; }
.panel { background:#fff; border-radius:8px; padding:20px; box-shadow:0 2px 8px rgba(0,0,0,.06); }
.panel h3 { margin-bottom:16px; }
.quick { display:flex; gap:12px; flex-wrap:wrap; }
.quick a { padding:10px 18px; background:#f0f5ff; color:#0f3460; border-radius:6px; text-decoration:none; }
.quick a:hover { background:#e0ebff; }
</style>
