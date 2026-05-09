<template>
  <div class="page">
    <div class="hdr"><h1>文件管理</h1><button @click="alert('存储中台已就绪，请在服务端接入真实驱动后启用')">上传文件</button></div>
    <div class="quota">配额：{{ quotaText }}</div>
    <table>
      <tr><th>文件名</th><th>大小</th><th>上传时间</th><th>操作</th></tr>
      <tr v-for="f in files" :key="f.id">
        <td>{{ f.file_name }}</td>
        <td>{{ (f.file_size / 1024).toFixed(1) }} KB</td>
        <td>{{ (f.created_at || '').slice(0,16).replace('T',' ') }}</td>
        <td><a :href="f.url" target="_blank">下载</a></td>
      </tr>
      <tr v-if="!files.length"><td colspan="4" class="empty">暂无文件</td></tr>
    </table>
  </div>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/utils/request'
const files = ref<any[]>([])
const quota = ref({ used_bytes: 0, max_bytes: 0 })
const quotaText = computed(() => `${(quota.value.used_bytes / 1048576).toFixed(1)} / ${(quota.value.max_bytes / 1048576).toFixed(0)} MB`)
onMounted(async () => {
  try { const r = await api.get('/api/v1/storage/files'); files.value = r.data?.data?.list || [] } catch {}
  try { const r = await api.get('/api/v1/storage/quota'); quota.value = r.data?.data || quota.value } catch {}
})
</script>
<style scoped>
.hdr { display:flex;justify-content:space-between;align-items:center;margin-bottom:16px; }
button { padding:8px 18px;background:#2d3436;color:#fff;border:none;border-radius:6px;cursor:pointer; }
.quota { margin-bottom:16px;background:#fff;padding:12px 16px;border-radius:6px; }
table { width:100%;background:#fff;border-radius:8px;border-collapse:collapse;box-shadow:0 2px 8px rgba(0,0,0,.06); }
th, td { padding:12px;text-align:left;border-bottom:1px solid #f0f0f0; }
th { background:#fafafa; } a { color:#2d3436; }
.empty { text-align:center;color:#999;padding:30px; }
</style>
