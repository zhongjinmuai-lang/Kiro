<template>
  <div class="page"><h1>系统设置</h1>
    <div class="card">
      <h3>品牌定制（贴牌）</h3>
      <div class="row"><label>服务商名称</label><input v-model="info.name" /></div>
      <div class="row"><label>主题色</label><input v-model="info.themeColor" type="color" /></div>
      <div class="row"><label>登录页公告</label><textarea v-model="info.notice" rows="3" /></div>
      <button @click="save">保存</button>
      <span v-if="saved" class="saved">已保存 ✓</span>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@/utils/request'
const info = ref<any>({ name:'', themeColor:'#0f3460', notice:'' })
const saved = ref(false)
async function save() { try { await api.put('/admin/provider/settings', info.value) } catch {}; saved.value = true; setTimeout(() => (saved.value = false), 2000) }
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06);max-width:600px; }
.card h3 { margin-bottom:16px; }
.row { margin-bottom:16px; } label { display:block;margin-bottom:6px;color:#555; }
input, textarea { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px; }
button { padding:8px 20px;background:#0f3460;color:#fff;border:none;border-radius:4px;cursor:pointer; }
.saved { color:#52c41a;margin-left:12px; }
</style>
