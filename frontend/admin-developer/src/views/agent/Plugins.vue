<template>
  <div class="page">
    <h1>插件管理</h1>
    <div class="card">
      <h3>已安装插件</h3>
      <ul class="plugins">
        <li v-for="p in plugins" :key="p.meta?.id || p.id">
          <div>
            <div class="name">{{ p.meta?.name || p.name }} <small>v{{ p.meta?.version || p.version }}</small></div>
            <div class="desc">{{ p.meta?.description || p.description }}</div>
          </div>
          <span class="status" :class="p.status">{{ p.status }}</span>
        </li>
        <li v-if="!plugins.length" class="empty">暂无插件，请在代码中通过 plugin.Manager.Install() 加载</li>
      </ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const plugins = ref<any[]>([])
onMounted(async () => {
  try { const r = await api.get('/agent/plugins'); plugins.value = r.data?.data || [] } catch {}
})
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:16px; }
.plugins { list-style:none;padding:0; }
.plugins li { display:flex;justify-content:space-between;align-items:center;padding:12px 0;border-bottom:1px solid #f5f5f5; }
.name { font-weight:bold; } .name small { color:#888;font-size:12px;margin-left:8px; }
.desc { color:#888;font-size:13px;margin-top:4px; }
.status { padding:2px 10px;border-radius:4px;font-size:12px; }
.status.running { background:#f6ffed;color:#52c41a; }
.status.loaded  { background:#e6f7ff;color:#1890ff; }
.status.stopped { background:#fff1f0;color:#f5222d; }
.empty { color:#999;text-align:center;padding:30px; }
</style>
