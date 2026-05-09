<template>
  <div class="page">
    <h1>存储中台</h1>
    <p class="hint">开发商准入：阿里 OSS / 腾讯 COS / 七牛 / 华为 OBS，服务商可绑定自有存储。</p>
    <div class="card">
      <h3>支持的存储厂商</h3>
      <div class="providers">
        <div class="prov" v-for="p in providers" :key="p.code">
          <div class="name">{{ p.name }}</div>
          <div class="desc">{{ p.desc }}</div>
          <span class="status">{{ p.enabled ? '已启用' : '未启用' }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
const providers = [
  { code: 'local',       name: '本地磁盘',     desc: '开发 / 单机部署', enabled: true },
  { code: 'aliyun_oss',  name: '阿里云 OSS',  desc: '接入 aliyun-oss-go-sdk', enabled: true },
  { code: 'tencent_cos', name: '腾讯云 COS',  desc: '接入 cos-go-sdk-v5',     enabled: true },
  { code: 'qiniu',       name: '七牛云',       desc: '接入 qiniu/go-sdk',       enabled: true },
  { code: 'huawei_obs',  name: '华为云 OBS',  desc: '接入 huaweicloud obs sdk', enabled: true },
  { code: 'minio',       name: 'MinIO',       desc: '私有化 S3 兼容',          enabled: true },
]
</script>
<style scoped>
.page h1 { margin-bottom:8px; } .hint { color:#888;margin-bottom:20px; }
.card { background:#fff;padding:24px;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.06); }
.card h3 { margin-bottom:16px; }
.providers { display:grid;grid-template-columns:repeat(3,1fr);gap:16px; }
.prov { border:1px solid #f0f0f0;border-radius:8px;padding:16px; }
.name { font-weight:bold;margin-bottom:4px; } .desc { color:#888;font-size:13px;margin-bottom:8px; }
.status { color:#52c41a;font-size:12px; }
</style>
