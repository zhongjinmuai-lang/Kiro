<template>
  <view class="page">
    <view class="header">
      <text class="title">工作台</text>
      <text class="sub">族谱成员管理</text>
    </view>

    <!-- 搜索 -->
    <view class="search-bar">
      <input v-model="keyword" placeholder="搜索成员姓名" class="search-input" @confirm="search" />
    </view>

    <!-- 成员列表 -->
    <view class="member-list">
      <view class="member-card" v-for="m in members" :key="m.id" @click="viewDetail(m)">
        <view class="member-avatar">
          <text class="avatar-text">{{ m.name?.charAt(0) || '?' }}</text>
        </view>
        <view class="member-info">
          <text class="member-name">{{ m.name }}</text>
          <text class="member-meta">第{{ m.generation }}代 · {{ genderText(m.gender) }}</text>
        </view>
        <text class="member-arrow">›</text>
      </view>
      <view v-if="!members.length && !loading" class="empty">
        <text class="empty-text">暂无成员数据</text>
      </view>
    </view>

    <!-- 加载更多 -->
    <view v-if="loading" class="loading">
      <text>加载中...</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const members = ref<any[]>([])
const keyword = ref('')
const loading = ref(false)
const page = ref(1)

function genderText(g: string) {
  return g === 'male' ? '男' : g === 'female' ? '女' : '未知'
}

async function loadMembers() {
  loading.value = true
  try {
    const res = await api.get(`/api/v1/genealogy/members?page=${page.value}&page_size=20`)
    members.value = res.data?.list || []
  } catch {}
  loading.value = false
}

function search() {
  // 前端过滤（小数据量），生产应后端搜索
  if (!keyword.value) {
    loadMembers()
    return
  }
  members.value = members.value.filter(m =>
    m.name?.includes(keyword.value) || m.alias_name?.includes(keyword.value)
  )
}

function viewDetail(m: any) {
  uni.showModal({
    title: m.name,
    content: `字号：${m.alias_name || '无'}\n世代：第${m.generation}代\n性别：${genderText(m.gender)}\n出生地：${m.birthplace || '未知'}`,
    showCancel: false,
  })
}

onMounted(loadMembers)
</script>

<style scoped>
.page { padding: 30rpx; background: #f5f7fa; min-height: 100vh; }
.header { margin-bottom: 30rpx; }
.title { font-size: 36rpx; font-weight: bold; color: #333; display: block; }
.sub { font-size: 26rpx; color: #999; display: block; margin-top: 8rpx; }
.search-bar { margin-bottom: 30rpx; }
.search-input { background: #fff; border-radius: 40rpx; padding: 20rpx 30rpx; font-size: 28rpx; box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.04); }
.member-list { }
.member-card { display: flex; align-items: center; background: #fff; border-radius: 16rpx; padding: 24rpx 30rpx; margin-bottom: 16rpx; box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.03); }
.member-avatar { width: 80rpx; height: 80rpx; border-radius: 50%; background: #e8f4ff; display: flex; align-items: center; justify-content: center; margin-right: 24rpx; }
.avatar-text { font-size: 32rpx; color: #4a9eff; font-weight: bold; }
.member-info { flex: 1; }
.member-name { font-size: 30rpx; color: #333; font-weight: 500; display: block; }
.member-meta { font-size: 24rpx; color: #999; margin-top: 6rpx; display: block; }
.member-arrow { font-size: 36rpx; color: #ccc; }
.empty { text-align: center; padding: 80rpx 0; }
.empty-text { font-size: 28rpx; color: #999; }
.loading { text-align: center; padding: 30rpx; color: #999; font-size: 26rpx; }
</style>
