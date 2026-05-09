<template>
  <div class="dashboard">
    <h1>工作台</h1>
    <div class="stats">
      <div class="card"><div class="v">{{ stats.members }}</div><div class="l">族谱成员</div></div>
      <div class="card"><div class="v">{{ stats.branches }}</div><div class="l">分支数</div></div>
      <div class="card"><div class="v">{{ stats.generations }}</div><div class="l">最大世代</div></div>
      <div class="card"><div class="v">{{ stats.announces }}</div><div class="l">公告</div></div>
    </div>
    <div class="grid">
      <div class="panel">
        <h3>最新家族公告</h3>
        <ul>
          <li v-for="a in announces" :key="a.id">
            <span class="ttl">{{ a.title }}</span>
            <span class="ts">{{ (a.publish_at || '').slice(0, 10) }}</span>
          </li>
          <li v-if="!announces.length" class="empty">暂无公告</li>
        </ul>
      </div>
      <div class="panel">
        <h3>快捷操作</h3>
        <div class="quick">
          <router-link to="/genealogy">族谱可视化</router-link>
          <router-link to="/files">上传文件</router-link>
          <router-link to="/messages">消息中心</router-link>
          <router-link to="/account">账户设置</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const stats = ref<any>({ members: 0, branches: 0, generations: 0, announces: 0 })
const announces = ref<any[]>([])

onMounted(async () => {
  try { const s = await api.get('/api/v1/genealogy/stats'); Object.assign(stats.value, s.data?.data || {}) } catch {}
  try {
    const a = await api.get('/api/v1/genealogy/announces?page=1&page_size=5')
    announces.value = a.data?.data?.list || []
    stats.value.announces = a.data?.data?.total || announces.value.length
  } catch {}
})
</script>

<style scoped>
.dashboard h1 { margin-bottom:24px; }
.stats { display:grid;grid-template-columns:repeat(4,1fr);gap:16px;margin-bottom:24px; }
.card { background:#fff;border-radius:8px;padding:24px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.v { font-size:26px;font-weight:bold;color:#2d3436; } .l { margin-top:8px;color:#666; }
.grid { display:grid;grid-template-columns:1fr 1fr;gap:16px; }
.panel { background:#fff;border-radius:8px;padding:20px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.panel h3 { margin-bottom:12px; }
ul { list-style:none;padding:0; }
li { padding:8px 0;border-bottom:1px solid #f5f5f5;display:flex;justify-content:space-between; }
.ttl { flex:1; } .ts { color:#999;font-size:13px; }
.empty { color:#999;text-align:center; }
.quick { display:flex;flex-direction:column;gap:8px; }
.quick a { padding:10px 14px;background:#f5f5f5;color:#2d3436;border-radius:6px;text-decoration:none; }
.quick a:hover { background:#e8e8e8; }
</style>
