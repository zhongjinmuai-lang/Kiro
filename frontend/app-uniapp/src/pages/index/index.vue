<template>
  <view class="container">
    <view class="header">
      <text class="title">MU 智能体平台</text>
      <text class="subtitle">自研全能智能体主体框架</text>
    </view>

    <view class="features">
      <view class="feature-card" v-for="item in features" :key="item.title">
        <text class="feature-title">{{ item.title }}</text>
        <text class="feature-desc">{{ item.desc }}</text>
      </view>
    </view>

    <view class="actions">
      <button class="btn-primary" @click="goToWorkspace">进入工作台</button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const features = ref([
  { title: '三级SaaS管控', desc: '开发商→服务商→终端客户' },
  { title: '统一中台', desc: '支付/存储/通知一站式管理' },
  { title: 'AI智能调度', desc: '智能体引擎自动优化' },
  { title: '插件热插拔', desc: '按需加载，灵活扩展' },
])

function goToWorkspace() {
  uni.switchTab({ url: '/pages/workspace/workspace' })
}
</script>

<style>
.container { padding: 40rpx; }
.header { text-align: center; margin-bottom: 60rpx; }
.title { font-size: 48rpx; font-weight: bold; color: #333; display: block; }
.subtitle { font-size: 28rpx; color: #999; margin-top: 16rpx; display: block; }

.features { display: flex; flex-wrap: wrap; gap: 20rpx; margin-bottom: 60rpx; }
.feature-card {
  width: calc(50% - 10rpx);
  background: #fff;
  border-radius: 16rpx;
  padding: 30rpx;
  box-shadow: 0 4rpx 16rpx rgba(0,0,0,0.06);
}
.feature-title { font-size: 30rpx; font-weight: bold; color: #333; display: block; }
.feature-desc { font-size: 24rpx; color: #666; margin-top: 12rpx; display: block; }

.btn-primary {
  width: 100%;
  height: 88rpx;
  line-height: 88rpx;
  background: #4a9eff;
  color: #fff;
  border-radius: 44rpx;
  font-size: 32rpx;
}
</style>
