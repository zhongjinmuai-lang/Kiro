<template>
  <div class="page"><h1>权限管理</h1>
    <div class="card">
      <h3>本服务商权限</h3>
      <ul><li v-for="p in perms" :key="p">{{ p }}</li><li v-if="!perms.length" class="empty">暂无权限</li></ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const perms = ref<string[]>([])
onMounted(async () => { try { const r = await api.get('/admin/provider/permissions'); perms.value = r.data?.data || [] } catch {} })
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:12px; }
ul { list-style:none;padding:0;display:flex;flex-wrap:wrap;gap:8px; }
li { background:#f0f5ff;color:#0f3460;padding:6px 12px;border-radius:4px;font-family:monospace;font-size:12px; }
.empty { color:#999; }
</style>
