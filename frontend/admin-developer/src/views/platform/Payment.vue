<template>
  <div class="page">
    <h1>支付中台</h1>
    <p class="hint">开发商顶层集权：统一管理全局支付渠道，授权给服务商使用。</p>
    <div class="card">
      <div class="card-head">
        <h3>全局支付渠道</h3>
        <button @click="alert('生产接入真实 SDK 后启用')">+ 准入新渠道</button>
      </div>
      <ul class="channels">
        <li v-for="c in channels" :key="c.id">
          <span class="name">{{ c.name }}</span>
          <span class="tag">{{ c.type }}</span>
          <span :class="c.status === 1 ? 'ok' : 'ko'">{{ c.status === 1 ? '已启用' : '已下架' }}</span>
        </li>
        <li v-if="!channels.length" class="empty">尚未配置支付渠道（请接入 wechatpay-go / smartwalle/alipay 后启用）</li>
      </ul>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const channels = ref<any[]>([])
onMounted(async () => {
  try { const r = await api.get('/admin/developer/payment/channels'); channels.value = r.data?.data?.list || [] } catch {}
})
</script>
<style scoped>
.page h1 { margin-bottom:8px; } .hint { color:#888;margin-bottom:20px; }
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card-head { display:flex;justify-content:space-between;align-items:center;margin-bottom:16px; }
.card-head button { padding:6px 16px;background:#4a9eff;color:#fff;border:none;border-radius:4px;cursor:pointer; }
.channels { list-style:none;padding:0; } .channels li { padding:12px 0;border-bottom:1px solid #f0f0f0;display:flex;gap:12px;align-items:center; }
.name { flex:1;font-weight:bold; } .tag { background:#f5f5f5;padding:2px 10px;border-radius:4px;font-size:12px; }
.ok { color:#52c41a; } .ko { color:#ff4d4f; } .empty { text-align:center;color:#999;padding:30px; }
</style>
