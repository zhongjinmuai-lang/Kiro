<template>
  <div class="page">
    <h1>系统设置</h1>
    <div class="card">
      <h3>MU 框架信息</h3>
      <ul>
        <li>版本：<b>{{ info.version || '1.0.0' }}</b></li>
        <li>环境：<b>{{ info.env || 'prod' }}</b></li>
        <li>运行时：<b>{{ info.runtime || 'Go 1.26.1' }}</b></li>
        <li>数据库：<b>{{ info.database || 'PostgreSQL 18.3' }}</b></li>
        <li>框架：<b>{{ info.framework || 'MU Framework' }}</b></li>
      </ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const info = ref<any>({})
onMounted(async () => { try { const r = await api.get('/version'); info.value = r.data?.data || {} } catch {} })
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06);max-width:600px; }
.card h3 { margin-bottom:16px; }
ul { list-style:none;padding:0; } li { padding:8px 0;border-bottom:1px solid #f5f5f5; }
</style>
