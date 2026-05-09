<template>
  <div class="page">
    <h1>通知中台</h1>
    <p class="hint">短信 / 邮件 / 微信推送 / 站内信统一封装，开发商定义全局模板。</p>
    <div class="card">
      <h3>支持的通知通道</h3>
      <ul>
        <li v-for="c in channels" :key="c.type">
          <span class="tag">{{ c.type }}</span>
          <span class="name">{{ c.name }}</span>
          <span class="desc">{{ c.desc }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
<script setup lang="ts">
const channels = [
  { type: 'sms',       name: '短信',           desc: '阿里云短信 / 腾讯云短信 / 自有短信网关' },
  { type: 'email',     name: '邮件',           desc: '基于 SMTP 协议' },
  { type: 'wechat',    name: '微信模板消息',   desc: '公众号 / 小程序订阅消息' },
  { type: 'push',      name: 'App 推送',       desc: '极光 / 友盟 / 厂商推送' },
  { type: 'websocket', name: '站内信',         desc: 'WebSocket 实时推送' },
]
</script>
<style scoped>
.page h1 { margin-bottom:8px; } .hint { color:#888;margin-bottom:20px; }
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:16px; }
ul { list-style:none;padding:0; }
li { padding:10px 0;border-bottom:1px solid #f5f5f5;display:flex;gap:12px;align-items:center; }
.tag { background:#f0f5ff;color:#4a9eff;padding:2px 10px;border-radius:4px;font-size:12px;font-family:monospace; }
.name { font-weight:bold; } .desc { color:#888;font-size:13px; }
</style>
