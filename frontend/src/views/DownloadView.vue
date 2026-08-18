<script setup lang="ts">
import AppIcon from '@/components/AppIcon.vue'
const platforms = [
  { icon: 'download', title: 'Android', text: '安卓手机和平板', note: '安装包准备中' },
  { icon: 'image', title: 'iOS / iPadOS', text: '苹果手机和平板', note: '即将上架 App Store' },
  { icon: 'document', title: 'Windows / macOS', text: '电脑端客户端', note: '桌面版准备中' },
]
</script>
<template>
  <section class="rf-page download-page">
    <header class="download-hero"><p class="download-kicker">ROB FORUM CLIENT</p><h1>客户端下载</h1><p>在更顺手的设备上访问罗布玩家社区。各平台安装包正在整理中，发布后会第一时间更新下载地址。</p><RouterLink to="/" class="rf-primary-button"><AppIcon name="home" size="16" />继续使用网页版</RouterLink></header>
    <section class="download-grid" aria-label="客户端平台"><article v-for="item in platforms" :key="item.title" class="download-card"><div class="download-icon"><AppIcon :name="item.icon" size="28" /></div><div><h2>{{ item.title }}</h2><p>{{ item.text }}</p></div><span class="download-status">{{ item.note }}</span></article></section>
    <aside class="download-tip"><AppIcon name="notice" size="18" /><div><strong>安全提示</strong><p>请只从本页面或官方公告获取安装包，暂时不要下载来历不明的第三方客户端。</p></div></aside>
  </section>
</template>
<style scoped>
.download-page { padding: 34px 22px 56px; }.download-hero { max-width: 680px; margin: 0 auto; text-align: center; }.download-kicker { margin: 0 0 8px; color: var(--primary); font-size: 11px; font-weight: 800; letter-spacing: .12em; }.download-hero h1 { margin: 0; font-size: clamp(30px, 5vw, 46px); }.download-hero p:not(.download-kicker) { margin: 12px auto 20px; color: var(--rf-muted); line-height: 1.75; }.download-hero .rf-primary-button { display: inline-flex; align-items: center; gap: 6px; }.download-grid { display: grid; max-width: 880px; margin: 34px auto 20px; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }.download-card { display: grid; min-height: 180px; align-content: start; gap: 15px; padding: 20px; border: 1px solid var(--rf-line); border-radius: 14px; background: var(--rf-bg); box-shadow: 0 8px 24px rgb(15 23 42 / 5%); }.download-icon { display: grid; width: 52px; height: 52px; place-items: center; border-radius: 14px; color: var(--primary); background: color-mix(in srgb, var(--primary) 11%, transparent); }.download-card h2 { margin: 0; font-size: 18px; }.download-card p { margin: 4px 0 0; color: var(--rf-muted); font-size: 12px; }.download-status { width: max-content; margin-top: auto; padding: 4px 8px; border-radius: 99px; color: var(--rf-muted); background: var(--rf-bg-subtle); font-size: 11px; }.download-tip { display: flex; max-width: 880px; margin: 18px auto 0; gap: 10px; padding: 14px 16px; border: 1px solid color-mix(in srgb, var(--primary) 18%, var(--rf-line)); border-radius: 10px; color: var(--primary); background: color-mix(in srgb, var(--primary) 5%, var(--rf-bg)); }.download-tip strong { font-size: 13px; }.download-tip p { margin: 3px 0 0; color: var(--rf-muted); font-size: 12px; line-height: 1.6; }
@media (max-width: 700px) { .download-page { padding: 24px 12px 40px; }.download-grid { grid-template-columns: 1fr; margin-top: 24px; }.download-card { min-height: 0; grid-template-columns: 52px minmax(0, 1fr); }.download-icon { grid-row: span 2; }.download-status { grid-column: 2; margin-top: -7px; } }
</style>
