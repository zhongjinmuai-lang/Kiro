<template>
  <div class="page"><h1>通知管理</h1>
    <div class="card">
      <h3>可用通知模板</h3>
      <ul><li v-for="t in list" :key="t.id">{{ t.name }}</li><li v-if="!list.length" class="empty">暂无模板</li></ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const list = ref<any[]>([])
onMounted(async () => { try { const r = await api.get('/admin/provider/notify/templates'); list.value = r.data?.data?.list || [] } catch {} })
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:12px; }
ul { list-style:none;padding:0; } li { padding:10px 0;border-bottom:1px solid #f5f5f5; }
.empty { color:#999;text-align:center;padding:30px; }
</style>
