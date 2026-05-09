<template>
  <div class="page">
    <div class="header">
      <h1>租户管理</h1>
      <button class="btn-primary" @click="showCreate = true">+ 新增服务商</button>
    </div>
    <table class="tbl">
      <thead>
        <tr><th>编码</th><th>名称</th><th>层级</th><th>状态</th><th>创建时间</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="t in list" :key="t.id">
          <td>{{ t.code }}</td>
          <td>{{ t.name }}</td>
          <td><span class="tag">{{ t.level }}</span></td>
          <td><span :class="t.status === 1 ? 'ok' : 'ko'">{{ t.status === 1 ? '启用' : '禁用' }}</span></td>
          <td>{{ (t.created_at || '').slice(0,10) }}</td>
          <td>
            <a @click="toggle(t)">{{ t.status === 1 ? '禁用' : '启用' }}</a>
            <a @click="del(t)" style="color:#f5222d">删除</a>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="6" class="empty">暂无租户</td></tr>
      </tbody>
    </table>

    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="dlg">
        <h3>新增服务商</h3>
        <label>名称<input v-model="form.name" /></label>
        <label>编码<input v-model="form.code" placeholder="唯一编码，英文/数字" /></label>
        <div class="btns">
          <button @click="showCreate = false">取消</button>
          <button class="btn-primary" @click="create">确认</button>
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
  try {
    const r = await api.get('/admin/developer/providers')
    list.value = r.data?.data?.list || []
  } catch { list.value = [] }
}
async function create() {
  try {
    await api.post('/admin/developer/providers', form.value)
    showCreate.value = false
    form.value = { name: '', code: '' }
    load()
  } catch (e: any) {
    alert(e.response?.data?.message || '创建失败')
  }
}
async function toggle(t: any) {
  await api.put(`/admin/developer/providers/${t.id}/status`, { status: t.status === 1 ? 0 : 1 })
  load()
}
async function del(t: any) {
  if (!confirm(`确认删除 ${t.name} ？此操作将级联删除下属所有客户`)) return
  await api.delete(`/admin/developer/providers/${t.id}`)
  load()
}
onMounted(load)
</script>

<style scoped>
.page h1 { margin:0; }
.header { display:flex;justify-content:space-between;align-items:center;margin-bottom:20px; }
.btn-primary { padding:8px 18px;background:#4a9eff;color:#fff;border:none;border-radius:6px;cursor:pointer; }
.tbl { width:100%;background:#fff;border-radius:8px;border-collapse:collapse;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.tbl th, .tbl td { padding:12px 16px;text-align:left;border-bottom:1px solid #f0f0f0; }
.tbl th { background:#fafafa;color:#555; }
.tag { background:#f0f5ff;color:#4a9eff;padding:2px 8px;border-radius:4px;font-size:12px; }
.ok { color:#52c41a; } .ko { color:#ff4d4f; }
a { color:#4a9eff;cursor:pointer;margin-right:12px; }
.empty { text-align:center;color:#999;padding:40px; }
.modal { position:fixed;inset:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center;z-index:1000; }
.dlg { background:#fff;border-radius:12px;padding:24px;width:400px; }
.dlg h3 { margin-bottom:16px; }
.dlg label { display:block;margin-bottom:12px;color:#555; }
.dlg input { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px;margin-top:4px; }
.btns { display:flex;gap:12px;justify-content:flex-end;margin-top:20px; }
.btns button { padding:8px 18px;background:#f0f0f0;border:none;border-radius:6px;cursor:pointer; }
</style>
