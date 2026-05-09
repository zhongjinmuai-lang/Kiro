<template>
  <div class="genealogy">
    <div class="header">
      <h1>族谱可视化</h1>
      <div class="actions">
        <button @click="showCreate = true">+ 新增成员</button>
      </div>
    </div>

    <div class="layout">
      <aside class="side">
        <h3>分支</h3>
        <ul>
          <li v-for="b in branches" :key="b.id" :class="{ active: b.id === currentBranch }" @click="loadMembers(b.id)">{{ b.name }}</li>
          <li v-if="!branches.length" class="empty">暂无分支</li>
        </ul>
      </aside>
      <main class="main">
        <div v-if="tree" class="tree"><tree-node :node="tree" /></div>
        <div v-else class="tip">选择左侧分支或新增族谱起点</div>
      </main>
      <section class="detail" v-if="selected">
        <h3>{{ selected.name }}</h3>
        <p>字号：{{ selected.alias_name || '-' }}</p>
        <p>世代：第 {{ selected.generation }} 代</p>
        <p>性别：{{ selected.gender === 'male' ? '男' : selected.gender === 'female' ? '女' : '未知' }}</p>
        <p>出生地：{{ selected.birthplace || '-' }}</p>
        <div class="btns">
          <button @click="viewAncestors">溯源祖先</button>
          <button @click="viewDescendants">分支遍历</button>
        </div>
      </section>
    </div>

    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="dlg">
        <h3>新增成员</h3>
        <label>姓名<input v-model="form.name" /></label>
        <label>字号<input v-model="form.alias_name" /></label>
        <label>性别<select v-model="form.gender"><option value="male">男</option><option value="female">女</option><option value="unknown">未知</option></select></label>
        <label>父亲ID（可选）<input v-model="form.father_id" placeholder="上级成员 ID" /></label>
        <label>世代（可选，留空自动推导）<input v-model.number="form.generation" type="number" /></label>
        <div class="btns">
          <button @click="showCreate = false">取消</button>
          <button class="primary" @click="create">确认</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h, type FunctionalComponent } from 'vue'
import { api } from '@/utils/request'

const TreeNode: FunctionalComponent<{ node: any }> = (props) => {
  const n = props.node
  if (!n) return h('div')
  return h('div', { class: 'tn' }, [
    h('div', { class: 'tn-self' }, [
      h('span', { class: `g-${n.gender || 'unknown'}` }, n.name),
      h('small', null, `· 第${n.generation}代`),
    ]),
    n.children?.length
      ? h('div', { class: 'tn-children' }, n.children.map((c: any) => h(TreeNode as any, { node: c })))
      : null,
  ])
}

const branches = ref<any[]>([])
const currentBranch = ref<string>('')
const tree = ref<any>(null)
const selected = ref<any>(null)
const showCreate = ref(false)
const form = ref<any>({ name: '', alias_name: '', gender: 'male', father_id: '', generation: 0 })

async function loadBranches() {
  try { const r = await api.get('/api/v1/genealogy/branches'); branches.value = r.data?.data || [] } catch {}
  if (branches.value.length) loadMembers(branches.value[0].id)
}

async function loadMembers(branchId: string) {
  currentBranch.value = branchId
  try {
    const r = await api.get('/api/v1/genealogy/members?branch_id=' + branchId + '&page_size=1')
    const list = r.data?.data?.list || []
    if (list.length) {
      selected.value = list[0]
      const t = await api.get(`/api/v1/genealogy/tree?root=${list[0].id}&depth=10`)
      tree.value = t.data?.data
    } else {
      selected.value = null
      tree.value = null
    }
  } catch {}
}

async function create() {
  try {
    const payload: any = { name: form.value.name, alias_name: form.value.alias_name, gender: form.value.gender, branch_id: currentBranch.value }
    if (form.value.father_id) payload.father_id = form.value.father_id
    if (form.value.generation) payload.generation = form.value.generation
    await api.post('/api/v1/genealogy/members', payload)
    showCreate.value = false
    form.value = { name: '', alias_name: '', gender: 'male', father_id: '', generation: 0 }
    loadMembers(currentBranch.value)
  } catch (e: any) { alert(e.response?.data?.message || '创建失败') }
}

async function viewAncestors() {
  if (!selected.value) return
  const r = await api.get(`/api/v1/genealogy/members/${selected.value.id}/ancestors`)
  alert('祖先链路：\n' + (r.data?.data || []).map((m: any) => m.name).join(' ← '))
}
async function viewDescendants() {
  if (!selected.value) return
  const r = await api.get(`/api/v1/genealogy/members/${selected.value.id}/descendants`)
  alert('后裔数量：' + (r.data?.data || []).length)
}

onMounted(loadBranches)
</script>

<style scoped>
.header { display:flex;justify-content:space-between;align-items:center;margin-bottom:20px; }
.actions button { padding:8px 16px;background:#2d3436;color:#fff;border:none;border-radius:6px;cursor:pointer; }
.layout { display:grid;grid-template-columns:200px 1fr 260px;gap:16px;min-height:500px; }
.side, .main, .detail { background:#fff;border-radius:8px;padding:16px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.side ul { list-style:none;padding:0;margin-top:12px; }
.side li { padding:8px 10px;border-radius:4px;cursor:pointer; }
.side li.active { background:#f0f5ff;color:#0f3460; }
.empty { color:#999;text-align:center;padding:20px; }
.tip { color:#999;text-align:center;padding:80px 0; }
.btns button { padding:6px 12px;margin-top:8px;margin-right:8px;background:#f5f5f5;border:none;border-radius:4px;cursor:pointer; }
:deep(.tn) { margin:4px 0;padding-left:16px;border-left:1px dashed #ccc; }
:deep(.tn-self) { padding:4px 8px;display:inline-block;background:#fafafa;border-radius:4px; }
:deep(.g-male) { color:#1890ff; } :deep(.g-female) { color:#eb2f96; } :deep(.g-unknown) { color:#888; }
:deep(.tn-self small) { color:#aaa;margin-left:6px; }
.modal { position:fixed;inset:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center;z-index:100; }
.dlg { background:#fff;border-radius:12px;padding:24px;width:400px; }
.dlg h3 { margin-bottom:16px; } .dlg label { display:block;margin-bottom:12px;color:#555;font-size:13px; }
.dlg input, .dlg select { width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:4px;margin-top:4px; }
.dlg .btns { display:flex;gap:12px;justify-content:flex-end;margin-top:16px; }
.dlg button { padding:8px 18px;background:#f0f0f0;border:none;border-radius:6px;cursor:pointer; }
.dlg button.primary { background:#2d3436;color:#fff; }
</style>
