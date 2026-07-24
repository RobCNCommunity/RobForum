<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { createPost, errorMessage, fetchBoards, type Board } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import { postTypes } from '@/postTypes'

interface UploadItem {
  name?: string
  type?: string
  url?: string
  formData?: FormData
}

const router = useRouter()
const formRef = ref<{ validate: () => Promise<unknown> } | null>(null)
const boards = ref<Board[]>([])
const fileList = ref<UploadItem[]>([])
const loading = ref(false)
const form = reactive({
  board_id: undefined as number | undefined,
  title: '',
  post_type: 'discussion',
  content: '',
})

const selectedCount = computed(() => fileList.value.length)
const rules = {
  title: [
    { required: true, message: '请输入标题' },
    { max: 180, message: '标题不能超过 180 个字符' },
  ],
  content: [
    { required: true, message: '请输入正文' },
    { max: 50000, message: '正文不能超过 50000 个字符' },
  ],
}

onMounted(async () => {
  try {
    boards.value = await fetchBoards()
  } catch (error) {
    Notify.danger(errorMessage(error, '板块加载失败'))
  }
})

async function beforeUpload(files: FileList | File[]) {
  const accepted = Array.from(files).filter((file) => {
    const extensionOK = /\.(png|jpe?g)$/i.test(file.name)
    const typeOK = file.type === 'image/png' || file.type === 'image/jpeg'
    return extensionOK && typeOK
  })
  if (accepted.length !== files.length) Notify.warn('仅支持 PNG 和 JPG 图片')
  return accepted
}

function selectedFiles() {
  return fileList.value
    .map((item) => item.formData?.get('files'))
    .filter((item): item is File => item instanceof File)
}

async function submit() {
  if (!form.board_id) {
    Notify.warn('请选择板块')
    return
  }
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    const post = await createPost({
      board_id: form.board_id,
      title: form.title.trim(),
      post_type: form.post_type,
      content: form.content.trim(),
      files: selectedFiles(),
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
</script>

<template>
  <PageContainer title="发布帖子">
    <section class="rf-create-post">
      <nut-form ref="formRef" :model-value="form" :rules="rules" class="rf-create-form">
        <nut-form-item label="板块" required>
          <select v-model="form.board_id" class="rf-board-select" required aria-label="选择板块">
            <option :value="undefined" disabled>选择板块</option>
            <option v-for="board in boards" :key="board.id" :value="board.id">{{ board.name }}</option>
          </select>
        </nut-form-item>

        <nut-form-item label="类型" required>
          <div class="rf-post-type-options" role="radiogroup" aria-label="帖子类型">
            <button
              v-for="item in postTypes"
              :key="item.value"
              type="button"
              role="radio"
              :aria-checked="form.post_type === item.value"
              :class="{ selected: form.post_type === item.value }"
              @click="form.post_type = item.value"
            >
              <span><AppIcon :name="item.icon" size="18" /></span>
              <div><strong>{{ item.label }}</strong><small>{{ item.description }}</small></div>
            </button>
          </div>
        </nut-form-item>

        <nut-form-item label="标题" prop="title">
          <input v-model="form.title" class="rf-create-title-input" maxlength="180" placeholder="填写标题" />
        </nut-form-item>

        <nut-form-item label="正文" prop="content" class="rf-content-item">
          <nut-textarea
            v-model="form.content"
            :rows="12"
            :max-length="50000"
            limit-show
            placeholder="分享你的想法"
          />
        </nut-form-item>

        <nut-form-item class="rf-upload-item">
          <div class="rf-upload-head">
            <strong>图片</strong>
            <span>{{ selectedCount }}/4</span>
          </div>
          <nut-uploader
            v-model:file-list="fileList"
            name="files"
            accept="image/png,image/jpeg"
            multiple
            :maximum="4"
            :maximize="5 * 1024 * 1024"
            :auto-upload="false"
            :before-upload="beforeUpload"
            @oversize="Notify.warn('单张图片不能超过 5 MB')"
          />
        </nut-form-item>
      </nut-form>

      <footer class="rf-create-actions">
        <nut-button plain type="default" :disabled="loading" @click="router.back()">取消</nut-button>
        <nut-button type="primary" :loading="loading" @click="submit">发布</nut-button>
      </footer>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-create-post { min-height: 100%; padding: 8px 20px 28px; color: var(--rf-text); background: var(--rf-bg); }
.rf-create-form { color: var(--rf-text); background: transparent; }
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
.rf-post-type-options { display: grid; width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.rf-post-type-options > button { display: grid; min-width: 0; min-height: 58px; grid-template-columns: 32px minmax(0, 1fr); align-items: center; gap: 9px; padding: 8px 10px; border: 1px solid var(--rf-line); border-radius: 8px; color: var(--rf-text); background: var(--rf-bg); text-align: left; transition: border-color .16s ease, background-color .16s ease, color .16s ease; }
.rf-post-type-options > button:hover { border-color: color-mix(in srgb, var(--primary) 45%, var(--rf-line)); background: var(--rf-bg-hover); }
.rf-post-type-options > button.selected { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 7%, var(--rf-bg)); box-shadow: inset 0 0 0 1px var(--primary); }
.rf-post-type-options > button > span { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 50%; background: var(--rf-bg-subtle); }
.rf-post-type-options > button.selected > span { background: color-mix(in srgb, var(--primary) 13%, var(--rf-bg)); }
.rf-post-type-options > button > div { display: flex; min-width: 0; flex-direction: column; gap: 2px; }
.rf-post-type-options strong { font-size: 14px; }
.rf-post-type-options small { overflow: hidden; color: var(--rf-muted); font-size: 11px; font-weight: 400; text-overflow: ellipsis; white-space: nowrap; }
.rf-content-item :deep(.nut-form-item__body) { align-items: flex-start; }
.rf-content-item :deep(.nut-textarea) { overflow: hidden; padding: 12px 12px 32px !important; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg-subtle) !important; }
.rf-content-item :deep(.nut-textarea__textarea) { min-height: 220px; color: var(--rf-text) !important; background: transparent !important; line-height: 1.65; resize: vertical; }
.rf-content-item :deep(.nut-textarea__limit) { right: 12px; bottom: 9px; color: var(--rf-muted); background: transparent; font-size: 12px; font-variant-numeric: tabular-nums; }
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
  .rf-post-type-options { width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .rf-post-type-options > button { min-height: 50px; grid-template-columns: 28px minmax(0, 1fr); padding: 7px 8px; }
  .rf-post-type-options > button > span { width: 28px; height: 28px; }
  .rf-post-type-options small { display: none; }
  .rf-upload-item :deep(.nut-uploader) { width: 100%; }
  .rf-upload-item :deep(.nut-uploader__preview-list) { display: grid; width: 100%; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .rf-content-item :deep(.nut-textarea__textarea) { min-height: 180px; }
  .rf-create-actions { position: static; padding: 18px 0 8px; background: transparent; }
  .rf-create-actions .nut-button { flex: 1; }
}
</style>
