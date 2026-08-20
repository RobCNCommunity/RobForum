<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import Sortable from 'sortablejs'
import { createAdminRobloxNews, deleteAdminRobloxNews, errorMessage, fetchAdminRobloxNews, updateAdminRobloxNews, uploadAdminRobloxNewsImage, type RobloxNews, type RobloxNewsMedia } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import PageContainer from '@/components/PageContainer.vue'

const loading = ref(true)
const saving = ref(false)
const items = ref<RobloxNews[]>([])
const media = ref<RobloxNewsMedia[]>([])
const mediaGrid = ref<HTMLElement | null>(null)
const mediaInput = ref<HTMLInputElement | null>(null)
const mediaUploading = ref(false)
const previewMode = ref(false)
let mediaSortable: Sortable | undefined
const form = reactive({ id: 0, title: '', content: '', link_url: '', enabled: true })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminRobloxNews()
  } catch (error) {
    Notify.danger(errorMessage(error, '新闻加载失败'))
  } finally {
    loading.value = false
  }
}

function reset() { Object.assign(form, { id: 0, title: '', content: '', link_url: '', enabled: true }); media.value = []; previewMode.value = false }
function edit(item: RobloxNews) { Object.assign(form, { id: item.id, title: item.title, content: item.content, link_url: item.link_url || '', enabled: item.enabled }); media.value = [...(item.media || [])]; previewMode.value = false; void initializeMediaSorting() }

async function initializeMediaSorting() {
  await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
  mediaSortable?.destroy()
  if (!mediaGrid.value || media.value.length < 2) return
  mediaSortable = Sortable.create(mediaGrid.value, { animation: 160, draggable: 'figure', handle: '.news-media-drag', filter: '.news-media-remove', preventOnFilter: false, onEnd(event) {
    if (event.oldIndex === undefined || event.newIndex === undefined || event.oldIndex === event.newIndex) return
    const next = [...media.value]
    const [moved] = next.splice(event.oldIndex, 1)
    if (moved) next.splice(event.newIndex, 0, moved)
    media.value = next
  } })
}

async function chooseMedia(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || []).slice(0, Math.max(0, 4 - media.value.length))
  input.value = ''
  if (!files.length) return
  mediaUploading.value = true
  try {
    for (const file of files) {
      if (!['image/png', 'image/jpeg'].includes(file.type)) { Notify.warn('新闻图片仅支持 PNG 和 JPG'); continue }
      media.value.push(await uploadAdminRobloxNewsImage(file))
    }
    await initializeMediaSorting()
  } catch (error) { Notify.danger(errorMessage(error, '新闻图片上传失败')) }
  finally { mediaUploading.value = false }
}

function removeMedia(index: number) { media.value.splice(index, 1); void initializeMediaSorting() }

async function save() {
  if (saving.value) return
  if (!form.title.trim() || !form.content.trim()) {
    Notify.warn('标题和内容不能为空')
    return
  }
  saving.value = true
  try {
    const payload = { title: form.title.trim(), content: form.content.trim(), media: media.value, link_url: form.link_url.trim(), enabled: form.enabled }
    if (form.id) await updateAdminRobloxNews(form.id, payload)
    else await createAdminRobloxNews(payload)
    Notify.success('新闻已保存')
    reset()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '新闻保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(id: number) {
  if (!window.confirm('确定删除这条新闻吗？')) return
  try {
    await deleteAdminRobloxNews(id)
    Notify.success('新闻已删除')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '新闻删除失败'))
  }
}

onMounted(load)
onBeforeUnmount(() => mediaSortable?.destroy())
</script>

