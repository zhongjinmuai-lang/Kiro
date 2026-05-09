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
        <h3>智能体引擎状态</h3>
        <div class="panel-content">
          <p>运行状态：<span class="status-ok">正常</span></p>
          <p>活跃工作数：{{ agentStats.activeWorkers }}</p>
          <p>队列深度：{{ agentStats.queueSize }}</p>
          <p>运行时间：{{ agentStats.uptime }}</p>
        </div>
      </div>
      <div class="panel">
        <h3>近期进化事件</h3>
        <div class="panel-content">
          <p v-for="event in recentEvents" :key="event.id">
            [{{ event.strategy }}] {{ event.target }} - {{ event.result }}
          </p>
          <p v-if="!recentEvents.length" class="empty">暂无进化事件</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const stats = ref({ tenants: 0, providers: 0, customers: 0, plugins: 0 })
const agentStats = ref({ activeWorkers: 0, queueSize: 0, uptime: '0s' })
const recentEvents = ref<any[]>([])

onMounted(async () => {
  try {
    const [statsRes, agentRes] = await Promise.all([
      api.get('/api/v1/dashboard/stats'),
      api.get('/admin/v1/agent/status'),
    ])
    stats.value = statsRes.data.data || stats.value
    agentStats.value = agentRes.data.data || agentStats.value
  } catch (e) {
    console.warn('数据加载失败，使用默认值')
  }
})
</script>

<style scoped>
.dashboard h1 { margin-bottom: 24px; color: #333; }

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  text-align: center;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}

.stat-value { font-size: 32px; font-weight: bold; color: #4a9eff; }
.stat-label { margin-top: 8px; color: #666; }

.panels { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }

.panel {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.panel h3 { margin-bottom: 16px; color: #333; }
.panel-content p { margin: 8px 0; color: #555; }

.status-ok { color: #52c41a; font-weight: bold; }
.empty { color: #999; }
</style>
