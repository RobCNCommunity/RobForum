<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { createAdminNotice, deleteAdminNotice, errorMessage, fetchAdminNotices, updateAdminNotice, type Notice } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const loading = ref(true)
const saving = ref(false)
const items = ref<Notice[]>([])
const form = reactive({ id: 0, title: '', content: '', level: 'info', pinned: false, enabled: true })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminNotices()
  } catch (error) {
    Notify.danger(errorMessage(error, '公告加载失败'))
  } finally {
    loading.value = false
  }
}

function reset() { Object.assign(form, { id: 0, title: '', content: '', level: 'info', pinned: false, enabled: true }) }
function edit(item: Notice) { Object.assign(form, { id: item.id, title: item.title, content: item.content, level: item.level || 'info', pinned: item.pinned, enabled: item.enabled }) }

async function save() {
  if (!form.title.trim() || !form.content.trim()) {
    Notify.warn('标题和内容不能为空')
    return
  }
  saving.value = true
  try {
    const payload = { title: form.title.trim(), content: form.content.trim(), level: form.level, pinned: form.pinned, enabled: form.enabled }
    if (form.id) await updateAdminNotice(form.id, payload)
    else await createAdminNotice(payload)
    Notify.success('公告已保存')
    reset()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '公告保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(id: number) {
  if (!window.confirm('确定删除这条公告吗？')) return
  try {
    await deleteAdminNotice(id)
    Notify.success('公告已删除')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '公告删除失败'))
  }
}

function levelLabel(level: string) { return ({ info: '普通', warning: '提醒', important: '重要' } as Record<string, string>)[level] || level }
onMounted(load)
</script>

<template>
  <PageContainer title="公告管理">
    <div class="admin-grid">
      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="notice" size="18" />{{ form.id ? '编辑公告' : '发布公告' }}</h3><p>只把需要用户注意的内容放进公告，避免干扰信息流。</p></div><span class="rf-kicker">{{ form.id ? `#${form.id}` : 'NEW' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="save">
          <label class="rf-field-label"><span>标题</span><input v-model="form.title" class="rf-control" maxlength="120" required /></label>
          <label class="rf-field-label"><span>内容</span><textarea v-model="form.content" class="rf-control rf-textarea" rows="6" maxlength="1000" required /></label>
          <label class="rf-field-label"><span>级别</span><select v-model="form.level" class="rf-control"><option value="info">普通</option><option value="warning">提醒</option><option value="important">重要</option></select></label>
          <div class="rf-choice-row"><label class="rf-switch-row"><input v-model="form.pinned" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>置顶公告</span></label><label class="rf-switch-row"><input v-model="form.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>启用展示</span></label></div>
          <div class="rf-form-actions"><nut-button type="primary" :loading="saving" @click="save">保存公告</nut-button><button type="button" class="rf-secondary-button" @click="reset">清空</button></div>
        </form>
      </section>

      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3>公告列表</h3><p>已删除的公告不会再出现在首页铃铛中。</p></div><span class="rf-count">{{ items.length }}</span></header>
        <div v-if="loading" class="rf-list-loading"><span v-for="n in 3" :key="n" /></div>
        <div v-else-if="!items.length" class="rf-empty"><AppIcon name="notice" size="28" /><strong>暂无公告</strong><span>发布一条公告后会显示在这里。</span></div>
        <div v-else class="rf-admin-list">
          <article v-for="item in items" :key="item.id" class="rf-admin-list-row rf-admin-list-row--stacked">
            <div class="rf-admin-list-copy"><div class="rf-list-title"><strong>{{ item.title }}</strong><span class="rf-status-chip" :class="item.enabled ? 'is-on' : ''">{{ item.enabled ? '启用' : '停用' }}</span><span v-if="item.pinned" class="rf-status-chip is-warning">置顶</span></div><small>{{ levelLabel(item.level) }} · {{ new Date(item.updated_at || item.created_at).toLocaleString('zh-CN') }}</small><p>{{ item.content }}</p></div>
            <div class="rf-row-actions"><button type="button" class="rf-secondary-button rf-button-small" @click="edit(item)">编辑</button><button type="button" class="rf-danger-button rf-button-small" @click="remove(item.id)">删除</button></div>
          </article>
        </div>
      </section>
    </div>
  </PageContainer>
</template>
