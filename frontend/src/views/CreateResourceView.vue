<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { createResource, errorMessage } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const router = useRouter()
const form = reactive({ title: '', description: '', game: '', version: '', resource_type: 'map', price_yuan: 0 })
const selectedFile = ref<File | null>(null)
const previewFiles = ref<File[]>([])
const previewURLs = ref<string[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const previewInput = ref<HTMLInputElement | null>(null)
const submitting = ref(false)

function chooseFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] || null
}

function choosePreviews(event: Event) {
  const files = Array.from((event.target as HTMLInputElement).files || [])
  if (files.length > 5) {
    Notify.warn('最多选择 5 张预览图')
  }
  const accepted = files.slice(0, 5).filter((file) => file.size <= 5 * 1024 * 1024 && ['image/png', 'image/jpeg'].includes(file.type))
  if (accepted.length !== files.slice(0, 5).length) Notify.warn('预览图仅支持 PNG/JPG，且单张不超过 5 MB')
  previewURLs.value.forEach((url) => URL.revokeObjectURL(url))
  previewFiles.value = accepted
  previewURLs.value = accepted.map((file) => URL.createObjectURL(file))
}

function removePreview(index: number) {
  URL.revokeObjectURL(previewURLs.value[index])
  previewFiles.value.splice(index, 1)
  previewURLs.value.splice(index, 1)
  if (previewInput.value) previewInput.value.value = ''
}

function reset() {
  Object.assign(form, { title: '', description: '', game: '', version: '', resource_type: 'map', price_yuan: 0 })
  selectedFile.value = null
  previewURLs.value.forEach((url) => URL.revokeObjectURL(url))
  previewFiles.value = []
  previewURLs.value = []
  if (fileInput.value) fileInput.value.value = ''
  if (previewInput.value) previewInput.value.value = ''
}

async function submit() {
  if (!selectedFile.value) {
    Notify.warn('请选择要投稿的资源文件')
    return
  }
  if (!form.title.trim() || !form.description.trim() || !form.game.trim()) {
    Notify.warn('请填写标题、介绍和适用游戏')
    return
  }
  submitting.value = true
  try {
    await createResource({
      title: form.title.trim(),
      description: form.description.trim(),
      game: form.game.trim(),
      version: form.version.trim(),
      resource_type: form.resource_type,
      price_cents: Math.max(0, Math.round(Number(form.price_yuan || 0) * 100)),
      file: selectedFile.value,
      preview_files: previewFiles.value,
    })
    Notify.success('资源已提交，等待管理员审核')
    reset()
    router.push('/resources')
  } catch (error) {
    Notify.danger(errorMessage(error, '资源投稿失败'))
  } finally {
    submitting.value = false
  }
}

onBeforeUnmount(() => previewURLs.value.forEach((url) => URL.revokeObjectURL(url)))
</script>

