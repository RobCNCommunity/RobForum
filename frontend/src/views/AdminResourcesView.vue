<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchAdminResources, reviewResource, type Resource } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const items = ref<Resource[]>([])
const loading = ref(true)
const actionID = ref<number | null>(null)

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminResources('pending')
  } catch (error) {
    Notify.danger(errorMessage(error, '审核列表加载失败'))
  } finally {
    loading.value = false
  }
}

async function applyReview(item: Resource, status: string, reason = '') {
  actionID.value = item.id
  try {
    await reviewResource(item.id, status, reason)
    Notify.success('资源状态已更新')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '资源审核失败'))
  } finally {
    actionID.value = null
  }
}

function reject(item: Resource) {
  if (!window.confirm('拒绝这份资源？资源会保留在投稿记录中，并向投稿者显示未通过原因。')) return
  applyReview(item, 'rejected', '未通过社区资源安全审核')
}

function takedown(item: Resource) {
  if (!window.confirm('下架这份资源？下架后公共下载地址会立即失效。')) return
  applyReview(item, 'takedown', '管理员下架资源')
}

function fileSize(value?: number) {
  if (!value) return '未知大小'
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

onMounted(load)
</script>

<template>
  <PageContainer title="资源审核">
    <div class="rf-inline-note rf-inline-note--info"><AppIcon name="notice" size="18" /><div><strong>审核前检查文件名、类型、体积和投稿说明</strong><span>通过后资源会进入公共列表；拒绝或下架都会保留审核原因。</span></div></div>
    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 4" :key="n" /></div>
    <div v-else-if="!items.length" class="rf-empty"><AppIcon name="shop" size="28" /><strong>暂无待审核资源</strong><span>新的资源投稿会出现在这里。</span></div>
    <section v-else class="rf-review-list">
      <article v-for="item in items" :key="item.id" class="rf-review-row">
        <header><div><span class="rf-kicker">#{{ item.id }}</span><strong>{{ item.title }}</strong></div><time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time></header>
        <p>{{ item.description || '投稿者未填写资源说明。' }}</p>
        <dl><div><dt>投稿者</dt><dd>{{ item.creator_name }}</dd></div><div><dt>游戏与版本</dt><dd>{{ item.game }} {{ item.version }}</dd></div><div><dt>资源类型</dt><dd>{{ item.resource_type }}</dd></div><div><dt>文件</dt><dd>{{ item.file?.original_name || '未提供' }} · {{ fileSize(item.file?.size_bytes) }}</dd></div></dl>
        <footer><div class="rf-file-hash" v-if="item.file?.sha256"><span>SHA-256</span><code>{{ item.file.sha256 }}</code></div><div class="rf-row-actions"><nut-button size="small" type="primary" :loading="actionID === item.id" @click="applyReview(item, 'approved')">通过</nut-button><button type="button" class="rf-danger-button rf-button-small" :disabled="actionID === item.id" @click="reject(item)">拒绝</button><button type="button" class="rf-secondary-button rf-button-small" :disabled="actionID === item.id" @click="takedown(item)">下架</button></div></footer>
      </article>
    </section>
  </PageContainer>
</template>
