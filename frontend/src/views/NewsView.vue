<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { errorMessage, fetchNotices, type Notice } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import type { PostMedia } from '@/api'

const notices = ref<Notice[]>([])
const loading = ref(true)
const error = ref('')

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

function levelLabel(level: string) {
  return ({ info: '社区动态', success: '进展发布', warning: '重要提醒', error: '紧急公告' } as Record<string, string>)[level] || '社区动态'
}

function mediaFor(item: Notice): PostMedia[] {
  return (item.media || []).map((image, index) => ({ id: image.id || index + 1, url: image.url, mime_type: image.mime_type, width: image.width, height: image.height, size_bytes: image.size_bytes }))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    notices.value = await fetchNotices()
  } catch (cause) {
    error.value = errorMessage(cause, '新闻快报加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="rf-page news-page">
    <header class="rf-page-header news-header">
      <div class="rf-page-heading"><p class="news-kicker">COMMUNITY NEWSROOM</p><h1>新闻快报</h1><p>社区公告、服务变更和活动消息会集中发布在这里。</p></div>
      <RouterLink to="/" class="rf-secondary-button"><AppIcon name="back" size="16" />返回首页</RouterLink>
    </header>

    <div v-if="error" class="rf-inline-error" role="alert">{{ error }}<button type="button" @click="load">重试</button></div>
    <div v-if="loading" class="news-loading"><nut-skeleton v-for="n in 4" :key="n" animated title row="3" /></div>
    <div v-else-if="!notices.length" class="rf-empty"><AppIcon name="announcement" size="30" /><strong>暂无新闻快报</strong><span>新的社区消息发布后会显示在这里。</span></div>
    <section v-else class="news-list">
      <article v-for="item in notices" :key="item.id" :class="['news-item', `level-${item.level || 'info'}`]">
        <header><div class="news-meta"><span>{{ levelLabel(item.level) }}</span></div><time>{{ formatDate(item.updated_at || item.created_at) }}</time></header>
        <h2>{{ item.title }}</h2>
        <MarkdownContent :source="item.content" />
        <PostMediaGrid :media="mediaFor(item)" />
        <a v-if="item.link_url" :href="item.link_url" target="_blank" rel="noopener noreferrer">查看详情<AppIcon name="arrow" size="15" /></a>
      </article>
    </section>
  </section>
</template>

<style scoped>
.news-page { padding-bottom: 48px; }
.news-header { align-items: flex-start; }
.news-kicker { margin: 0 0 5px; color: var(--primary); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.news-loading { display: grid; gap: 18px; padding: 20px; }
.news-list { border-top: 1px solid var(--rf-line); }
.news-item { position: relative; padding: 22px 20px; border-bottom: 1px solid var(--rf-line); }
.news-item::before { position: absolute; top: 22px; bottom: 22px; left: 0; width: 3px; background: var(--primary); content: ''; }
.news-item.level-success::before { background: var(--rf-success); }
.news-item.level-warning::before { background: #d97706; }
.news-item.level-error::before { background: var(--rf-danger); }
.news-item header, .news-meta { display: flex; align-items: center; gap: 8px; }
.news-item header { justify-content: space-between; }
.news-meta span, .news-meta b { font-size: 11px; font-weight: 700; }
.news-meta span { color: var(--primary); }
.news-meta b { padding: 2px 6px; border-radius: 4px; color: #fff; background: var(--primary); }
.news-item time { color: var(--rf-muted); font-size: 11px; }
.news-item h2 { margin: 10px 0 8px; font-size: 20px; }
.news-item p { margin: 0; color: var(--rf-text); line-height: 1.75; white-space: pre-wrap; }
.news-item a { display: inline-flex; align-items: center; gap: 4px; margin-top: 14px; color: var(--primary); font-size: 13px; font-weight: 700; }
@media (max-width: 650px) { .news-header { align-items: stretch; flex-direction: column; }.news-header .rf-secondary-button { align-self: flex-start; }.news-item { padding-inline: 16px; }.news-item header { align-items: flex-start; flex-direction: column; } }
</style>
