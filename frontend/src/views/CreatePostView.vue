<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { createPost, errorMessage, fetchBoards, type Board } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import QuotedPostContext from '@/components/QuotedPostContext.vue'
import { useAuthStore } from '@/stores/auth'

interface SelectedMedia {
  file: File
  url: string
}

const router = useRouter()
const auth = useAuthStore()
const formRef = ref<{ validate: () => Promise<unknown> } | null>(null)
const boards = ref<Board[]>([])
const mediaInput = ref<HTMLInputElement | null>(null)
const media = ref<SelectedMedia[]>([])
const tags = ref<string[]>([])
const tagDraft = ref('')
const isMobile = ref(false)
const loading = ref(false)
const editorMode = ref<'edit' | 'preview'>('edit')
const form = reactive({
  board_id: undefined as number | undefined,
  title: '',
  content: '',
})
let mobileQuery: MediaQueryList | undefined

const selectedCount = computed(() => media.value.length)
const rules = {
  title: [
    { max: 180, message: '标题不能超过 180 个字符' },
  ],
  content: [
    { required: true, message: '请输入正文' },
    { max: 50000, message: '正文不能超过 50000 个字符' },
  ],
}

function updateMobile(event: MediaQueryListEvent | MediaQueryList) {
  isMobile.value = event.matches
}

function isVideo(file: File) {
  return file.type.startsWith('video/')
}

function prefillQuote(content: string) {
  if (!form.content) form.content = content
}

function handleEditorTabKeydown(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return
  if (!(event.currentTarget instanceof HTMLButtonElement)) return
  event.preventDefault()
  const nextMode = editorMode.value === 'edit' ? 'preview' : 'edit'
  editorMode.value = nextMode
  event.currentTarget.parentElement
    ?.querySelector<HTMLButtonElement>(`[data-editor-mode="${nextMode}"]`)
    ?.focus()
}

function chooseMedia(event: Event) {
  const input = event.target as HTMLInputElement
  const remaining = 4 - media.value.length
  const candidates = Array.from(input.files || []).slice(0, Math.max(0, remaining))
  if ((input.files?.length || 0) > remaining) Notify.warn('每篇帖子最多上传 4 个媒体文件')
  for (const file of candidates) {
    const image = ['image/png', 'image/jpeg'].includes(file.type) && /\.(png|jpe?g)$/i.test(file.name)
    const video = ['video/mp4', 'video/webm'].includes(file.type) && /\.(mp4|webm)$/i.test(file.name)
    const withinLimit = image ? file.size <= 5 * 1024 * 1024 : video && file.size <= 50 * 1024 * 1024
    if (!withinLimit) {
      Notify.warn('仅支持 PNG/JPG（5 MB 内）或 MP4/WebM（50 MB 内）')
      continue
    }
    media.value.push({ file, url: URL.createObjectURL(file) })
  }
  input.value = ''
}

function removeMedia(index: number) {
  const item = media.value[index]
  if (item) URL.revokeObjectURL(item.url)
  media.value.splice(index, 1)
}

