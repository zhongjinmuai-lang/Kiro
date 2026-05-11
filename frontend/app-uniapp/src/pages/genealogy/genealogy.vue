<template>
  <view class="page">
    <view class="header">
      <text class="title">族谱世系</text>
    </view>

    <!-- 分支选择 -->
    <scroll-view scroll-x class="branch-scroll">
      <view class="branch-list">
        <view class="branch-tag" :class="{ active: !currentBranch }" @click="currentBranch = ''; loadTree()">
          全部
        </view>
        <view class="branch-tag" :class="{ active: currentBranch === b.id }" v-for="b in branches" :key="b.id" @click="selectBranch(b)">
          {{ b.name }}
        </view>
      </view>
    </scroll-view>

    <!-- 世系树 -->
    <view class="tree-area" v-if="tree">
      <view class="tree-node" v-for="node in flatTree" :key="node.id" :style="{ paddingLeft: node.depth * 40 + 'rpx' }">
        <view class="node-line" v-if="node.depth > 0"></view>
        <view class="node-card" :class="'gender-' + node.gender" @click="showDetail(node)">
          <text class="node-name">{{ node.name }}</text>
          <text class="node-gen">第{{ node.generation }}代</text>
        </view>
      </view>
    </view>
    <view v-else class="empty">
      <text class="empty-text">{{ loading ? '加载中...' : '暂无族谱数据' }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api, isLoggedIn } from '@/utils/request'

const branches = ref<any[]>([])
const currentBranch = ref('')
const tree = ref<any>(null)
const flatTree = ref<any[]>([])
const loading = ref(false)

function selectBranch(b: any) {
  currentBranch.value = b.id
  loadTree()
}

// 将树形结构拍平用于列表渲染
function flattenTree(node: any, depth = 0): any[] {
  if (!node) return []
  const result = [{ ...node, depth }]
  if (node.children) {
    for (const child of node.children) {
      result.push(...flattenTree(child, depth + 1))
    }
  }
  return result
}

async function loadTree() {
  loading.value = true
  try {
    // 先获取根成员
    let url = '/api/v1/genealogy/members?page_size=1&page=1'
    if (currentBranch.value) url += '&branch_id=' + currentBranch.value
    const membersRes = await api.get(url)
    const list = membersRes.data?.list || []
    if (list.length > 0) {
      const rootId = list[0].id
      const treeRes = await api.get(`/api/v1/genealogy/tree?root=${rootId}&depth=10`)
      tree.value = treeRes.data
      flatTree.value = flattenTree(treeRes.data)
    } else {
      tree.value = null
      flatTree.value = []
    }
  } catch {}
  loading.value = false
}

function showDetail(node: any) {
  uni.showModal({
    title: node.name,
    content: `世代：第${node.generation}代\n性别：${node.gender === 'male' ? '男' : node.gender === 'female' ? '女' : '未知'}`,
    showCancel: false,
  })
}

onMounted(async () => {
  if (!isLoggedIn()) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  // 加载分支
  try {
    const res = await api.get('/api/v1/genealogy/branches')
    branches.value = res.data || []
  } catch {}
  loadTree()
})
</script>

<style scoped>
.page { padding: 30rpx; background: #f5f7fa; min-height: 100vh; }
.header { margin-bottom: 24rpx; }
.title { font-size: 36rpx; font-weight: bold; color: #333; }
.branch-scroll { margin-bottom: 30rpx; white-space: nowrap; }
.branch-list { display: flex; gap: 16rpx; }
.branch-tag { padding: 12rpx 28rpx; background: #fff; border-radius: 32rpx; font-size: 26rpx; color: #666; flex-shrink: 0; }
.branch-tag.active { background: #4a9eff; color: #fff; }
.tree-area { background: #fff; border-radius: 16rpx; padding: 20rpx; }
.tree-node { display: flex; align-items: center; margin-bottom: 12rpx; }
.node-line { width: 20rpx; height: 2rpx; background: #ddd; margin-right: 8rpx; }
.node-card { padding: 16rpx 24rpx; border-radius: 10rpx; border: 1rpx solid #e8e8e8; display: flex; align-items: center; gap: 12rpx; }
.gender-male { border-color: #4a9eff; background: #f0f7ff; }
.gender-female { border-color: #eb2f96; background: #fff0f6; }
.gender-unknown { border-color: #d9d9d9; background: #fafafa; }
.node-name { font-size: 28rpx; font-weight: 500; color: #333; }
.node-gen { font-size: 22rpx; color: #999; }
.empty { text-align: center; padding: 100rpx 0; }
.empty-text { font-size: 28rpx; color: #999; }
</style>
