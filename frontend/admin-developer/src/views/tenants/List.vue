<template>
  <div class="page">
    <div class="header">
      <h1>租户管理</h1>
      <button class="btn-primary" @click="showCreate = true">+ 新增服务商（开账户）</button>
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
            <a @click="openReset(t)">重置密码</a>
            <a @click="del(t)" style="color:#f5222d">删除</a>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="6" class="empty">暂无租户</td></tr>
      </tbody>
    </table>

    <!-- 新增服务商弹窗 -->
    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="dlg">
        <h3>新增服务商（自动开通账号）</h3>
        <div class="section">服务商基本信息</div>
        <label>名称<input v-model="form.name" /></label>
        <label>编码<input v-model="form.code" placeholder="唯一编码，如 provider-001" /></label>
        <div class="section">初始管理员账号（交付给服务商登录用）</div>
        <label>用户名<input v-model="form.admin_username" placeholder="默认 admin" /></label>
        <label>密码<input v-model="form.admin_password" type="password" placeholder="至少 6 位" /></label>
        <label>昵称<input v-model="form.admin_nickname" placeholder="可选" /></label>
        <label>邮箱<input v-model="form.admin_email" placeholder="选填" /></label>
        <label>手机<input v-model="form.admin_phone" placeholder="选填" /></label>
        <div class="btns">
          <button @click="showCreate = false">取消</button>
          <button class="btn-primary" @click="create" :disabled="creating">{{ creating ? '创建中...' : '确认' }}</button>
        </div>
      </div>
    </div>

    <!-- 重置密码弹窗 -->
    <div v-if="showReset" class="modal" @click.self="showReset = false">
      <div class="dlg">
        <h3>重置服务商管理员密码</h3>
        <p>目标：<b>{{ resetTarget?.name }}</b></p>
        <label>用户名<input v-model="resetForm.username" /></label>
        <label>新密码<input v-model="resetForm.new_password" type="password" /></label>
        <div class="btns">
          <button @click="showReset = false">取消</button>
          <button class="btn-primary" @click="doReset">确认</button>
        </div>
      </div>
    </div>

    <!-- 创建成功弹窗 -->
    <div v-if="createdResult" class="modal" @click.self="createdResult = null">
      <div class="dlg success">
        <h3>✅ 服务商账号已开通</h3>
        <p>请将以下登录信息交付服务商：</p>
        <div class="credential">
          <div><span>服务商编码：</span><code>{{ createdResult.tenant.code }}</code></div>
          <div><span>管理员用户名：</span><code>{{ createdResult.admin.username }}</code></div>
        </div>
        <div class="btns"><button class="btn-primary" @click="createdResult = null">确定</button></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const list = ref<any[]>([])
const showCreate = ref(false)
const showReset = ref(false)
const creating = ref(false)
const createdResult = ref<any>(null)
const resetTarget = ref<any>(null)

const form = ref({
  name: '', code: '',
  admin_username: 'admin', admin_password: '', admin_nickname: '',
  admin_email: '', admin_phone: '',
})
const resetForm = ref({ tenant_id: '', username: 'admin', new_password: '' })

async function load() {
  try { const r = await api.get('/admin/developer/providers'); list.value = r.data?.data?.list || [] }
  catch { list.value = [] }
}
async function create() {
  creating.value = true
  try {
    const r = await api.post('/admin/developer/providers', form.value)
    createdResult.value = r.data?.data
    showCreate.value = false
    form.value = { name: '', code: '', admin_username: 'admin', admin_password: '', admin_nickname: '', admin_email: '', admin_phone: '' }
    load()
  } catch (e: any) { alert(e.response?.data?.message || '创建失败') }
  finally { creating.value = false }
}
async function toggle(t: any) {
  await api.put(`/admin/developer/providers/${t.id}/status`, { status: t.status === 1 ? 0 : 1 })
  load()
}
function openReset(t: any) {
  resetTarget.value = t
  resetForm.value = { tenant_id: t.id, username: 'admin', new_password: '' }
  showReset.value = true
}
async function doReset() {
  try {
    await api.post('/admin/developer/providers/reset-password', resetForm.value)
    alert('密码已重置')
    showReset.value = false
  } catch (e: any) { alert(e.response?.data?.message || '失败') }
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
.btn-primary:disabled { opacity:.6; }
.tbl { width:100%;background:#fff;border-radius:8px;border-collapse:collapse;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.tbl th, .tbl td { padding:12px 16px;text-align:left;border-bottom:1px solid #f0f0f0; }
.tbl th { background:#fafafa;color:#555; }
.tag { background:#f0f5ff;color:#4a9eff;padding:2px 8px;border-radius:4px;font-size:12px; }
.ok { color:#52c41a; } .ko { color:#ff4d4f; }
a { color:#4a9eff;cursor:pointer;margin-right:12px; }
.empty { text-align:center;color:#999;padding:40px; }
.modal { position:fixed;inset:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center;z-index:1000; }
.dlg { background:#fff;border-radius:12px;padding:24px;width:440px;max-height:90vh;overflow-y:auto; }
.dlg h3 { margin-bottom:16px; }
.dlg .section { font-weight:600;color:#4a9eff;margin:12px 0 8px;font-size:13px; }
.dlg label { display:block;margin-bottom:10px;color:#555;font-size:13px; }
.dlg input { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px;margin-top:4px; }
.btns { display:flex;gap:12px;justify-content:flex-end;margin-top:16px; }
.btns button { padding:8px 18px;background:#f0f0f0;border:none;border-radius:6px;cursor:pointer; }
.success .credential { background:#f0fff4;border:1px solid #52c41a;padding:12px;border-radius:6px;margin:12px 0; }
.success .credential div { margin:6px 0; }
.success .credential span { color:#666; }
.success .credential code { background:#fff;padding:2px 8px;border-radius:3px;color:#4a9eff;font-weight:bold; }
</style>
