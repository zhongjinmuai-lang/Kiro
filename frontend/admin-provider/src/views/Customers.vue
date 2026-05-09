<template>
  <div class="page">
    <div class="header">
      <h1>客户管理</h1>
      <button @click="showCreate = true">+ 新增客户</button>
    </div>
    <table class="tbl">
      <thead><tr><th>编码</th><th>名称</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in list" :key="c.id">
          <td>{{ c.code }}</td><td>{{ c.name }}</td>
          <td><span :class="c.status === 1 ? 'ok' : 'ko'">{{ c.status === 1 ? '启用' : '禁用' }}</span></td>
          <td>{{ (c.created_at || '').slice(0, 10) }}</td>
          <td><a @click="toggle(c)">{{ c.status === 1 ? '禁用' : '启用' }}</a></td>
        </tr>
        <tr v-if="!list.length"><td colspan="5" class="empty">暂无客户</td></tr>
      </tbody>
    </table>
    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="dlg">
        <h3>新增终端客户</h3>
        <label>家族名称<input v-model="form.name" /></label>
        <label>家族编码<input v-model="form.code" placeholder="唯一编码" /></label>
        <div class="btns">
          <button @click="showCreate = false">取消</button>
          <button class="primary" @click="create">确认</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const list = ref<any[]>([])
const showCreate = ref(false)
const form = ref({ name: '', code: '' })

async function load() {
  try { const r = await api.get('/admin/provider/customers'); list.value = r.data?.data?.list || [] }
  catch { list.value = [] }
}
async function create() {
  try {
    await api.post('/admin/provider/customers', form.value)
    showCreate.value = false
    form.value = { name: '', code: '' }
    load()
  } catch (e: any) { alert(e.response?.data?.message || '创建失败') }
}
async function toggle(c: any) {
  await api.put(`/admin/provider/customers/${c.id}/status`, { status: c.status === 1 ? 0 : 1 })
  load()
}
onMounted(load)
</script>

<style scoped>
.header { display:flex;justify-content:space-between;align-items:center;margin-bottom:20px; }
.header button { padding:8px 18px;background:#0f3460;color:#fff;border:none;border-radius:6px;cursor:pointer; }
.tbl { width:100%;background:#fff;border-radius:8px;border-collapse:collapse;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.tbl th, .tbl td { padding:12px 16px;text-align:left;border-bottom:1px solid #f0f0f0; }
.tbl th { background:#fafafa;color:#555; }
.ok { color:#52c41a; } .ko { color:#ff4d4f; }
a { color:#0f3460;cursor:pointer; }
.empty { text-align:center;color:#999;padding:40px; }
.modal { position:fixed;inset:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center; }
.dlg { background:#fff;border-radius:12px;padding:24px;width:400px; }
.dlg h3 { margin-bottom:16px; }
.dlg label { display:block;margin-bottom:12px;color:#555; }
.dlg input { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px;margin-top:4px; }
.btns { display:flex;gap:12px;justify-content:flex-end;margin-top:20px; }
.btns button { padding:8px 18px;background:#f0f0f0;border:none;border-radius:6px;cursor:pointer; }
.btns button.primary { background:#0f3460;color:#fff; }
</style>
