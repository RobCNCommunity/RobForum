<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchRobloxNewsById, type RobloxNews, type RobloxNewsMedia } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PageContainer from '@/components/PageContainer.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const news = ref<RobloxNews | null>(null)
const loading = ref(true)
const error = ref('')

const newsID = computed(() => Number(route.params.id))
const coverImage = computed(() => {
  if (!news.value?.media?.length) return null
  return news.value.media[0]
})
const otherImages = computed(() => {
  if (!news.value?.media?.length) return []
  return news.value.media.slice(1)
})
const plainText = computed(() => {
  if (!news.value) return ''
  const md = news.value.content || ''
  return md
    .replace(/#+ /g, '')
    .replace(/\*\*|__/g, '')
    .replace(/\n/g, ' ')
    .trim()
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    news.value = await fetchRobloxNewsById(newsID.value)
    if (news.value && !news.value.content) {
      // Some old items may have content empty; show title only
    }
  } catch (cause) {
    error.value = errorMessage(cause, '新闻加载失败')
    Notify.danger(error.value, { duration: 3000 })
  } finally {
    loading.value = false
  }
}

function formatDate(value: string) {
  try {
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}

function openExternal(url: string) {
  if (url) window.open(url, '_blank', 'noopener,noreferrer')
}

function imageUrl(item: RobloxNewsMedia, maxH = 0) {
  let url = item.url || ''
  if (maxH > 0 && url.startsWith('http')) {
    return `${url}?h=${maxH}`
  }
  return url
}

watch(newsID, load)
onMounted(load)
</script>

<template>
  <PageContainer>
    <header class="rf-nd-header">
      <button type="button" class="rf-nd-back" @click="router.back()" aria-label="返回">
        <AppIcon name="back" size="20" />
      </button>
      <span v-if="loading" class="rf-nd-title">新闻详情</span>
      <span v-else class="rf-nd-title">{{ news?.title || '新闻详情' }}</span>
    </header>

    <section v-if="error" class="rf-nd-error">
      <AppIcon name="error" size="32" />
      <p>{{ error }}</p>
      <button type="button" class="rf-text-button" @click="load">重试</button>
    </section>

    <article v-else-if="!loading && news" class="rf-nd-article">
      <div v-if="coverImage" class="rf-nd-cover">
        <img :src="imageUrl(coverImage, 480)" :alt="news.title" loading="eager" />
      </div>

      <div class="rf-nd-body">
        <div class="rf-nd-meta">
          <span class="rf-nd-tag"><AppIcon name="flame" size="13" />Roblox 动态</span>
          <time>{{ formatDate(news.updated_at || news.created_at) }}</time>
        </div>

        <h1 class="rf-nd-headline">{{ news.title }}</h1>

        <div v-if="plainText.length > 200" class="rf-nd-summary">
          {{ plainText.slice(0, 180) }}…
        </div>

        <MarkdownContent :source="news.content" class="rf-nd-content" />

        <div v-if="otherImages.length" class="rf-nd-gallery">
          <img
            v-for="img in otherImages"
            :key="img.id"
            :src="imageUrl(img, 400)"
            alt=""
            loading="lazy"
          />
        </div>

        <div v-if="news.link_url" class="rf-nd-source">
          <a
            :href="news.link_url"
            target="_blank"
            rel="noopener noreferrer"
            @click.prevent="openExternal(news.link_url)"
          >
            <AppIcon name="link" size="14" />阅读原文
          </a>
        </div>
      </div>
    </article>

    <div v-else-if="loading" class="rf-nd-loading">
      <nut-skeleton v-for="n in 5" :key="n" animated title row="2" />
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-nd-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--rf-line);
}
.rf-nd-back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border-radius: 10px;
  color: var(--rf-text);
  background: var(--rf-bg-subtle);
  border: none;
  cursor: pointer;
  flex-shrink: 0;
}
.rf-nd-back:hover {
  background: var(--rf-line);
}
.rf-nd-title {
  font-size: 16px;
  font-weight: 700;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rf-nd-article {
  max-width: 720px;
  margin: 0 auto;
}
.rf-nd-cover {
  width: 100%;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background: var(--rf-bg-subtle);
}
.rf-nd-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.rf-nd-body {
  padding: 16px;
}
.rf-nd-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.rf-nd-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--primary);
  font-size: 12px;
  font-weight: 700;
}
.rf-nd-meta time {
  color: var(--rf-faint);
  font-size: 12px;
}
.rf-nd-headline {
  margin: 0 0 12px;
  font-size: 22px;
  font-weight: 800;
  line-height: 1.35;
}
.rf-nd-summary {
  margin-bottom: 14px;
  color: var(--rf-muted);
  font-size: 15px;
  line-height: 1.6;
}
.rf-nd-content :deep(.md-content) {
  font-size: 16px;
  line-height: 1.8;
  color: var(--rf-text);
}
.rf-nd-content :deep(.md-content p) {
  margin-bottom: 14px;
}
.rf-nd-content :deep(.md-content h2),
.rf-nd-content :deep(.md-content h3) {
  margin: 20px 0 10px;
  font-weight: 700;
}
.rf-nd-gallery {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 16px;
}
.rf-nd-gallery img {
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
  border-radius: 8px;
  background: var(--rf-bg-subtle);
}
.rf-nd-source {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--rf-line);
}
.rf-nd-source a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--primary);
  font-size: 14px;
  font-weight: 700;
  text-decoration: none;
}
.rf-nd-source a:hover {
  text-decoration: underline;
}
.rf-nd-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 16px;
  text-align: center;
  color: var(--rf-muted);
}
.rf-nd-error p {
  font-size: 14px;
}
.rf-nd-loading {
  padding: 16px;
  max-width: 720px;
  margin: 0 auto;
}
</style>
