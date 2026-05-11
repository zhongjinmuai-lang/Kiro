<template>
  <view class="login-page">
    <view class="logo-area">
      <text class="logo-text">MU</text>
      <text class="logo-sub">智能体族谱平台</text>
    </view>

    <view class="form">
      <view class="input-group">
        <text class="label">家族编码</text>
        <input v-model="form.tenantCode" placeholder="请输入家族编码" class="input" />
      </view>
      <view class="input-group">
        <text class="label">用户名</text>
        <input v-model="form.username" placeholder="请输入用户名" class="input" />
      </view>
      <view class="input-group">
        <text class="label">密码</text>
        <input v-model="form.password" type="password" placeholder="请输入密码" class="input" />
      </view>

      <button class="btn-login" :loading="loading" :disabled="loading" @click="handleLogin">
        {{ loading ? '登录中...' : '登 录' }}
      </button>

      <view class="tips">
        <text class="tip-text">默认账号：admin / admin123</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, saveAuth } from '@/utils/request'

const form = reactive({
  tenantCode: '',
  username: 'admin',
  password: '',
})
const loading = ref(false)

async function handleLogin() {
  if (!form.tenantCode) return uni.showToast({ title: '请输入家族编码', icon: 'none' })
  if (!form.username) return uni.showToast({ title: '请输入用户名', icon: 'none' })
  if (!form.password) return uni.showToast({ title: '请输入密码', icon: 'none' })

  loading.value = true
  try {
    const res = await api.post('/api/v1/auth/login', {
      tenant_code: form.tenantCode,
      username: form.username,
      password: form.password,
    })
    const { token, user } = res.data
    saveAuth(token.access_token, token.refresh_token)
    // 缓存用户信息
    uni.setStorageSync('mu_user', JSON.stringify(user))
    uni.showToast({ title: '登录成功', icon: 'success' })
    setTimeout(() => {
      uni.switchTab({ url: '/pages/index/index' })
    }, 500)
  } catch (e: any) {
    // 错误已在 request 层处理
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page { min-height: 100vh; background: linear-gradient(180deg, #f0f7ff 0%, #fff 100%); padding: 120rpx 60rpx 0; }
.logo-area { text-align: center; margin-bottom: 80rpx; }
.logo-text { font-size: 80rpx; font-weight: bold; color: #4a9eff; display: block; }
.logo-sub { font-size: 28rpx; color: #666; margin-top: 12rpx; display: block; }
.form { background: #fff; border-radius: 24rpx; padding: 48rpx 40rpx; box-shadow: 0 8rpx 32rpx rgba(74,158,255,0.08); }
.input-group { margin-bottom: 36rpx; }
.label { font-size: 26rpx; color: #333; margin-bottom: 12rpx; display: block; }
.input { width: 100%; height: 80rpx; border: 1rpx solid #e8e8e8; border-radius: 12rpx; padding: 0 24rpx; font-size: 28rpx; }
.btn-login { width: 100%; height: 88rpx; line-height: 88rpx; background: #4a9eff; color: #fff; border-radius: 44rpx; font-size: 32rpx; margin-top: 20rpx; border: none; }
.btn-login[disabled] { opacity: 0.6; }
.tips { text-align: center; margin-top: 30rpx; }
.tip-text { font-size: 24rpx; color: #999; }
</style>
