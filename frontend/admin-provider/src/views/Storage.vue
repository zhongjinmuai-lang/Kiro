<template>
  <div class="page"><h1>存储管理</h1>
    <div class="card">
      <h3>客户存储配额</h3>
      <ul><li v-for="q in list" :key="q.tenant_id">{{ q.name || q.tenant_id }}：{{ (q.used_bytes / 1048576).toFixed(1) }} / {{ (q.max_bytes / 1048576).toFixed(0) }} MB</li><li v-if="!list.length" class="empty">暂无数据</li></ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const list = ref<any[]>([])
onMounted(async () => { try { const r = await api.get('/admin/provider/storage/quotas'); list.value = r.data?.data?.list || [] } catch {} })
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:12px; }
ul { list-style:none;padding:0; } li { padding:10px 0;border-bottom:1px solid #f5f5f5; }
.empty { color:#999;text-align:center;padding:30px; }
</style>
