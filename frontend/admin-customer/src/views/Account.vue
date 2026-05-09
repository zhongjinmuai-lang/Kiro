<template>
  <div class="page"><h1>账户设置</h1>
    <div class="card">
      <h3>基本信息</h3>
      <div class="row"><label>用户名</label><input v-model="me.username" disabled /></div>
      <div class="row"><label>昵称</label><input v-model="me.nickname" /></div>
      <div class="row"><label>邮箱</label><input v-model="me.email" /></div>
      <div class="row"><label>手机</label><input v-model="me.phone" /></div>
    </div>
    <div class="card">
      <h3>修改密码</h3>
      <div class="row"><label>旧密码</label><input type="password" v-model="pwd.old_password" /></div>
      <div class="row"><label>新密码</label><input type="password" v-model="pwd.new_password" /></div>
      <button @click="changePwd">修改密码</button>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'
const me = ref<any>({ username:'', nickname:'', email:'', phone:'' })
const pwd = ref({ old_password:'', new_password:'' })
onMounted(async () => { try { const r = await api.get('/api/v1/me'); me.value = r.data?.data || me.value } catch {} })
async function changePwd() {
  try { await api.put('/api/v1/auth/password', pwd.value); alert('密码修改成功，请重新登录'); localStorage.clear(); location.href = '/login' }
  catch (e:any) { alert(e.response?.data?.message || '失败') }
}
</script>
<style scoped>
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06);margin-bottom:16px;max-width:600px; }
.card h3 { margin-bottom:16px; }
.row { margin-bottom:14px; } label { display:block;margin-bottom:6px;color:#555; }
input { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px; }
button { padding:8px 20px;background:#2d3436;color:#fff;border:none;border-radius:4px;cursor:pointer; }
</style>
