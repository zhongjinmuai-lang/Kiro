<template>
  <div class="page"><h1>支付配置</h1>
    <div class="card">
      <h3>可用支付渠道（开发商已授权）</h3>
      <ul><li v-for="c in channels" :key="c.id">{{ c.name }} · {{ c.type }}</li><li v-if="!channels.length" class="empty">暂无可用渠道</li></ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const channels = ref<any[]>([])
onMounted(async () => { try { const r = await api.get('/admin/provider/payment/channels'); channels.value = r.data?.data?.list || [] } catch {} })
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:12px; }
ul { list-style:none;padding:0; } li { padding:10px 0;border-bottom:1px solid #f5f5f5; }
.empty { color:#999;text-align:center;padding:30px; }
</style>
