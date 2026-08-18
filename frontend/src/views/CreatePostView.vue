<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import Sortable from 'sortablejs'
import { createPost, errorMessage, fetchBoards, type Board } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import MarkdownContent from '@/components/MarkdownContent.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import QuotedPostContext from '@/components/QuotedPostContext.vue'
import { useAuthStore } from '@/stores/auth'
import { prepareCommunityMediaFile } from '@/lib/videoCompression'

interface SelectedMedia {
  file: File
  url: string
}

const props = withDefaults(defineProps<{ longForm?: boolean }>(), { longForm: false })
const router = useRouter()
const auth = useAuthStore()
const formRef = ref<{ validate: () => Promise<unknown> } | null>(null)
const editorInput = ref<HTMLTextAreaElement | null>(null)
const editorSelectionStart = ref(0)
const boards = ref<Board[]>([])
const mediaInput = ref<HTMLInputElement | null>(null)
const mediaGrid = ref<HTMLElement | null>(null)
const media = ref<SelectedMedia[]>([])
const mediaOrderVersion = ref(0)
const tags = ref<string[]>([])
const tagDraft = ref('')
const isMobile = ref(false)
const loading = ref(false)
const preparingMedia = ref(false)
const mediaPreparationLabel = ref('')
const editorMode = ref<'edit' | 'preview'>('edit')
const form = reactive({
  board_id: undefined as number | undefined,
  title: '',
  content: '',
})
let mobileQuery: MediaQueryList | undefined
let mediaPreparationController: AbortController | undefined
let mediaSortable: Sortable | undefined

const selectedCount = computed(() => media.value.length)
const isLongForm = computed(() => props.longForm)
const composerTitle = computed(() => isLongForm.value ? '发布长文' : '新帖子')
const contentPlaceholder = computed(() => isLongForm.value ? '开始撰写长文，支持 Markdown' : '分享你的想法')
const inlinePreviewContent = computed(() => {
  let content = form.content
  for (let index = 0; index < media.value.length; index += 1) {
    content = content.replaceAll(mediaMarker(index), mediaPreviewPath(index))
  }
  return content
})
const inlinePreviewImages = computed(() => Object.fromEntries(media.value.map((item, index) => [mediaPreviewPath(index), item.url])))
const rules = computed(() => ({
  title: [
    ...(isLongForm.value ? [{ required: true, message: '请输入长文标题' }] : []),
    { max: 180, message: '标题不能超过 180 个字符' },
  ],
  content: [
    { required: true, message: '请输入正文' },
    { max: 50000, message: '正文不能超过 50000 个字符' },
  ],
}))

function updateMobile(event: MediaQueryListEvent | MediaQueryList) {
  isMobile.value = event.matches
}

function isVideo(file: File) {
  return file.type.startsWith('video/')
}

function createMediaPreviewURL(file: File) {
  if (!file.type.startsWith('image/')) return Promise.resolve(URL.createObjectURL(file))
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener('load', () => {
      if (typeof reader.result === 'string') resolve(reader.result)
      else reject(new Error('图片预览加载失败'))
    }, { once: true })
    reader.addEventListener('error', () => reject(new Error('图片预览加载失败')), { once: true })
    reader.readAsDataURL(file)
  })
}

function revokeMediaPreview(item: SelectedMedia | undefined) {
  if (item?.url.startsWith('blob:')) URL.revokeObjectURL(item.url)
}

function mediaMarker(index: number) {
  return `robforum-upload://media/${index}`
}

function mediaPreviewPath(index: number) {
  return `/__robforum_inline_preview__/${index}`
}

function inlineMarkerPresent(index: number) {
  return form.content.includes(mediaMarker(index))
}

function rememberEditorSelection() {
  if (editorInput.value) editorSelectionStart.value = editorInput.value.selectionStart
}

function insertEditorText(text: string) {
  const position = Math.min(editorSelectionStart.value, form.content.length)
  const before = form.content.slice(0, position)
  const after = form.content.slice(position)
  const prefix = before && !before.endsWith('\n') ? '\n\n' : before.endsWith('\n\n') || !before ? '' : '\n'
  const suffix = after && !after.startsWith('\n') ? '\n\n' : after.startsWith('\n\n') || !after ? '' : '\n'
  const insertion = `${prefix}${text}${suffix}`
  form.content = before + insertion + after
  editorSelectionStart.value = position + insertion.length
  editorMode.value = 'edit'
  void nextTick(() => {
    editorInput.value?.focus()
    editorInput.value?.setSelectionRange(editorSelectionStart.value, editorSelectionStart.value)
  })
}

