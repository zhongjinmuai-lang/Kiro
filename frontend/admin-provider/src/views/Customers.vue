<template>
  <div class="page">
    <div class="header">
      <h1>客户管理</h1>
      <button @click="showCreate = true">+ 新增终端客户（开账户）</button>
    </div>
    <table class="tbl">
      <thead><tr><th>编码</th><th>名称</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="c in list" :key="c.id">
          <td>{{ c.code }}</td>
          <td>{{ c.name }}</td>
          <td><span :class="c.status === 1 ? 'ok' : 'ko'">{{ c.status === 1 ? '启用' : '禁用' }}</span></td>
          <td>{{ (c.created_at || '').slice(0, 10) }}</td>
          <td>
            <a @click="toggle(c)">{{ c.status === 1 ? '禁用' : '启用' }}</a>
            <a @click="openReset(c)" style="margin-left:12px;">重置密码</a>
          </td>
        </tr>
        <tr v-if="!list.length"><td colspan="5" class="empty">暂无客户</td></tr>
      </tbody>
    </table>

    <!-- 创建客户弹窗 -->
    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="dlg">
        <h3>新增终端客户（自动开通账号）</h3>
        <div class="section">家族基本信息</div>
        <label>家族名称<input v-model="form.name" placeholder="如：张氏宗族" /></label>
        <label>家族编码<input v-model="form.code" placeholder="唯一编码，如 zhang-family-001" /></label>
        <div class="section">初始管理员账号（创建后给客户登录用）</div>
        <label>用户名<input v-model="form.admin_username" placeholder="如 admin" /></label>
        <label>密码<input v-model="form.admin_password" type="password" placeholder="至少 6 位" /></label>
        <label>昵称<input v-model="form.admin_nickname" placeholder="如 族长张三（可选）" /></label>
        <label>邮箱<input v-model="form.admin_email" placeholder="选填" /></label>
        <label>手机<input v-model="form.admin_phone" placeholder="选填" /></label>
        <div class="btns">
          <button @click="showCreate = false">取消</button>
          <button class="primary" @click="create" :disabled="creating">{{ creating ? '创建中...' : '确认' }}</button>
        </div>
      </div>
    </div>

    <!-- 重置密码弹窗 -->
    <div v-if="showReset" class="modal" @click.self="showReset = false">
      <div class="dlg">
        <h3>重置终端客户管理员密码</h3>
        <p class="tip">家族：<b>{{ resetTarget?.name }}</b></p>
        <label>管理员用户名<input v-model="resetForm.username" placeholder="默认 admin" /></label>
        <label>新密码<input v-model="resetForm.new_password" type="password" placeholder="至少 6 位" /></label>
        <div class="btns">
          <button @click="showReset = false">取消</button>
          <button class="primary" @click="doReset">确认重置</button>
        </div>
      </div>
    </div>

    <!-- 创建成功提示 -->
    <div v-if="createdResult" class="modal" @click.self="createdResult = null">
      <div class="dlg success">
        <h3>✅ 客户账号已开通</h3>
        <p>请将以下登录信息转交客户：</p>
        <div class="credential">
          <div><span>家族编码：</span><code>{{ createdResult.tenant.code }}</code></div>
          <div><span>用户名：</span><code>{{ createdResult.admin.username }}</code></div>
          <div class="warn">密码已安全存储（bcrypt），如需查看请使用"重置密码"功能。</div>
        </div>
        <div class="btns"><button class="primary" @click="createdResult = null">我知道了</button></div>
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
  try { const r = await api.get('/admin/provider/customers'); list.value = r.data?.data?.list || [] }
  catch { list.value = [] }
}
async function create() {
  creating.value = true
  try {
    const r = await api.post('/admin/provider/customers', form.value)
    createdResult.value = r.data?.data
    showCreate.value = false
    form.value = { name: '', code: '', admin_username: 'admin', admin_password: '', admin_nickname: '', admin_email: '', admin_phone: '' }
    load()
  } catch (e: any) { alert(e.response?.data?.message || '创建失败') }
  finally { creating.value = false }
}
async function toggle(c: any) {
  await api.put(`/admin/provider/customers/${c.id}/status`, { status: c.status === 1 ? 0 : 1 })
  load()
}
function openReset(c: any) {
  resetTarget.value = c
  resetForm.value = { tenant_id: c.id, username: 'admin', new_password: '' }
  showReset.value = true
}
async function doReset() {
  try {
    await api.post('/admin/provider/customers/reset-password', resetForm.value)
    alert('密码已重置，请通知客户')
    showReset.value = false
  } catch (e: any) { alert(e.response?.data?.message || '重置失败') }
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
.modal { position:fixed;inset:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center;z-index:1000; }
.dlg { background:#fff;border-radius:12px;padding:24px;width:440px;max-height:90vh;overflow-y:auto; }
.dlg h3 { margin-bottom:16px; }
.dlg .section { font-weight:600;color:#0f3460;margin:12px 0 8px;font-size:13px; }
.dlg label { display:block;margin-bottom:10px;color:#555;font-size:13px; }
.dlg input { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px;margin-top:4px; }
.dlg .tip { color:#666;margin-bottom:12px; }
.btns { display:flex;gap:12px;justify-content:flex-end;margin-top:16px; }
.btns button { padding:8px 18px;background:#f0f0f0;border:none;border-radius:6px;cursor:pointer; }
.btns button.primary { background:#0f3460;color:#fff; }
.btns button:disabled { opacity:.6; }
.success .credential { background:#f0fff4;border:1px solid #52c41a;padding:12px;border-radius:6px;margin:12px 0; }
.success .credential div { margin:6px 0;font-size:14px; }
.success .credential span { color:#666; }
.success .credential code { background:#fff;padding:2px 8px;border-radius:3px;color:#0f3460;font-weight:bold; }
.success .warn { color:#fa8c16;font-size:12px;margin-top:8px; }
</style>
