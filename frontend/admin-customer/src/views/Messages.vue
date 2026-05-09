<template>
  <div class="page"><h1>消息中心</h1>
    <ul class="msgs">
      <li v-for="m in list" :key="m.id" :class="{ unread: m.status === 0 }">
        <div class="ttl">{{ m.title || '系统通知' }}</div>
        <div class="ct">{{ m.content }}</div>
        <div class="ts">{{ (m.created_at || '').slice(0, 16).replace('T', ' ') }}</div>
      </li>
      <li v-if="!list.length" class="empty">暂无消息</li>
    </ul>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const list = ref<any[]>([])
onMounted(async () => { try { const r = await api.get('/api/v1/messages'); list.value = r.data?.data?.list || [] } catch {} })
</script>
<style scoped>
.msgs { list-style:none;padding:0;background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
li { padding:16px 20px;border-bottom:1px solid #f0f0f0; }
li.unread .ttl { font-weight:bold; }
.ttl { margin-bottom:4px; } .ct { color:#555;font-size:14px; }
.ts { color:#999;font-size:12px;margin-top:4px; }
.empty { text-align:center;color:#999;padding:50px; }
</style>
