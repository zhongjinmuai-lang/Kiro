<template>
  <div class="plugins-page">
    <div class="page-header">
      <div class="header-left">
        <h1>插件市场</h1>
        <span class="subtitle">管理智能体插件的安装、启停与配置</span>
      </div>
      <div class="header-right">
        <input v-model="search" placeholder="搜索插件..." class="search-input" />
        <button class="btn-primary" @click="refreshPlugins">
          <span class="icon">↻</span> 刷新
        </button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <div class="stat-card">
        <span class="stat-num">{{ stats.total || 0 }}</span>
        <span class="stat-label">总插件数</span>
      </div>
      <div class="stat-card running">
        <span class="stat-num">{{ stats.running || 0 }}</span>
        <span class="stat-label">运行中</span>
      </div>
      <div class="stat-card stopped">
        <span class="stat-num">{{ stats.stopped || 0 }}</span>
        <span class="stat-label">已停止</span>
      </div>
      <div class="stat-card error">
        <span class="stat-num">{{ stats.error || 0 }}</span>
        <span class="stat-label">异常</span>
      </div>
    </div>

    <!-- 分类筛选 -->
    <div class="filter-bar">
      <button :class="['filter-btn', { active: filter === '' }]" @click="filter = ''">全部</button>
      <button :class="['filter-btn', { active: filter === 'running' }]" @click="filter = 'running'">运行中</button>
      <button :class="['filter-btn', { active: filter === 'loaded' }]" @click="filter = 'loaded'">已加载</button>
      <button :class="['filter-btn', { active: filter === 'stopped' }]" @click="filter = 'stopped'">已停止</button>
      <button :class="['filter-btn', { active: filter === 'error' }]" @click="filter = 'error'">异常</button>
    </div>

    <!-- 插件网格 -->
    <div class="plugin-grid">
      <div class="plugin-card" v-for="p in filteredPlugins" :key="p.meta?.id || p.id">
        <div class="card-header">
          <div class="plugin-icon" :class="'cat-' + (p.meta?.category || 'default')">
            {{ getCategoryIcon(p.meta?.category) }}
          </div>
          <div class="status-badge" :class="p.status">{{ statusText(p.status) }}</div>
        </div>
        <div class="card-body">
          <h3 class="plugin-name">{{ p.meta?.name || p.name || '未知插件' }}</h3>
          <p class="plugin-version">v{{ p.meta?.version || '1.0.0' }}</p>
          <p class="plugin-desc">{{ p.meta?.description || '暂无描述' }}</p>
          <div class="plugin-meta">
            <span class="meta-item" v-if="p.meta?.author">
              <span class="meta-icon">👤</span> {{ p.meta.author }}
            </span>
            <span class="meta-item" v-if="p.meta?.category">
              <span class="meta-icon">📂</span> {{ p.meta.category }}
            </span>
          </div>
        </div>
        <div class="card-footer">
          <button v-if="p.status !== 'running'" class="btn-sm btn-start" @click="startPlugin(p)" :disabled="actionLoading === p.meta?.id">
            ▶ 启动
          </button>
          <button v-if="p.status === 'running'" class="btn-sm btn-stop" @click="stopPlugin(p)" :disabled="actionLoading === p.meta?.id">
            ■ 停止
          </button>
          <button class="btn-sm btn-restart" @click="restartPlugin(p)" :disabled="actionLoading === p.meta?.id">
            ↻ 重启
          </button>
          <button class="btn-sm btn-danger" @click="uninstallPlugin(p)">
            🗑 卸载
          </button>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-if="!filteredPlugins.length && !loading" class="empty-state">
        <div class="empty-icon">🧩</div>
        <h3>{{ search ? '没有匹配的插件' : '暂无已安装插件' }}</h3>
        <p>插件通过代码注册到智能体引擎，或从插件仓库安装</p>
        <button class="btn-primary" @click="showInstallDialog = true">+ 安装插件</button>
      </div>
    </div>

    <!-- 健康检查结果 -->
    <div class="section" v-if="Object.keys(healthResults).length">
      <h2>健康检查</h2>
      <div class="health-grid">
        <div class="health-item" v-for="(h, id) in healthResults" :key="id" :class="{ healthy: h.healthy, unhealthy: !h.healthy }">
          <span class="health-dot"></span>
          <span class="health-id">{{ id }}</span>
          <span class="health-msg">{{ h.message }}</span>
          <span class="health-time">{{ formatTime(h.checked_at) }}</span>
        </div>
      </div>
    </div>

    <!-- 安装插件弹窗 -->
    <div v-if="showInstallDialog" class="modal" @click.self="showInstallDialog = false">
      <div class="dlg">
        <h3>安装插件</h3>
        <p class="tip">从插件仓库 URL 或本地路径安装插件到智能体引擎</p>
        <label>插件来源
          <select v-model="installForm.source">
            <option value="registry">官方插件仓库</option>
            <option value="url">远程 URL</option>
            <option value="local">本地路径</option>
          </select>
        </label>
        <label v-if="installForm.source === 'url'">URL 地址
          <input v-model="installForm.url" placeholder="https://plugins.mu-framework.cn/xxx.tar.gz" />
        </label>
        <label v-if="installForm.source === 'local'">本地路径
          <input v-model="installForm.path" placeholder="/www/wwwroot/mu-framework/plugins/my-plugin" />
        </label>
        <label v-if="installForm.source === 'registry'">选择插件
          <select v-model="installForm.pluginId">
            <option value="">请选择...</option>
            <option value="hello">Hello 示例插件</option>
            <option value="genealogy-ai">族谱AI识别增强</option>
            <option value="notify-wechat">微信模板消息推送</option>
            <option value="storage-cleaner">存储清理定时任务</option>
            <option value="payment-reconcile">支付对账自动化</option>
          </select>
        </label>
        <div class="btns">
          <button @click="showInstallDialog = false">取消</button>
          <button class="btn-primary" @click="installPlugin" :disabled="installing">
            {{ installing ? '安装中...' : '确认安装' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/utils/request'

// 状态
const plugins = ref<any[]>([])
const stats = ref<any>({})
const healthResults = ref<Record<string, any>>({})
const loading = ref(false)
const search = ref('')
const filter = ref('')
const actionLoading = ref('')
const showInstallDialog = ref(false)
const installing = ref(false)
const installForm = ref({ source: 'registry', url: '', path: '', pluginId: '' })

// 计算属性
const filteredPlugins = computed(() => {
  let list = plugins.value
  if (filter.value) {
    list = list.filter(p => p.status === filter.value)
  }
  if (search.value) {
    const kw = search.value.toLowerCase()
    list = list.filter(p =>
      (p.meta?.name || '').toLowerCase().includes(kw) ||
      (p.meta?.description || '').toLowerCase().includes(kw) ||
      (p.meta?.category || '').toLowerCase().includes(kw)
    )
  }
  return list
})

// 方法
function statusText(s: string) {
  const map: Record<string, string> = {
    running: '运行中', loaded: '已加载', stopped: '已停止', error: '异常', upgrading: '升级中'
  }
  return map[s] || s
}

function getCategoryIcon(category: string) {
  const map: Record<string, string> = {
    ai: '🤖', payment: '💳', storage: '📦', notify: '🔔',
    genealogy: '🌳', security: '🔒', sample: '🧪', analytics: '📊'
  }
  return map[category] || '🧩'
}

function formatTime(t: string) {
  if (!t) return ''
  return new Date(t).toLocaleTimeString()
}

async function refreshPlugins() {
  loading.value = true
  try {
    const r = await api.get('/agent/plugins')
    plugins.value = r.data?.data || []
    // 计算统计
    const s = { total: plugins.value.length, running: 0, stopped: 0, loaded: 0, error: 0 }
    plugins.value.forEach(p => {
      if (p.status === 'running') s.running++
      else if (p.status === 'stopped') s.stopped++
      else if (p.status === 'loaded') s.loaded++
      else if (p.status === 'error') s.error++
    })
    stats.value = s
  } catch { plugins.value = [] }
  loading.value = false

  // 健康检查
  try {
    const h = await api.get('/agent/plugins/health')
    healthResults.value = h.data?.data || {}
  } catch {}
}

async function startPlugin(p: any) {
  const id = p.meta?.id || p.id
  actionLoading.value = id
  try {
    await api.post(`/agent/plugins/${id}/start`)
    await refreshPlugins()
  } catch (e: any) { alert(e.response?.data?.message || '启动失败') }
  actionLoading.value = ''
}

async function stopPlugin(p: any) {
  const id = p.meta?.id || p.id
  actionLoading.value = id
  try {
    await api.post(`/agent/plugins/${id}/stop`)
    await refreshPlugins()
  } catch (e: any) { alert(e.response?.data?.message || '停止失败') }
  actionLoading.value = ''
}

async function restartPlugin(p: any) {
  const id = p.meta?.id || p.id
  actionLoading.value = id
  try {
    await api.post(`/agent/plugins/${id}/stop`)
    await api.post(`/agent/plugins/${id}/start`)
    await refreshPlugins()
  } catch (e: any) { alert(e.response?.data?.message || '重启失败') }
  actionLoading.value = ''
}

async function uninstallPlugin(p: any) {
  const id = p.meta?.id || p.id
  const name = p.meta?.name || id
  if (!confirm(`确认卸载插件「${name}」？此操作不可逆。`)) return
  try {
    await api.delete(`/agent/plugins/${id}`)
    await refreshPlugins()
  } catch (e: any) { alert(e.response?.data?.message || '卸载失败') }
}

async function installPlugin() {
  installing.value = true
  try {
    await api.post('/agent/plugins/install', installForm.value)
    showInstallDialog.value = false
    installForm.value = { source: 'registry', url: '', path: '', pluginId: '' }
    await refreshPlugins()
    alert('插件安装成功')
  } catch (e: any) { alert(e.response?.data?.message || '安装失败') }
  installing.value = false
}

onMounted(refreshPlugins)
</script>

<style scoped>
.plugins-page { }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 24px; }
.header-left h1 { margin: 0; font-size: 24px; }
.subtitle { color: #888; font-size: 13px; margin-top: 4px; display: block; }
.header-right { display: flex; gap: 12px; align-items: center; }
.search-input { padding: 8px 16px; border: 1px solid #e0e0e0; border-radius: 8px; width: 220px; font-size: 14px; }
.btn-primary { padding: 8px 20px; background: #4a9eff; color: #fff; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; }
.btn-primary:hover { background: #3a8eef; }
.btn-primary:disabled { opacity: .6; }
.icon { margin-right: 4px; }

/* 统计卡片 */
.stats-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
.stat-card { background: #fff; border-radius: 12px; padding: 20px; text-align: center; box-shadow: 0 2px 8px rgba(0,0,0,.04); border-left: 4px solid #e0e0e0; }
.stat-card.running { border-left-color: #52c41a; }
.stat-card.stopped { border-left-color: #faad14; }
.stat-card.error { border-left-color: #ff4d4f; }
.stat-num { font-size: 32px; font-weight: bold; color: #333; display: block; }
.stat-label { font-size: 13px; color: #888; margin-top: 4px; display: block; }

/* 筛选栏 */
.filter-bar { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
.filter-btn { padding: 6px 16px; background: #f5f5f5; border: 1px solid #e0e0e0; border-radius: 20px; cursor: pointer; font-size: 13px; color: #555; }
.filter-btn.active { background: #4a9eff; color: #fff; border-color: #4a9eff; }
.filter-btn:hover { border-color: #4a9eff; }

/* 插件网格 */
.plugin-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 20px; }
.plugin-card { background: #fff; border-radius: 12px; box-shadow: 0 2px 12px rgba(0,0,0,.06); overflow: hidden; transition: transform .2s, box-shadow .2s; }
.plugin-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgba(0,0,0,.1); }

.card-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px 0; }
.plugin-icon { width: 44px; height: 44px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 22px; background: #f0f5ff; }
.cat-ai { background: #f0f5ff; }
.cat-payment { background: #fff7e6; }
.cat-storage { background: #f6ffed; }
.cat-notify { background: #fff1f0; }
.cat-sample { background: #f9f0ff; }

.status-badge { padding: 3px 10px; border-radius: 12px; font-size: 12px; font-weight: 500; }
.status-badge.running { background: #f6ffed; color: #52c41a; }
.status-badge.loaded { background: #e6f7ff; color: #1890ff; }
.status-badge.stopped { background: #fffbe6; color: #faad14; }
.status-badge.error { background: #fff1f0; color: #ff4d4f; }
.status-badge.upgrading { background: #f9f0ff; color: #722ed1; }

.card-body { padding: 16px 20px; }
.plugin-name { margin: 0 0 4px; font-size: 16px; color: #333; }
.plugin-version { font-size: 12px; color: #999; margin: 0 0 8px; }
.plugin-desc { font-size: 13px; color: #666; line-height: 1.5; margin: 0 0 12px; min-height: 40px; }
.plugin-meta { display: flex; gap: 16px; flex-wrap: wrap; }
.meta-item { font-size: 12px; color: #888; display: flex; align-items: center; gap: 4px; }

.card-footer { padding: 12px 20px; border-top: 1px solid #f5f5f5; display: flex; gap: 8px; flex-wrap: wrap; }
.btn-sm { padding: 5px 12px; border: 1px solid #e0e0e0; border-radius: 6px; font-size: 12px; cursor: pointer; background: #fff; }
.btn-sm:hover { border-color: #4a9eff; color: #4a9eff; }
.btn-sm:disabled { opacity: .5; cursor: not-allowed; }
.btn-start { color: #52c41a; border-color: #b7eb8f; }
.btn-start:hover { background: #f6ffed; }
.btn-stop { color: #faad14; border-color: #ffe58f; }
.btn-stop:hover { background: #fffbe6; }
.btn-restart { color: #1890ff; border-color: #91d5ff; }
.btn-restart:hover { background: #e6f7ff; }
.btn-danger { color: #ff4d4f; border-color: #ffccc7; }
.btn-danger:hover { background: #fff1f0; }

/* 空状态 */
.empty-state { grid-column: 1 / -1; text-align: center; padding: 80px 0; }
.empty-icon { font-size: 64px; margin-bottom: 16px; }
.empty-state h3 { color: #333; margin-bottom: 8px; }
.empty-state p { color: #888; margin-bottom: 24px; }

/* 健康检查 */
.section { margin-top: 32px; }
.section h2 { font-size: 18px; margin-bottom: 16px; }
.health-grid { display: flex; flex-direction: column; gap: 8px; }
.health-item { display: flex; align-items: center; gap: 12px; padding: 10px 16px; background: #fff; border-radius: 8px; font-size: 13px; }
.health-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.healthy .health-dot { background: #52c41a; }
.unhealthy .health-dot { background: #ff4d4f; }
.health-id { font-weight: 600; color: #333; min-width: 100px; }
.health-msg { flex: 1; color: #666; }
.health-time { color: #999; font-size: 12px; }

/* 弹窗 */
.modal { position: fixed; inset: 0; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.dlg { background: #fff; border-radius: 16px; padding: 28px; width: 480px; max-height: 90vh; overflow-y: auto; }
.dlg h3 { margin: 0 0 8px; font-size: 18px; }
.dlg .tip { color: #888; font-size: 13px; margin-bottom: 20px; }
.dlg label { display: block; margin-bottom: 16px; font-size: 13px; color: #555; }
.dlg input, .dlg select { width: 100%; padding: 10px 14px; border: 1px solid #e0e0e0; border-radius: 8px; margin-top: 6px; font-size: 14px; }
.btns { display: flex; gap: 12px; justify-content: flex-end; margin-top: 24px; }
.btns button { padding: 10px 24px; border-radius: 8px; border: 1px solid #e0e0e0; cursor: pointer; font-size: 14px; background: #f5f5f5; }
</style>
