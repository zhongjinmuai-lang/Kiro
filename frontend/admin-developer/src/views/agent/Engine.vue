<template>
  <div class="page">
    <h1>智能体引擎</h1>
    <div class="stats">
      <div class="card"><div class="v">{{ stats.total_tasks }}</div><div class="l">总任务</div></div>
      <div class="card"><div class="v">{{ stats.completed_tasks }}</div><div class="l">已完成</div></div>
      <div class="card"><div class="v">{{ stats.active_workers }}</div><div class="l">活跃Worker</div></div>
      <div class="card"><div class="v">{{ stats.queue_size }}</div><div class="l">队列深度</div></div>
    </div>
    <div class="panel">
      <h3>进化历史</h3>
      <ul>
        <li v-for="e in events" :key="e.id">
          [{{ e.strategy }}] {{ e.target }} - <span :class="e.success ? 'ok' : 'ko'">{{ e.result }}</span>
        </li>
        <li v-if="!events.length" class="empty">暂无进化事件</li>
      </ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const stats = ref<any>({ total_tasks:0, completed_tasks:0, active_workers:0, queue_size:0 })
const events = ref<any[]>([])
onMounted(async () => {
  try { const r = await api.get('/agent/stats'); stats.value = r.data?.data || stats.value } catch {}
  try { const r = await api.get('/agent/evolution/events'); events.value = r.data?.data || [] } catch {}
})
</script>
<style scoped>
.stats { display:grid;grid-template-columns:repeat(4,1fr);gap:16px;margin-bottom:20px; }
.card { background:#fff;border-radius:8px;padding:24px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.v { font-size:28px;font-weight:bold;color:#4a9eff; } .l { margin-top:8px;color:#666; }
.panel { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.panel h3 { margin-bottom:12px; }
ul { list-style:none;padding:0; } li { padding:10px 0;border-bottom:1px solid #f5f5f5; }
.ok { color:#52c41a; } .ko { color:#f5222d; } .empty { color:#999;text-align:center; }
</style>