<template>
  <PageContainer>
    <div class="rf-resource-editor-page">
      <header class="rf-resource-editor-heading">
        <button type="button" class="rf-icon-text-button" @click="router.back()"><AppIcon name="back" size="18" />返回资源中心</button>
        <p class="rf-kicker">Creator studio</p>
        <h1>发布资源</h1>
        <p>把作品介绍清楚，添加几张预览图，提交后由管理员人工审核。</p>
      </header>

      <form class="rf-panel rf-editor-form rf-resource-create-form" @submit.prevent="submit">
        <section>
          <h2>基本信息</h2>
          <div class="rf-form-grid rf-form-grid--two">
            <label>资源标题<input v-model="form.title" maxlength="180" placeholder="例如：新手地图模板" required /></label>
            <label>适用游戏<input v-model="form.game" maxlength="120" placeholder="例如：Brookhaven" required /></label>
            <label>版本<input v-model="form.version" maxlength="80" placeholder="例如：1.0.0" /></label>
            <label>资源类型<select v-model="form.resource_type"><option value="map">地图</option><option value="script">脚本</option><option value="asset">素材</option><option value="guide">教程</option><option value="other">其他</option></select></label>
          </div>
          <label>详细介绍<textarea v-model="form.description" rows="8" maxlength="20000" placeholder="介绍功能、使用方法、适用版本和注意事项" required /></label>
        </section>

        <section>
          <h2>文件与预览</h2>
          <div class="rf-upload-dropzone">
            <AppIcon name="upload" size="24" />
            <strong>{{ selectedFile ? selectedFile.name : '选择资源文件' }}</strong>
            <small>支持 ZIP、PDF、文本、Roblox 文件等，最大 100 MB</small>
            <input ref="fileInput" type="file" @change="chooseFile" />
          </div>
          <label>背景/预览图 <small>最多 5 张，PNG/JPG，单张不超过 5 MB</small><input ref="previewInput" type="file" accept="image/png,image/jpeg" multiple @change="choosePreviews" /></label>
          <div v-if="previewURLs.length" class="rf-resource-preview-picker"><figure v-for="(url, index) in previewURLs" :key="url"><img :src="url" alt="预览图" /><button type="button" aria-label="移除预览图" @click="removePreview(index)">×</button></figure></div>
        </section>

        <section>
          <h2>发布设置</h2>
          <label>售价（元）<input v-model.number="form.price_yuan" type="number" min="0" max="10000" step="0.01" /><small>填写 0 即为免费资源。付费资源审核通过后才会公开。</small></label>
        </section>

        <footer class="rf-form-actions"><button type="button" class="rf-secondary-button" @click="router.back()">取消</button><button type="submit" class="rf-primary-button" :disabled="submitting">{{ submitting ? '提交中…' : '提交人工审核' }}</button></footer>
      </form>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-resource-editor-page { max-width: 760px; margin: 0 auto; }
.rf-resource-editor-heading { padding: 8px 0 24px; }
.rf-resource-editor-heading .rf-icon-text-button { margin-bottom: 24px; padding-left: 0; }
.rf-resource-editor-heading h1 { margin: 4px 0 6px; font-size: 28px; letter-spacing: -.02em; }
.rf-resource-editor-heading p:last-child { margin: 0; color: var(--rf-muted); }
.rf-resource-create-form { display: flex; flex-direction: column; gap: 28px; padding: 24px; }
.rf-resource-create-form section { display: flex; flex-direction: column; gap: 16px; }
.rf-resource-create-form section + section { padding-top: 24px; border-top: 1px solid var(--rf-line); }
.rf-resource-create-form h2 { margin: 0; font-size: 17px; }
.rf-resource-create-form label { display: flex; flex-direction: column; gap: 7px; color: var(--rf-text); font-size: 13px; font-weight: 650; }
.rf-resource-create-form label small { color: var(--rf-muted); font-weight: 400; }
.rf-resource-create-form input:not([type='file']), .rf-resource-create-form select, .rf-resource-create-form textarea { width: 100%; min-height: 44px; padding: 10px 12px; border: 1px solid var(--rf-line); border-radius: 6px; outline: 0; color: var(--rf-text); background: var(--rf-bg); }
.rf-resource-create-form textarea { resize: vertical; line-height: 1.55; }
.rf-resource-create-form input:focus, .rf-resource-create-form select:focus, .rf-resource-create-form textarea:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 16%, transparent); }
.rf-upload-dropzone { position: relative; display: flex; min-height: 150px; flex-direction: column; align-items: center; justify-content: center; gap: 6px; padding: 24px; border: 1px dashed var(--rf-faint); border-radius: 12px; color: var(--rf-muted); background: var(--rf-bg-subtle); text-align: center; }
.rf-upload-dropzone strong { color: var(--rf-text); }
.rf-upload-dropzone small { font-size: 12px; }
.rf-upload-dropzone input { position: absolute; inset: 0; width: 100%; height: 100%; cursor: pointer; opacity: 0; }
.rf-resource-preview-picker { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.rf-resource-preview-picker figure { position: relative; aspect-ratio: 1 / 1; margin: 0; overflow: hidden; border-radius: 8px; background: var(--rf-bg-subtle); }
.rf-resource-preview-picker img { width: 100%; height: 100%; object-fit: cover; }
.rf-resource-preview-picker button { position: absolute; top: 5px; right: 5px; display: grid; width: 25px; height: 25px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15,20,25,.72); font-size: 18px; line-height: 1; }
@media (max-width: 560px) { .rf-resource-create-form { padding: 16px 12px; } .rf-form-grid--two { grid-template-columns: 1fr; } .rf-resource-preview-picker { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