function insertExistingMedia(index: number) {
  if (!isLongForm.value || inlineMarkerPresent(index)) return
  const file = media.value[index]?.file
  if (!file || isVideo(file)) return
  const alt = file.name.replace(/\.[^.]+$/, '').trim() || `图片 ${index + 1}`
  insertEditorText(`![${alt}](${mediaMarker(index)})`)
}

function reorderMedia(oldIndex: number, newIndex: number) {
  if (oldIndex === newIndex || oldIndex < 0 || newIndex < 0 || oldIndex >= media.value.length || newIndex >= media.value.length) return
  const previous = [...media.value]
  const next = [...previous]
  const [moved] = next.splice(oldIndex, 1)
  if (!moved) return
  next.splice(newIndex, 0, moved)

  if (isLongForm.value) {
    previous.forEach((_, index) => {
      form.content = form.content.replaceAll(mediaMarker(index), `robforum-upload://media-reorder/${index}`)
    })
    next.forEach((item, index) => {
      const previousIndex = previous.indexOf(item)
      form.content = form.content.replaceAll(`robforum-upload://media-reorder/${previousIndex}`, mediaMarker(index))
    })
  }
  media.value = next
  mediaOrderVersion.value += 1
  void initializeMediaSorting()
}

function handleMediaDragKeydown(event: KeyboardEvent, index: number) {
  const offsets: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -2, ArrowDown: 2 }
  const offset = offsets[event.key]
  if (!offset) return
  event.preventDefault()
  const target = Math.min(Math.max(index + offset, 0), media.value.length - 1)
  reorderMedia(index, target)
  void nextTick(() => mediaGrid.value?.querySelectorAll<HTMLElement>('.rf-media-drag')[target]?.focus())
}

