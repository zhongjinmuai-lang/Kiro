<template>
  <view class="page">
    <view class="header">
      <text class="title">消息中心</text>
    </view>

    <view class="msg-list">
      <view class="msg-item" v-for="m in messages" :key="m.id">
        <view class="msg-dot" :class="{ unread: !m.read_at }"></view>
        <view class="msg-content">
          <text class="msg-text">{{ m.content }}</text>
          <text class="msg-time">{{ formatTime(m.created_at) }}</text>
        </view>
      </view>
      <view v-if="!messages.length" class="empty">
        <text class="empty-icon">📭</text>
        <text class="empty-text">暂无消息</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@/utils/request'

const messages = ref<any[]>([])

function formatTime(t: string) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + '分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + '小时前'
  return t.slice(0, 10)
}

onMounted(async () => {
  try {
    const res = await api.get('/api/v1/messages?page_size=50')
    messages.value = res.data?.list || []
  } catch {}
})
</script>

<style scoped>
.page { padding: 30rpx; background: #f5f7fa; min-height: 100vh; }
.header { margin-bottom: 30rpx; }
.title { font-size: 36rpx; font-weight: bold; color: #333; display: block; }
.msg-list { }
.msg-item { display: flex; align-items: flex-start; background: #fff; border-radius: 12rpx; padding: 24rpx 30rpx; margin-bottom: 16rpx; }
.msg-dot { width: 16rpx; height: 16rpx; border-radius: 50%; background: #ddd; margin-top: 10rpx; margin-right: 20rpx; flex-shrink: 0; }
.msg-dot.unread { background: #4a9eff; }
.msg-content { flex: 1; }
.msg-text { font-size: 28rpx; color: #333; display: block; line-height: 1.5; }
.msg-time { font-size: 24rpx; color: #999; margin-top: 8rpx; display: block; }
.empty { text-align: center; padding: 120rpx 0; }
.empty-icon { font-size: 80rpx; display: block; margin-bottom: 20rpx; }
.empty-text { font-size: 28rpx; color: #999; }
</style>