function addTag() {
  const value = tagDraft.value.trim().replace(/^#+/, '').toLocaleLowerCase()
  if (!value) return
  if (tags.value.length >= 5) {
    Notify.warn('每篇帖子最多添加 5 个标签')
    return
  }
  if ([...value].length > 24 || /[\s#]/u.test(value)) {
    Notify.warn('标签不能包含空格或 #，且不能超过 24 个字符')
    return
  }
  if (!tags.value.includes(value)) tags.value.push(value)
  tagDraft.value = ''
}

function removeTag(index: number) {
  tags.value.splice(index, 1)
}

function handleTagKeydown(event: KeyboardEvent) {
  if (!['Enter', ',', '，'].includes(event.key)) return
  event.preventDefault()
  addTag()
}

async function submit() {
  if (loading.value) return
  loading.value = true
  try {
    if (!form.board_id) {
      Notify.warn('请选择板块')
      return
    }
    if (!form.content.trim()) {
      Notify.warn('请输入正文')
      return
    }
    if ([...form.title].length > 180 || [...form.content].length > 50000) {
      Notify.warn('标题或正文超过长度限制')
      return
    }
    if (formRef.value) {
      try {
        await formRef.value.validate()
      } catch {
        return
      }
    }
    const post = await createPost({
      board_id: form.board_id,
      title: form.title.trim(),
      content: form.content.trim(),
      tags: tags.value,
      files: media.value.map((item) => item.file),
    })
    if (post.status === 'pending') {
      Notify.success('帖子已提交审核，审核通过后会公开显示')
      await router.push('/')
    } else {
      Notify.success('帖子已发布')
      await router.push(`/posts/${post.id}`)
    }
  } catch (error) {
    Notify.danger(errorMessage(error, '帖子发布失败'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  mobileQuery = window.matchMedia('(max-width: 640px)')
  updateMobile(mobileQuery)
  mobileQuery.addEventListener('change', updateMobile)
  try {
    boards.value = await fetchBoards()
  } catch (error) {
    Notify.danger(errorMessage(error, '板块加载失败'))
  }
})

onBeforeUnmount(() => {
  mobileQuery?.removeEventListener('change', updateMobile)
  media.value.forEach((item) => URL.revokeObjectURL(item.url))
})
</script>

<template>
  <PageContainer>
    <section v-if="isMobile" class="rf-mobile-composer">
      <header>
        <button type="button" class="rf-icon-button" aria-label="取消发布" :disabled="loading" @click="router.back()"><AppIcon name="back" size="21" /></button>
        <strong>新帖子</strong>
        <button type="button" class="rf-mobile-publish" :disabled="loading || !form.board_id || !form.content.trim()" @click="submit">{{ loading ? '发布中' : '发布' }}</button>
      </header>

      <label class="rf-mobile-board">
        <AppIcon name="category" size="17" />
        <select v-model="form.board_id" required aria-label="选择板块">
          <option :value="undefined" disabled>选择发布板块</option>
          <option v-for="board in boards" :key="board.id" :value="board.id">{{ board.name }}</option>
        </select>
        <AppIcon name="chevron" size="16" />
      </label>

      <QuotedPostContext @loaded="prefillQuote" />

      <div class="rf-mobile-editor">
        <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="40" />
        <div>
          <input v-model="form.title" maxlength="180" placeholder="添加标题（可选）" aria-label="帖子标题，可选" />
          <div class="rf-markdown-mode" role="tablist" aria-label="正文模式">
            <button type="button" role="tab" data-editor-mode="edit" :tabindex="editorMode === 'edit' ? 0 : -1" :aria-selected="editorMode === 'edit'" :class="{ active: editorMode === 'edit' }" @click="editorMode = 'edit'" @keydown="handleEditorTabKeydown"><AppIcon name="edit" size="15" />编辑</button>
            <button type="button" role="tab" data-editor-mode="preview" :tabindex="editorMode === 'preview' ? 0 : -1" :aria-selected="editorMode === 'preview'" :class="{ active: editorMode === 'preview' }" @click="editorMode = 'preview'" @keydown="handleEditorTabKeydown"><AppIcon name="eye" size="15" />预览</button>
          </div>
          <textarea v-show="editorMode === 'edit'" v-model="form.content" rows="9" maxlength="50000" autofocus placeholder="分享你的想法" aria-label="帖子正文，支持 Markdown" />
          <div v-show="editorMode === 'preview'" class="rf-markdown-preview">
            <MarkdownContent v-if="form.content.trim()" :source="form.content" />
            <span v-else>暂无内容</span>
          </div>
        </div>
      </div>

      <div v-if="tags.length" class="rf-tag-list" aria-label="已添加标签">
        <button v-for="(tag, index) in tags" :key="tag" type="button" :aria-label="`移除标签 ${tag}`" @click="removeTag(index)">#{{ tag }}<AppIcon name="close" size="13" /></button>
      </div>
      <div v-if="media.length" class="rf-selected-media">
        <figure v-for="(item, index) in media" :key="item.url">
          <video v-if="isVideo(item.file)" :src="item.url" controls playsinline preload="metadata" />
          <img v-else :src="item.url" alt="待上传图片" />
          <button type="button" aria-label="移除媒体" @click="removeMedia(index)"><AppIcon name="close" size="16" /></button>
        </figure>
      </div>

      <footer class="rf-mobile-tools">
        <label title="添加图片或视频" aria-label="添加图片或视频"><AppIcon name="photo" size="21" /><input ref="mediaInput" type="file" accept="image/png,image/jpeg,video/mp4,video/webm" multiple @change="chooseMedia" /></label>
        <label class="rf-mobile-tag-input"><AppIcon name="tag" size="19" /><input v-model="tagDraft" maxlength="25" placeholder="添加标签" @keydown="handleTagKeydown" /><button v-if="tagDraft" type="button" aria-label="确认添加标签" @click="addTag"><AppIcon name="add" size="17" /></button></label>
        <span>{{ selectedCount }}/4</span>
      </footer>
    </section>

    <section v-else class="rf-create-post">
      <header class="rf-create-heading"><h1>发布帖子</h1></header>
      <QuotedPostContext @loaded="prefillQuote" />
      <nut-form ref="formRef" :model-value="form" :rules="rules" class="rf-create-form">
        <nut-form-item label="板块" required>
          <select v-model="form.board_id" class="rf-board-select" required aria-label="选择板块">
            <option :value="undefined" disabled>选择板块</option>
            <option v-for="board in boards" :key="board.id" :value="board.id">{{ board.name }}</option>
          </select>
        </nut-form-item>
        <nut-form-item label="标题" prop="title">
          <input v-model="form.title" class="rf-create-title-input" maxlength="180" placeholder="标题（可选）" />
        </nut-form-item>
        <nut-form-item label="正文" prop="content" class="rf-content-item">
          <div class="rf-markdown-editor">
            <div class="rf-markdown-mode" role="tablist" aria-label="正文模式">
              <button type="button" role="tab" data-editor-mode="edit" :tabindex="editorMode === 'edit' ? 0 : -1" :aria-selected="editorMode === 'edit'" :class="{ active: editorMode === 'edit' }" @click="editorMode = 'edit'" @keydown="handleEditorTabKeydown"><AppIcon name="edit" size="15" />编辑</button>
              <button type="button" role="tab" data-editor-mode="preview" :tabindex="editorMode === 'preview' ? 0 : -1" :aria-selected="editorMode === 'preview'" :class="{ active: editorMode === 'preview' }" @click="editorMode = 'preview'" @keydown="handleEditorTabKeydown"><AppIcon name="eye" size="15" />预览</button>
            </div>
            <div v-show="editorMode === 'edit'" class="rf-desktop-markdown-input">
              <textarea v-model="form.content" rows="9" maxlength="50000" placeholder="分享你的想法" aria-label="帖子正文，支持 Markdown" />
              <span>{{ form.content.length }}/50000</span>
            </div>
            <div v-show="editorMode === 'preview'" class="rf-markdown-preview">
              <MarkdownContent v-if="form.content.trim()" :source="form.content" />
              <span v-else>暂无内容</span>
            </div>
          </div>
        </nut-form-item>
        <nut-form-item label="标签">
          <div class="rf-tag-editor">
            <div v-if="tags.length" class="rf-tag-list"><button v-for="(tag, index) in tags" :key="tag" type="button" :aria-label="`移除标签 ${tag}`" @click="removeTag(index)">#{{ tag }}<AppIcon name="close" size="13" /></button></div>
            <div class="rf-tag-control"><AppIcon name="tag" size="17" /><input v-model="tagDraft" maxlength="25" placeholder="输入标签后按回车" @keydown="handleTagKeydown" /><button type="button" :disabled="!tagDraft.trim()" @click="addTag">添加</button><span>{{ tags.length }}/5</span></div>
          </div>
        </nut-form-item>
        <nut-form-item class="rf-upload-item">
          <div class="rf-upload-head"><strong>图片和视频</strong><span>{{ selectedCount }}/4</span></div>
          <div v-if="media.length" class="rf-selected-media">
            <figure v-for="(item, index) in media" :key="item.url">
              <video v-if="isVideo(item.file)" :src="item.url" controls playsinline preload="metadata" />
              <img v-else :src="item.url" alt="待上传图片" />
              <button type="button" aria-label="移除媒体" @click="removeMedia(index)"><AppIcon name="close" size="16" /></button>
            </figure>
          </div>
          <label class="rf-media-picker"><AppIcon name="upload" size="19" /><span>选择图片或视频</span><input ref="mediaInput" type="file" accept="image/png,image/jpeg,video/mp4,video/webm" multiple @change="chooseMedia" /></label>
        </nut-form-item>
      </nut-form>
      <footer class="rf-create-actions"><nut-button plain type="default" :disabled="loading" @click="router.back()">取消</nut-button><nut-button type="primary" :loading="loading" @click="submit">发布</nut-button></footer>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-create-post { min-height: 100%; padding: 8px 20px 28px; color: var(--rf-text); background: var(--rf-bg); }
.rf-create-form { color: var(--rf-text); background: transparent; }
.rf-create-form :deep(.nut-cell-group__warp) { margin: 0; overflow: visible; background: transparent; }
.rf-create-form :deep(.nut-form-item) { padding: 17px 0; border-bottom: 1px solid var(--rf-line); background: transparent; }
.rf-create-form :deep(.nut-form-item__label) { width: 72px; color: var(--rf-text); font-weight: 650; }
.rf-create-form :deep(.nut-form-item__body),
.rf-create-form :deep(.nut-form-item__body__slots) { min-width: 0; flex: 1; color: var(--rf-text); background: transparent; }
.rf-create-form :deep(.nut-form-item__body__slots),
.rf-create-form :deep(.nut-input),
.rf-create-form :deep(.nut-textarea),
.rf-create-form :deep(.nut-textarea__textarea) { width: 100%; min-width: 0; }
.rf-create-form :deep(.nut-input),
.rf-create-form :deep(.nut-textarea) { padding: 0; background: transparent; }
.rf-create-form :deep(input),
.rf-create-form :deep(textarea),
.rf-create-form :deep(select) { color: var(--rf-text); caret-color: var(--primary); }
.rf-create-form :deep(input::placeholder),
.rf-create-form :deep(textarea::placeholder) { color: var(--rf-muted); opacity: 1; }
.rf-create-title-input { width: 100%; min-width: 0; height: 44px; padding: 0 12px; border: 0; border-radius: 12px; outline: 0; color: var(--rf-text); background: var(--rf-bg-subtle); }
.rf-create-title-input:focus { box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 15%, transparent); }
.rf-board-select {
  width: 100%;
  min-height: 40px;
  padding: 0 36px 0 12px;
  border: 1px solid var(--rf-line);
  border-radius: 8px;
  color: var(--rf-text);
  background: var(--rf-bg);
}
.rf-board-select option { color: var(--rf-text); background: var(--rf-bg); }
.rf-content-item :deep(.nut-form-item__body) { align-items: flex-start; }
.rf-markdown-editor { width: 100%; min-width: 0; }
.rf-markdown-mode { display: inline-grid; grid-template-columns: repeat(2, minmax(72px, 1fr)); gap: 2px; margin-bottom: 8px; padding: 3px; border: 1px solid var(--rf-line); border-radius: 7px; background: var(--rf-bg-subtle); }
.rf-markdown-mode button { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 6px; padding: 0 11px; border-radius: 5px; color: var(--rf-muted); background: transparent; font-size: 13px; font-weight: 650; }
.rf-markdown-mode button.active { color: var(--rf-text); background: var(--rf-bg); box-shadow: 0 1px 3px color-mix(in srgb, var(--rf-text) 10%, transparent); }
.rf-markdown-preview { min-height: 254px; padding: 12px; overflow: auto; border: 1px solid var(--rf-line); border-radius: 10px; color: var(--rf-text); background: var(--rf-bg-subtle); line-height: 1.65; }
.rf-markdown-preview > span { color: var(--rf-muted); }
.rf-desktop-markdown-input { position: relative; overflow: hidden; padding: 12px 12px 32px; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg-subtle); }
.rf-desktop-markdown-input textarea { display: block; width: 100%; min-height: 220px; padding: 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font: inherit; line-height: 1.65; resize: vertical; }
.rf-desktop-markdown-input > span { position: absolute; right: 12px; bottom: 9px; color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-upload-item :deep(.nut-form-item__body),
.rf-upload-item :deep(.nut-form-item__body__slots) { width: 100%; }
.rf-upload-head { display: flex; align-items: center; justify-content: space-between; width: 100%; margin-bottom: 11px; }
.rf-upload-head span { color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-upload-item :deep(.nut-uploader__preview) { border-radius: 10px; }
.rf-upload-item :deep(.nut-uploader),
.rf-upload-item :deep(.nut-uploader__preview-list) { color: var(--rf-text); background: transparent !important; }
.rf-upload-item :deep(.nut-uploader__upload) { color: var(--rf-muted); background: var(--rf-bg-subtle) !important; }
.rf-create-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 20px;
}
.rf-create-actions .nut-button { min-width: 92px; }
@media (max-width: 560px) {
  .rf-create-post { padding: 0 12px 22px; }
  .rf-create-form :deep(.nut-form-item) { display: block; padding: 15px 0; }
  .rf-create-form :deep(.nut-form-item__label) { width: 100%; margin-right: 0; margin-bottom: 9px; }
  .rf-create-form :deep(.nut-form-item__body),
  .rf-create-form :deep(.nut-form-item__body__slots),
  .rf-create-form :deep(.nut-input),
  .rf-create-form :deep(.nut-textarea),
  .rf-create-form :deep(.nut-textarea__textarea) { width: 100%; min-width: 0; max-width: none; margin-left: 0; }
  .rf-create-form :deep(input),
  .rf-create-form :deep(textarea),
  .rf-board-select { width: 100%; min-width: 0; font-size: 16px; }
  .rf-upload-item :deep(.nut-uploader) { width: 100%; }
  .rf-upload-item :deep(.nut-uploader__preview-list) { display: grid; width: 100%; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .rf-content-item :deep(.nut-textarea__textarea) { min-height: 180px; }
  .rf-create-actions { position: static; padding: 18px 0 8px; background: transparent; }
  .rf-create-actions .nut-button { flex: 1; }
}
.rf-create-heading { min-height: 62px; padding: 16px 0 10px; border-bottom: 1px solid var(--rf-line); }
.rf-create-heading h1 { margin: 0; font-size: 22px; letter-spacing: 0; }
.rf-tag-editor { width: 100%; min-width: 0; }
.rf-tag-list { display: flex; flex-wrap: wrap; gap: 7px; }
.rf-tag-list button { display: inline-flex; min-height: 30px; align-items: center; gap: 4px; padding: 0 10px; border: 1px solid color-mix(in srgb, var(--primary) 30%, var(--rf-line)); border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 7%, var(--rf-bg)); font-size: 12px; }
.rf-tag-list button:hover { background: color-mix(in srgb, var(--primary) 12%, var(--rf-bg)); }
.rf-tag-control { display: grid; min-height: 42px; grid-template-columns: 20px minmax(0, 1fr) auto auto; align-items: center; gap: 8px; margin-top: 9px; padding: 0 10px; border: 1px solid var(--rf-line); border-radius: 8px; color: var(--rf-muted); background: var(--rf-bg-subtle); }
.rf-tag-control input { min-width: 0; height: 40px; border: 0; outline: 0; color: var(--rf-text); background: transparent; }
.rf-tag-control button { min-height: 30px; padding: 0 10px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 12px; font-weight: 700; }
.rf-tag-control button:disabled { opacity: .45; }
.rf-tag-control > span { font-size: 11px; font-variant-numeric: tabular-nums; }
.rf-selected-media { display: grid; width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin: 10px 0; }
.rf-selected-media figure { position: relative; aspect-ratio: 16 / 10; margin: 0; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: #000; }
.rf-selected-media img, .rf-selected-media video { display: block; width: 100%; height: 100%; object-fit: contain; }
.rf-selected-media figure > button { position: absolute; top: 6px; right: 6px; display: inline-grid; width: 30px; height: 30px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .74); }
.rf-media-picker { display: inline-flex; min-height: 42px; align-items: center; gap: 8px; padding: 0 14px; border: 1px solid var(--rf-line); border-radius: 8px; color: var(--primary); background: var(--rf-bg); cursor: pointer; font-weight: 650; }
.rf-media-picker:hover { background: var(--rf-bg-hover); }
.rf-media-picker input, .rf-mobile-tools input[type="file"] { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }
.rf-mobile-composer { position: relative; display: flex; min-height: calc(100dvh - var(--rf-mobile-bottom-nav)); flex-direction: column; background: var(--rf-bg); }
.rf-mobile-composer > header { position: sticky; top: 0; z-index: 5; display: grid; min-height: 54px; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 8px; padding: 4px 10px; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(12px); }
.rf-mobile-composer > header strong { justify-self: center; font-size: 17px; }
.rf-mobile-publish { min-width: 66px; min-height: 36px; padding: 0 14px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 13px; font-weight: 700; }
.rf-mobile-publish:disabled { cursor: not-allowed; opacity: .45; }
.rf-mobile-board { display: grid; min-height: 48px; grid-template-columns: 20px minmax(0, 1fr) 16px; align-items: center; gap: 8px; padding: 0 14px; border-bottom: 1px solid var(--rf-line); color: var(--primary); }
.rf-mobile-board select { width: 100%; height: 46px; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 15px; font-weight: 650; appearance: none; }
.rf-mobile-editor { display: grid; flex: 1; grid-template-columns: 40px minmax(0, 1fr); align-items: start; gap: 10px; padding: 14px; }
.rf-mobile-editor > div { min-width: 0; }
.rf-mobile-editor input { width: 100%; height: 40px; padding: 0; border: 0; border-bottom: 1px solid var(--rf-line); outline: 0; color: var(--rf-text); background: transparent; font-size: 17px; font-weight: 650; }
.rf-mobile-editor .rf-markdown-mode { margin: 8px 0 0; }
.rf-mobile-editor textarea { display: block; width: 100%; min-height: 210px; padding: 12px 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 18px; line-height: 1.55; resize: none; }
.rf-mobile-editor .rf-markdown-preview { min-height: 210px; padding: 12px 0; border: 0; background: transparent; font-size: 17px; }
.rf-mobile-composer > .rf-tag-list, .rf-mobile-composer > .rf-selected-media { margin: 0; padding: 0 14px 10px 64px; }
.rf-mobile-tools { position: sticky; bottom: var(--rf-mobile-bottom-nav); z-index: 4; display: grid; min-height: 54px; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 6px; padding: 5px 12px; border-top: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 94%, transparent); backdrop-filter: blur(12px); }
.rf-mobile-tools > label:first-child { display: inline-grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; color: var(--primary); cursor: pointer; }
.rf-mobile-tools > label:first-child:hover { background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-mobile-tag-input { display: grid; min-width: 0; height: 40px; grid-template-columns: 20px minmax(0, 1fr) 30px; align-items: center; gap: 5px; padding: 0 8px; border-radius: var(--rf-pill); color: var(--rf-muted); background: var(--rf-bg-subtle); }
.rf-mobile-tag-input input { min-width: 0; height: 38px; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 14px; }
.rf-mobile-tag-input button { display: inline-grid; width: 28px; height: 28px; place-items: center; border-radius: 50%; color: #fff; background: var(--primary); }
.rf-mobile-tools > span { min-width: 30px; color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; text-align: right; }
@media (max-width: 380px) {
  .rf-mobile-editor { padding-inline: 10px; }
  .rf-mobile-composer > .rf-tag-list, .rf-mobile-composer > .rf-selected-media { padding-left: 60px; padding-right: 10px; }
}
</style>
