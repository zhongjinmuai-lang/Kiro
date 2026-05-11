<template>
  <view class="page">
    <view class="profile-card">
      <view class="avatar">
        <text class="avatar-text">{{ user?.nickname?.charAt(0) || user?.username?.charAt(0) || '?' }}</text>
      </view>
      <view class="profile-info">
        <text class="name">{{ user?.nickname || user?.username || '未登录' }}</text>
        <text class="email">{{ user?.email || '' }}</text>
      </view>
    </view>

    <view class="menu-list">
      <view class="menu-item" @click="handleChangePassword">
        <text class="menu-icon">🔑</text>
        <text class="menu-text">修改密码</text>
        <text class="menu-arrow">›</text>
      </view>
      <view class="menu-item" @click="handleAbout">
        <text class="menu-icon">ℹ️</text>
        <text class="menu-text">关于</text>
        <text class="menu-arrow">›</text>
      </view>
    </view>

    <button class="btn-logout" @click="handleLogout">退出登录</button>
  </view>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { clearAuth } from '@/utils/request'

const user = ref<any>(null)

onMounted(() => {
  const cached = uni.getStorageSync('mu_user')
  if (cached) user.value = JSON.parse(cached)
})

function handleChangePassword() {
  uni.showToast({ title: '开发中', icon: 'none' })
}

function handleAbout() {
  uni.showModal({
    title: 'MU 智能体族谱平台',
    content: '版本：v2.6.0\n框架：MU Framework\n技术：UniApp X + Vue3',
    showCancel: false,
  })
}

function handleLogout() {
  uni.showModal({
    title: '确认退出',
    content: '退出后需要重新登录',
    success: (res) => {
      if (res.confirm) {
        clearAuth()
        uni.removeStorageSync('mu_user')
        uni.reLaunch({ url: '/pages/login/login' })
      }
    },
  })
}
</script>

<style scoped>
.page { padding: 30rpx; background: #f5f7fa; min-height: 100vh; }
.profile-card { display: flex; align-items: center; background: #fff; border-radius: 20rpx; padding: 40rpx 30rpx; margin-bottom: 30rpx; box-shadow: 0 4rpx 16rpx rgba(0,0,0,0.04); }
.avatar { width: 100rpx; height: 100rpx; border-radius: 50%; background: linear-gradient(135deg, #4a9eff, #67c8ff); display: flex; align-items: center; justify-content: center; margin-right: 30rpx; }
.avatar-text { font-size: 40rpx; color: #fff; font-weight: bold; }
.profile-info { flex: 1; }
.name { font-size: 34rpx; font-weight: 600; color: #333; display: block; }
.email { font-size: 26rpx; color: #999; margin-top: 8rpx; display: block; }
.menu-list { background: #fff; border-radius: 16rpx; margin-bottom: 40rpx; overflow: hidden; }
.menu-item { display: flex; align-items: center; padding: 32rpx 30rpx; border-bottom: 1rpx solid #f5f5f5; }
.menu-item:last-child { border-bottom: none; }
.menu-icon { font-size: 36rpx; margin-right: 20rpx; }
.menu-text { flex: 1; font-size: 30rpx; color: #333; }
.menu-arrow { font-size: 32rpx; color: #ccc; }
.btn-logout { width: 100%; height: 88rpx; line-height: 88rpx; background: #fff; color: #ff4d4f; border-radius: 44rpx; font-size: 30rpx; border: 1rpx solid #ff4d4f; }
</style>
