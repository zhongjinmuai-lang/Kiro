<template>
  <view class="container">
    <view class="header">
      <text class="greeting">{{ greeting }}</text>
      <text class="subtitle">{{ user?.nickname || user?.username || '欢迎使用' }}</text>
    </view>

    <!-- 统计卡片 -->
    <view class="stats" v-if="stats">
      <view class="stat-card">
        <text class="stat-num">{{ stats.members || 0 }}</text>
        <text class="stat-label">族人数</text>
      </view>
      <view class="stat-card">
        <text class="stat-num">{{ stats.branches || 0 }}</text>
        <text class="stat-label">分支数</text>
      </view>
      <view class="stat-card">
        <text class="stat-num">{{ stats.generations || 0 }}</text>
        <text class="stat-label">世代数</text>
      </view>
    </view>

    <!-- 快捷功能 -->
    <view class="section">
      <text class="section-title">快捷功能</text>
      <view class="features">
        <view class="feature-card" v-for="item in features" :key="item.title" @click="item.action">
          <text class="feature-icon">{{ item.icon }}</text>
          <text class="feature-title">{{ item.title }}</text>
        </view>
      </view>
    </view>

    <!-- 最新公告 -->
    <view class="section" v-if="announces.length">
      <text class="section-title">家族公告</text>
      <view class="announce-list">
        <view class="announce-item" v-for="a in announces" :key="a.id">
          <text class="announce-title">{{ a.title }}</text>
          <text class="announce-time">{{ formatDate(a.publish_at) }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api, isLoggedIn } from '@/utils/request'

const user = ref<any>(null)
const stats = ref<any>(null)
const announces = ref<any[]>([])

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

const features = [
  { icon: '🌳', title: '族谱', action: () => uni.navigateTo({ url: '/pages/genealogy/genealogy' }) },
  { icon: '👥', title: '成员', action: () => uni.switchTab({ url: '/pages/workspace/workspace' }) },
  { icon: '📢', title: '公告', action: () => uni.showToast({ title: '开发中', icon: 'none' }) },
  { icon: '📁', title: '文件', action: () => uni.showToast({ title: '开发中', icon: 'none' }) },
  { icon: '💬', title: '消息', action: () => uni.switchTab({ url: '/pages/messages/messages' }) },
  { icon: '⚙️', title: '设置', action: () => uni.switchTab({ url: '/pages/mine/mine' }) },
]

function formatDate(d: string) {
  return d ? d.slice(0, 10) : ''
}

onMounted(async () => {
  // 检查登录状态
  if (!isLoggedIn()) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }

  // 恢复用户信息
  const cached = uni.getStorageSync('mu_user')
  if (cached) user.value = JSON.parse(cached)

  // 加载统计
  try {
    const res = await api.get('/api/v1/genealogy/stats')
    stats.value = res.data
  } catch {}

  // 加载公告
  try {
    const res = await api.get('/api/v1/genealogy/announces?page_size=5')
    announces.value = res.data?.list || []
  } catch {}
})
</script>

<style scoped>
.container { padding: 30rpx; background: #f5f7fa; min-height: 100vh; }
.header { margin-bottom: 40rpx; }
.greeting { font-size: 40rpx; font-weight: bold; color: #333; display: block; }
.subtitle { font-size: 28rpx; color: #666; margin-top: 8rpx; display: block; }

.stats { display: flex; gap: 20rpx; margin-bottom: 40rpx; }
.stat-card { flex: 1; background: #fff; border-radius: 16rpx; padding: 30rpx; text-align: center; box-shadow: 0 4rpx 16rpx rgba(0,0,0,0.04); }
.stat-num { font-size: 44rpx; font-weight: bold; color: #4a9eff; display: block; }
.stat-label { font-size: 24rpx; color: #999; margin-top: 8rpx; display: block; }

.section { margin-bottom: 40rpx; }
.section-title { font-size: 30rpx; font-weight: 600; color: #333; margin-bottom: 20rpx; display: block; }

.features { display: flex; flex-wrap: wrap; gap: 20rpx; }
.feature-card { width: calc(33.33% - 14rpx); background: #fff; border-radius: 16rpx; padding: 30rpx 0; text-align: center; box-shadow: 0 4rpx 12rpx rgba(0,0,0,0.04); }
.feature-icon { font-size: 48rpx; display: block; }
.feature-title { font-size: 24rpx; color: #666; margin-top: 12rpx; display: block; }

.announce-list { background: #fff; border-radius: 16rpx; overflow: hidden; }
.announce-item { padding: 24rpx 30rpx; border-bottom: 1rpx solid #f0f0f0; display: flex; justify-content: space-between; align-items: center; }
.announce-item:last-child { border-bottom: none; }
.announce-title { font-size: 28rpx; color: #333; flex: 1; }
.announce-time { font-size: 24rpx; color: #999; margin-left: 20rpx; }
</style>