<template>
  <PageContainer title="新闻管理">
    <div class="admin-grid">
      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="flame" size="18" />{{ form.id ? '编辑新闻' : '发布新闻' }}</h3><p>发布 Roblox 平台动态、版本更新与官方活动消息。</p></div><span class="rf-kicker">{{ form.id ? `#${form.id}` : 'NEW' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="save">
          <label class="rf-field-label"><span>标题</span><input v-model="form.title" class="rf-control" maxlength="120" required /></label>
          <label class="rf-field-label"><span>正文</span><div class="news-editor"><nav><button type="button" :class="{ active: !previewMode }" @click="previewMode = false"><AppIcon name="edit" size="15" />编辑</button><button type="button" :class="{ active: previewMode }" @click="previewMode = true"><AppIcon name="eye" size="15" />预览</button></nav><textarea v-if="!previewMode" v-model="form.content" class="rf-control rf-textarea" rows="7" maxlength="10000" required /><div v-else class="news-editor-preview"><MarkdownContent :source="form.content || '暂无内容'" /></div></div></label>
          <div class="rf-field-label news-media-field"><span>新闻图片（最多 4 张，可拖拽排序）</span><div v-if="media.length" ref="mediaGrid" class="news-media-grid"><figure v-for="(item, index) in media" :key="item.url"><img :src="item.url" alt="" /><button type="button" class="news-media-drag" :aria-label="`拖动第 ${index + 1} 张图片`"><AppIcon name="drag" size="16" /></button><button type="button" class="news-media-remove" :aria-label="`移除第 ${index + 1} 张图片`" @click="removeMedia(index)"><AppIcon name="close" size="15" /></button></figure></div><button type="button" class="rf-secondary-button news-media-picker" :disabled="mediaUploading || media.length >= 4" @click="mediaInput?.click()"><AppIcon name="upload" size="16" />{{ mediaUploading ? '上传中…' : media.length >= 4 ? '已达到 4 张' : '从本地图库添加图片' }}</button><input ref="mediaInput" type="file" accept="image/png,image/jpeg" multiple class="news-media-input" @change="chooseMedia" /></div>
          <label class="rf-field-label"><span>原文链接（可选）</span><input v-model="form.link_url" class="rf-control" type="url" maxlength="500" placeholder="https://devforum.roblox.com/ 或官方公告地址" /></label>
          <div class="rf-choice-row"><label class="rf-switch-row"><input v-model="form.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>公开展示</span></label></div>
          <div class="rf-form-actions"><nut-button type="primary" :loading="saving" :disabled="mediaUploading" @click="save">保存新闻</nut-button><button type="button" class="rf-secondary-button" @click="reset">清空</button></div>
        </form>
      </section>

      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3>新闻列表</h3><p>停用的新闻不会出现在「新闻快报」页面。</p></div><span class="rf-count">{{ items.length }}</span></header>
        <div v-if="loading" class="rf-list-loading"><span v-for="n in 3" :key="n" /></div>
        <div v-else-if="!items.length" class="rf-empty"><AppIcon name="flame" size="28" /><strong>暂无新闻</strong><span>发布一条新闻后会显示在这里。</span></div>
        <div v-else class="rf-admin-list">
          <article v-for="item in items" :key="item.id" class="rf-admin-list-row rf-admin-list-row--stacked">
            <div class="rf-admin-list-copy"><div class="rf-list-title"><strong>{{ item.title }}</strong><span class="rf-status-chip" :class="item.enabled ? 'is-on' : ''">{{ item.enabled ? '启用' : '停用' }}</span></div><small>{{ new Date(item.updated_at || item.created_at).toLocaleString('zh-CN') }}</small><MarkdownContent :source="item.content" compact /><div v-if="item.media?.length" class="news-admin-thumbs"><img v-for="image in item.media" :key="image.url" :src="image.url" alt="" /></div><a v-if="item.link_url" :href="item.link_url" target="_blank" rel="noopener noreferrer" class="rf-notice-admin-link"><AppIcon name="link" size="14" />{{ item.link_url }}</a></div>
            <div class="rf-row-actions"><button type="button" class="rf-secondary-button rf-button-small" @click="edit(item)">编辑</button><button type="button" class="rf-danger-button rf-button-small" @click="remove(item.id)">删除</button></div>
          </article>
        </div>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-notice-admin-link { display: inline-flex; max-width: 100%; align-items: center; gap: 5px; margin-top: 7px; overflow: hidden; color: var(--primary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.news-editor { overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg); }
.news-editor nav { display: flex; gap: 2px; padding: 4px; border-bottom: 1px solid var(--rf-line); background: var(--rf-bg-subtle); }
.news-editor nav button { display: inline-flex; align-items: center; gap: 5px; padding: 7px 10px; border: 0; border-radius: 6px; color: var(--rf-muted); background: transparent; font-size: 12px; font-weight: 700; }
.news-editor nav button.active { color: var(--primary); background: var(--rf-bg); box-shadow: 0 1px 3px rgb(0 0 0 / 8%); }
.news-editor .rf-textarea { min-height: 170px; border: 0; border-radius: 0; resize: vertical; }
.news-editor-preview { min-height: 170px; padding: 13px; }
.news-media-field { gap: 8px; }
.news-media-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.news-media-grid figure { position: relative; aspect-ratio: 1; margin: 0; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg-subtle); }
.news-media-grid img { width: 100%; height: 100%; object-fit: cover; }
.news-media-drag, .news-media-remove { position: absolute; display: grid; width: 26px; height: 26px; place-items: center; border: 0; border-radius: 50%; color: #fff; background: rgb(0 0 0 / 55%); }
.news-media-drag { right: 5px; bottom: 5px; cursor: grab; }
.news-media-remove { top: 5px; right: 5px; cursor: pointer; }
.news-media-picker { width: max-content; }
.news-media-input { display: none; }
.news-admin-thumbs { display: grid; grid-template-columns: repeat(4, 58px); gap: 5px; margin-top: 8px; }
.news-admin-thumbs img { width: 58px; height: 58px; border-radius: 5px; object-fit: cover; }
@media (max-width: 520px) { .news-media-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); max-width: 260px; }.news-media-picker { width: 100%; }.news-admin-thumbs { grid-template-columns: repeat(4, 48px); }.news-admin-thumbs img { width: 48px; height: 48px; } }
</style>