async function initializeMediaSorting() {
  await nextTick()
  mediaSortable?.destroy()
  mediaSortable = undefined
  if (!mediaGrid.value || media.value.length < 2) return
  mediaSortable = Sortable.create(mediaGrid.value, {
    animation: 160,
    draggable: 'figure',
    handle: '.rf-media-drag',
    filter: '.rf-media-remove, .rf-inline-media-insert, video',
    preventOnFilter: false,
    delay: 120,
    delayOnTouchOnly: true,
    touchStartThreshold: 4,
    fallbackTolerance: 4,
    ghostClass: 'rf-media-ghost',
    chosenClass: 'rf-media-chosen',
    dragClass: 'rf-media-dragging',
    onEnd(event) {
      if (event.oldIndex === undefined || event.newIndex === undefined) return
      reorderMedia(event.oldIndex, event.newIndex)
    },
  })
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

function formatUploadSize(bytes: number) {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

async function chooseMedia(event: Event) {
  const input = event.target as HTMLInputElement
  if (preparingMedia.value) {
    input.value = ''
    return
  }
  const remaining = 4 - media.value.length
  const candidates = Array.from(input.files || []).slice(0, Math.max(0, remaining))
  if ((input.files?.length || 0) > remaining) Notify.warn('每篇帖子最多上传 4 个媒体文件')
  mediaPreparationController = new AbortController()
  preparingMedia.value = true
  try {
    for (let index = 0; index < candidates.length; index += 1) {
      const file = candidates[index]
      if (!file) continue
      if (isLongForm.value && !file.type.startsWith('image/')) {
        Notify.warn('长文正文暂只支持插入图片')
        continue
      }
      mediaPreparationLabel.value = `处理文件 ${index + 1}/${candidates.length}`
      try {
        const prepared = await prepareCommunityMediaFile(file, (progress) => {
          mediaPreparationLabel.value = `压缩视频 ${Math.round(progress * 100)}%`
        }, mediaPreparationController.signal)
        const mediaIndex = media.value.length
        media.value.push({ file: prepared.file, url: await createMediaPreviewURL(prepared.file) })
        if (isLongForm.value) insertExistingMedia(mediaIndex)
        if (prepared.compressed) {
          Notify.success(`视频已压缩：${formatUploadSize(prepared.originalSize)} → ${formatUploadSize(prepared.file.size)}`)
        }
      } catch (error) {
        if (error instanceof DOMException && error.name === 'AbortError') return
        Notify.warn(error instanceof Error ? error.message : '媒体文件处理失败')
      }
    }
  } finally {
    preparingMedia.value = false
    mediaPreparationLabel.value = ''
    mediaPreparationController = undefined
    input.value = ''
  }
}

function removeMedia(index: number) {
  const item = media.value[index]
  revokeMediaPreview(item)
  if (isLongForm.value) {
    const escapedMarker = mediaMarker(index).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    form.content = form.content.replace(new RegExp(`!\\[[^\\]]*\\]\\(${escapedMarker}\\)\\s*`, 'g'), '')
    for (let next = index + 1; next < media.value.length; next += 1) {
      form.content = form.content.replaceAll(mediaMarker(next), mediaMarker(next - 1))
    }
  }
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
  if (loading.value || preparingMedia.value) return
  loading.value = true
  try {
    if (!form.board_id) {
      Notify.warn('请选择板块')
      return
    }
    if (isLongForm.value && !form.title.trim()) {
      Notify.warn('请输入长文标题')
      return
    }
    if (!form.content.trim()) {
      Notify.warn('请输入正文')
      return
    }
    if (isLongForm.value) {
      const missingIndex = media.value.findIndex((item, index) => !isVideo(item.file) && !inlineMarkerPresent(index))
      if (missingIndex >= 0) {
        Notify.warn(`图片 ${missingIndex + 1} 尚未插入正文`)
        return
      }
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
      Notify.success(`${isLongForm.value ? '长文' : '帖子'}已提交审核，审核通过后会公开显示`)
      await router.push('/')
    } else {
      Notify.success(isLongForm.value ? '长文已发布' : '帖子已发布')
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

watch([selectedCount, isMobile], () => { void initializeMediaSorting() })

onBeforeUnmount(() => {
  mediaPreparationController?.abort()
  mobileQuery?.removeEventListener('change', updateMobile)
  mediaSortable?.destroy()
  media.value.forEach(revokeMediaPreview)
})
</script>

<template>
  <PageContainer>
    <section v-if="isMobile" class="rf-mobile-composer" :class="{ 'rf-mobile-composer--long': isLongForm }">
      <header>
        <button type="button" class="rf-icon-button" aria-label="取消发布" :disabled="loading || preparingMedia" @click="router.back()"><AppIcon name="back" size="21" /></button>
        <strong>{{ composerTitle }}</strong>
        <button type="button" class="rf-mobile-publish" :disabled="loading || preparingMedia || !form.board_id || !form.content.trim() || (isLongForm && !form.title.trim())" @click="submit">{{ loading ? '发布中' : '发布' }}</button>
      </header>

      <div class="rf-mobile-composer-scroll">
        <nav class="rf-composer-type-tabs" aria-label="发布类型">
          <RouterLink to="/posts/new" :class="{ active: !isLongForm }"><AppIcon name="edit" size="16" />帖子</RouterLink>
          <RouterLink to="/posts/new/long" :class="{ active: isLongForm }"><AppIcon name="document" size="16" />长文</RouterLink>
        </nav>
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
          <UserAvatar :src="auth.user?.avatar_url" :name="auth.user?.display_name" :size="40" :frame="auth.user?.avatar_frame" />
          <div>
            <input v-model="form.title" maxlength="180" :placeholder="isLongForm ? '添加长文标题（必填）' : '添加标题（可选）'" :aria-label="isLongForm ? '长文标题，必填' : '帖子标题，可选'" />
            <div class="rf-markdown-mode" role="tablist" aria-label="正文模式">
              <button type="button" role="tab" data-editor-mode="edit" :tabindex="editorMode === 'edit' ? 0 : -1" :aria-selected="editorMode === 'edit'" :class="{ active: editorMode === 'edit' }" @click="editorMode = 'edit'" @keydown="handleEditorTabKeydown"><AppIcon name="edit" size="15" />编辑</button>
              <button type="button" role="tab" data-editor-mode="preview" :tabindex="editorMode === 'preview' ? 0 : -1" :aria-selected="editorMode === 'preview'" :class="{ active: editorMode === 'preview' }" @click="editorMode = 'preview'" @keydown="handleEditorTabKeydown"><AppIcon name="eye" size="15" />预览</button>
            </div>
            <textarea v-show="editorMode === 'edit'" ref="editorInput" v-model="form.content" rows="9" maxlength="50000" autofocus :placeholder="contentPlaceholder" aria-label="帖子正文，支持 Markdown" @click="rememberEditorSelection" @keyup="rememberEditorSelection" @select="rememberEditorSelection" />
            <div v-show="editorMode === 'preview'" class="rf-markdown-preview">
              <MarkdownContent v-if="form.content.trim()" :source="isLongForm ? inlinePreviewContent : form.content" :image-replacements="isLongForm ? inlinePreviewImages : undefined" />
              <span v-else>暂无内容</span>
            </div>
          </div>
        </div>

        <div v-if="tags.length" class="rf-tag-list" aria-label="已添加标签">
          <button v-for="(tag, index) in tags" :key="tag" type="button" :aria-label="`移除标签 ${tag}`" @click="removeTag(index)">#{{ tag }}<AppIcon name="close" size="13" /></button>
        </div>
        <div v-if="media.length" :key="mediaOrderVersion" ref="mediaGrid" class="rf-selected-media">
          <figure v-for="(item, index) in media" :key="item.url">
            <video v-if="isVideo(item.file)" :src="item.url" controls playsinline preload="metadata" />
            <img v-else :src="item.url" :alt="item.file.name || '待上传图片'" />
            <button type="button" class="rf-media-drag" aria-label="拖动调整媒体顺序" title="拖动调整媒体顺序" @keydown="handleMediaDragKeydown($event, index)"><AppIcon name="drag" size="17" /></button>
            <button v-if="isLongForm" type="button" class="rf-inline-media-insert" :disabled="inlineMarkerPresent(index)" @click="insertExistingMedia(index)">{{ inlineMarkerPresent(index) ? '已插入' : '插入正文' }}</button>
            <button type="button" class="rf-media-remove" aria-label="移除媒体" @click="removeMedia(index)"><AppIcon name="close" size="16" /></button>
          </figure>
        </div>
        <p v-if="isLongForm" class="rf-long-media-hint"><AppIcon name="image" size="16" />选择图片后会插入当前光标位置；剪切图片标记即可移动到其他段落。</p>
      </div>

      <footer class="rf-mobile-tools">
        <label :title="isLongForm ? '在光标处插入图片' : '添加图片或视频'" :aria-label="isLongForm ? '在光标处插入图片' : '添加图片或视频'" :aria-disabled="preparingMedia" @pointerdown="rememberEditorSelection"><AppIcon name="photo" size="21" /><input ref="mediaInput" type="file" :accept="isLongForm ? 'image/png,image/jpeg' : 'image/png,image/jpeg,video/mp4,video/webm'" multiple :disabled="preparingMedia" @change="chooseMedia" /></label>
        <label class="rf-mobile-tag-input"><AppIcon name="tag" size="19" /><input v-model="tagDraft" maxlength="25" placeholder="添加标签" @keydown="handleTagKeydown" /><button v-if="tagDraft" type="button" aria-label="确认添加标签" @click="addTag"><AppIcon name="add" size="17" /></button></label>
        <span aria-live="polite">{{ preparingMedia ? mediaPreparationLabel : `${selectedCount}/4` }}</span>
      </footer>
    </section>

    <section v-else class="rf-create-post" :class="{ 'rf-create-post--long': isLongForm }">
      <header class="rf-create-heading"><div><h1>{{ isLongForm ? '发布长文' : '发布帖子' }}</h1><p v-if="isLongForm">适合攻略、教程和完整经验分享，图片可以插入正文任意段落。</p></div></header>
      <nav class="rf-composer-type-tabs" aria-label="发布类型">
        <RouterLink to="/posts/new" :class="{ active: !isLongForm }"><AppIcon name="edit" size="16" />普通帖子</RouterLink>
        <RouterLink to="/posts/new/long" :class="{ active: isLongForm }"><AppIcon name="document" size="16" />长文</RouterLink>
      </nav>
      <QuotedPostContext @loaded="prefillQuote" />
      <nut-form ref="formRef" :model-value="form" :rules="rules" class="rf-create-form">
        <nut-form-item label="板块" required>
          <select v-model="form.board_id" class="rf-board-select" required aria-label="选择板块">
            <option :value="undefined" disabled>选择板块</option>
            <option v-for="board in boards" :key="board.id" :value="board.id">{{ board.name }}</option>
          </select>
        </nut-form-item>
        <nut-form-item :label="isLongForm ? '长文标题' : '标题'" prop="title" :required="isLongForm">
          <input v-model="form.title" class="rf-create-title-input" maxlength="180" :placeholder="isLongForm ? '请输入清晰的长文标题' : '标题（可选）'" />
        </nut-form-item>
        <nut-form-item label="正文" prop="content" class="rf-content-item">
          <div class="rf-markdown-editor">
            <div class="rf-markdown-mode" role="tablist" aria-label="正文模式">
              <button type="button" role="tab" data-editor-mode="edit" :tabindex="editorMode === 'edit' ? 0 : -1" :aria-selected="editorMode === 'edit'" :class="{ active: editorMode === 'edit' }" @click="editorMode = 'edit'" @keydown="handleEditorTabKeydown"><AppIcon name="edit" size="15" />编辑</button>
              <button type="button" role="tab" data-editor-mode="preview" :tabindex="editorMode === 'preview' ? 0 : -1" :aria-selected="editorMode === 'preview'" :class="{ active: editorMode === 'preview' }" @click="editorMode = 'preview'" @keydown="handleEditorTabKeydown"><AppIcon name="eye" size="15" />预览</button>
            </div>
            <div v-show="editorMode === 'edit'" class="rf-desktop-markdown-input">
              <textarea ref="editorInput" v-model="form.content" rows="9" maxlength="50000" :placeholder="contentPlaceholder" aria-label="帖子正文，支持 Markdown" @click="rememberEditorSelection" @keyup="rememberEditorSelection" @select="rememberEditorSelection" />
              <span>{{ form.content.length }}/50000</span>
            </div>
            <div v-show="editorMode === 'preview'" class="rf-markdown-preview">
              <MarkdownContent v-if="form.content.trim()" :source="isLongForm ? inlinePreviewContent : form.content" :image-replacements="isLongForm ? inlinePreviewImages : undefined" />
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
          <p v-if="isLongForm" class="rf-upload-help">先把光标放到目标段落，再选择图片。编辑器会在该位置插入图片标记，预览时显示真实图片。</p>
          <div v-if="media.length" :key="mediaOrderVersion" ref="mediaGrid" class="rf-selected-media">
            <figure v-for="(item, index) in media" :key="item.url">
              <video v-if="isVideo(item.file)" :src="item.url" controls playsinline preload="metadata" />
              <img v-else :src="item.url" :alt="item.file.name || '待上传图片'" />
              <button type="button" class="rf-media-drag" aria-label="拖动调整媒体顺序" title="拖动调整媒体顺序" @keydown="handleMediaDragKeydown($event, index)"><AppIcon name="drag" size="17" /></button>
              <button v-if="isLongForm" type="button" class="rf-inline-media-insert" :disabled="inlineMarkerPresent(index)" @click="insertExistingMedia(index)">{{ inlineMarkerPresent(index) ? '已插入' : '插入正文' }}</button>
              <button type="button" class="rf-media-remove" aria-label="移除媒体" @click="removeMedia(index)"><AppIcon name="close" size="16" /></button>
            </figure>
          </div>
          <label class="rf-media-picker" :class="{ disabled: preparingMedia }" @pointerdown="rememberEditorSelection"><AppIcon name="upload" size="19" /><span aria-live="polite">{{ preparingMedia ? mediaPreparationLabel : isLongForm ? '在光标处插入图片' : '选择图片或视频' }}</span><input ref="mediaInput" type="file" :accept="isLongForm ? 'image/png,image/jpeg' : 'image/png,image/jpeg,video/mp4,video/webm'" multiple :disabled="preparingMedia" @change="chooseMedia" /></label>
        </nut-form-item>
      </nut-form>
      <footer class="rf-create-actions"><nut-button plain type="default" :disabled="loading || preparingMedia" @click="router.back()">取消</nut-button><nut-button type="primary" :loading="loading" :disabled="preparingMedia" @click="submit">{{ isLongForm ? '发布长文' : '发布' }}</nut-button></footer>
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
.rf-markdown-preview { width: 100%; min-width: 0; max-width: 100%; min-height: 254px; padding: 12px; overflow: auto; overflow-wrap: anywhere; border: 1px solid var(--rf-line); border-radius: 10px; color: var(--rf-text); background: var(--rf-bg-subtle); line-height: 1.65; }
.rf-markdown-preview > span { color: var(--rf-muted); }
.rf-desktop-markdown-input { position: relative; overflow: hidden; padding: 12px 12px 32px; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg-subtle); }
.rf-desktop-markdown-input textarea { display: block; width: 100%; min-height: 220px; padding: 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font: inherit; line-height: 1.65; resize: vertical; }
.rf-desktop-markdown-input > span { position: absolute; right: 12px; bottom: 9px; color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-upload-item :deep(.nut-form-item__body),
.rf-upload-item :deep(.nut-form-item__body__slots) { width: 100%; }
.rf-upload-head { display: flex; align-items: center; justify-content: space-between; width: 100%; margin-bottom: 11px; }
.rf-upload-head span { color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.rf-upload-help { width: 100%; margin: -3px 0 11px; color: var(--rf-muted); font-size: 12px; line-height: 1.55; }
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
.rf-create-heading p { margin: 5px 0 0; color: var(--rf-muted); font-size: 13px; }
.rf-composer-type-tabs { display: inline-grid; grid-template-columns: repeat(2, minmax(110px, 1fr)); gap: 3px; margin: 14px 0 2px; padding: 3px; border: 1px solid var(--rf-line); border-radius: 8px; background: var(--rf-bg-subtle); }
.rf-composer-type-tabs a { display: inline-flex; min-height: 38px; align-items: center; justify-content: center; gap: 6px; padding: 0 14px; border-radius: 6px; color: var(--rf-muted); font-size: 13px; font-weight: 700; }
.rf-composer-type-tabs a.active { color: var(--primary); background: var(--rf-bg); box-shadow: 0 1px 3px color-mix(in srgb, var(--rf-text) 10%, transparent); }
.rf-create-post--long .rf-desktop-markdown-input textarea, .rf-create-post--long .rf-markdown-preview { min-height: 420px; }
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
.rf-selected-media figure { position: relative; aspect-ratio: 1 / 1; margin: 0; overflow: hidden; border: 1px solid var(--rf-line); border-radius: 8px; background: #000; }
.rf-selected-media img, .rf-selected-media video { display: block; width: 100%; height: 100%; object-fit: contain; }
.rf-media-drag { position: absolute; top: 6px; left: 6px; display: inline-grid; width: 30px; height: 30px; place-items: center; touch-action: none; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .74); cursor: grab; }
.rf-media-drag:active { cursor: grabbing; }
.rf-selected-media .rf-media-ghost { opacity: .32; }
.rf-selected-media .rf-media-chosen { border-color: var(--primary); box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 24%, transparent); }
.rf-selected-media .rf-media-dragging { opacity: .9; }
.rf-media-remove { position: absolute; top: 6px; right: 6px; display: inline-grid; width: 30px; height: 30px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .74); }
.rf-inline-media-insert { position: absolute; right: 7px; bottom: 7px; min-width: 72px; min-height: 30px; padding: 0 10px; border-radius: var(--rf-pill); color: #fff; background: rgba(15, 20, 25, .82); font-size: 11px; font-weight: 700; }
.rf-inline-media-insert:disabled { color: #fff; opacity: .72; }
.rf-media-picker { display: inline-flex; min-height: 42px; align-items: center; gap: 8px; padding: 0 14px; border: 1px solid var(--rf-line); border-radius: 8px; color: var(--primary); background: var(--rf-bg); cursor: pointer; font-weight: 650; }
.rf-media-picker:hover { background: var(--rf-bg-hover); }
.rf-media-picker.disabled { cursor: wait; opacity: .6; }
.rf-media-picker input, .rf-mobile-tools input[type="file"] { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }
.rf-mobile-composer { position: relative; display: grid; width: 100%; min-width: 0; height: calc(100dvh - var(--rf-mobile-bottom-nav)); max-height: calc(100dvh - var(--rf-mobile-bottom-nav)); grid-template-rows: auto minmax(0, 1fr) auto; overflow: hidden; background: var(--rf-bg); }
.rf-mobile-composer > header { position: relative; z-index: 5; display: grid; min-height: 54px; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 8px; padding: 4px 10px; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(12px); }
.rf-mobile-composer > header strong { justify-self: center; font-size: 17px; }
.rf-mobile-composer-scroll { min-width: 0; min-height: 0; overflow-x: hidden; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.rf-mobile-publish { min-width: 66px; min-height: 36px; padding: 0 14px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 13px; font-weight: 700; }
.rf-mobile-publish:disabled { cursor: not-allowed; opacity: .45; }
.rf-mobile-board { display: grid; min-height: 48px; grid-template-columns: 20px minmax(0, 1fr) 16px; align-items: center; gap: 8px; padding: 0 14px; border-bottom: 1px solid var(--rf-line); color: var(--primary); }
.rf-mobile-board select { width: 100%; height: 46px; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 15px; font-weight: 650; appearance: none; }
.rf-mobile-editor { display: grid; width: 100%; min-width: 0; grid-template-columns: 40px minmax(0, 1fr); align-items: start; gap: 10px; padding: 14px; }
.rf-mobile-editor > div { min-width: 0; }
.rf-mobile-editor input { width: 100%; height: 40px; padding: 0; border: 0; border-bottom: 1px solid var(--rf-line); outline: 0; color: var(--rf-text); background: transparent; font-size: 17px; font-weight: 650; }
.rf-mobile-editor .rf-markdown-mode { margin: 8px 0 0; }
.rf-mobile-editor textarea { display: block; width: 100%; min-height: 210px; padding: 12px 0; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 18px; line-height: 1.55; resize: none; }
.rf-mobile-editor .rf-markdown-preview { min-height: 210px; padding: 12px 0; border: 0; background: transparent; font-size: 17px; }
.rf-mobile-composer-scroll > .rf-tag-list, .rf-mobile-composer-scroll > .rf-selected-media { margin: 0; padding: 0 14px 10px 64px; }
.rf-mobile-composer-scroll > .rf-composer-type-tabs { display: grid; margin: 10px 14px 2px; }
.rf-mobile-composer--long .rf-mobile-editor textarea, .rf-mobile-composer--long .rf-mobile-editor .rf-markdown-preview { min-height: 340px; }
.rf-long-media-hint { display: flex; align-items: flex-start; gap: 6px; margin: 0; padding: 0 14px 12px 64px; color: var(--rf-muted); font-size: 12px; line-height: 1.5; }
.rf-long-media-hint svg { flex: 0 0 auto; margin-top: 1px; }
.rf-mobile-tools { position: relative; z-index: 4; display: grid; min-height: 54px; grid-template-columns: 44px minmax(0, 1fr) auto; align-items: center; gap: 6px; padding: 5px 12px calc(5px + env(safe-area-inset-bottom)); border-top: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 94%, transparent); backdrop-filter: blur(12px); }
.rf-mobile-tools > label:first-child { display: inline-grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; color: var(--primary); cursor: pointer; }
.rf-mobile-tools > label:first-child:hover { background: color-mix(in srgb, var(--primary) 10%, transparent); }
.rf-mobile-tag-input { display: grid; min-width: 0; height: 40px; grid-template-columns: 20px minmax(0, 1fr) 30px; align-items: center; gap: 5px; padding: 0 8px; border-radius: var(--rf-pill); color: var(--rf-muted); background: var(--rf-bg-subtle); }
.rf-mobile-tag-input input { min-width: 0; height: 38px; border: 0; outline: 0; color: var(--rf-text); background: transparent; font-size: 14px; }
.rf-mobile-tag-input button { display: inline-grid; width: 28px; height: 28px; place-items: center; border-radius: 50%; color: #fff; background: var(--primary); }
.rf-mobile-tools > span { min-width: 30px; color: var(--rf-muted); font-size: 12px; font-variant-numeric: tabular-nums; text-align: right; }
@media (max-width: 380px) {
  .rf-mobile-editor { padding-inline: 10px; }
  .rf-mobile-composer-scroll > .rf-tag-list, .rf-mobile-composer-scroll > .rf-selected-media { padding-left: 60px; padding-right: 10px; }
  .rf-long-media-hint { padding-right: 10px; padding-left: 60px; }
}
</style>
