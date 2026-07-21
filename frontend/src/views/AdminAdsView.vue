<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { createAdminAd, deleteAdminAd, errorMessage, fetchAdminAds, updateAdminAd, type AdSlot } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const loading = ref(true)
const saving = ref(false)
const items = ref<AdSlot[]>([])
const form = reactive({ id: 0, title: '', image_url: '', link_url: '', sort_order: 0, enabled: true })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminAds()
  } catch (error) {
    Notify.danger(errorMessage(error, '广告加载失败'))
  } finally {
    loading.value = false
  }
}

function reset() {
  Object.assign(form, { id: 0, title: '', image_url: '', link_url: '', sort_order: items.value.length, enabled: true })
}

function edit(item: AdSlot) {
  Object.assign(form, { id: item.id, title: item.title, image_url: item.image_url, link_url: item.link_url, sort_order: item.sort_order, enabled: item.enabled })
}

async function save() {
  if (!form.image_url.trim() && !form.title.trim()) {
    Notify.warn('至少填写广告标题或图片地址')
    return
  }
  saving.value = true
  try {
    const payload = { title: form.title.trim(), image_url: form.image_url.trim(), link_url: form.link_url.trim(), sort_order: form.sort_order, enabled: form.enabled }
    if (form.id) await updateAdminAd(form.id, payload)
    else await createAdminAd(payload)
    Notify.success('广告已保存')
    reset()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '广告保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(id: number) {
  if (!window.confirm('确定删除这条广告吗？')) return
  try {
    await deleteAdminAd(id)
    Notify.success('广告已删除')
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '广告删除失败'))
  }
}

onMounted(load)
</script>

<template>
  <PageContainer title="广告管理">
    <div class="admin-grid">
      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="image" size="18" />{{ form.id ? '编辑广告' : '新增广告' }}</h3><p>建议使用清晰的横向图片，链接应使用 HTTPS。</p></div><span class="rf-kicker">{{ form.id ? `#${form.id}` : 'NEW' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="save">
          <label class="rf-field-label"><span>标题</span><input v-model="form.title" class="rf-control" maxlength="120" /></label>
          <label class="rf-field-label"><span>图片 URL</span><input v-model="form.image_url" class="rf-control" type="url" placeholder="https://..." /></label>
          <div v-if="form.image_url" class="rf-ad-preview"><img :src="form.image_url" alt="广告预览" /></div>
          <label class="rf-field-label"><span>跳转链接</span><input v-model="form.link_url" class="rf-control" type="url" placeholder="https://..." /></label>
          <div class="rf-form-grid rf-form-grid--two">
            <label class="rf-field-label"><span>排序</span><input v-model.number="form.sort_order" class="rf-control" type="number" min="0" step="1" /></label>
            <label class="rf-switch-row rf-switch-row--field"><input v-model="form.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>启用展示</span></label>
          </div>
          <div class="rf-form-actions"><nut-button type="primary" :loading="saving" @click="save">保存广告</nut-button><button type="button" class="rf-secondary-button" @click="reset">清空</button></div>
        </form>
      </section>

      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3>广告列表</h3><p>拖拽排序暂未开放，使用排序数字控制轮播顺序。</p></div><span class="rf-count">{{ items.length }}</span></header>
        <div v-if="loading" class="rf-list-loading"><span v-for="n in 3" :key="n" /></div>
        <div v-else-if="!items.length" class="rf-empty"><AppIcon name="image" size="28" /><strong>暂无广告</strong><span>添加一条横幅广告后会显示在首页。</span></div>
        <div v-else class="rf-admin-list">
          <article v-for="item in items" :key="item.id" class="rf-admin-list-row">
            <img v-if="item.image_url" :src="item.image_url" :alt="item.title || '广告'" />
            <div class="rf-admin-list-copy"><strong>{{ item.title || '无标题' }}</strong><small>#{{ item.id }} · 排序 {{ item.sort_order }} · {{ item.enabled ? '启用' : '停用' }}</small></div>
            <div class="rf-row-actions"><button type="button" class="rf-secondary-button rf-button-small" @click="edit(item)">编辑</button><button type="button" class="rf-danger-button rf-button-small" @click="remove(item.id)">删除</button></div>
          </article>
        </div>
      </section>
    </div>
  </PageContainer>
</template>
