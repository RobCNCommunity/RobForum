<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { createAdminMusicCategory, deleteAdminMusicCategory, errorMessage, fetchAdminMusicCategories, updateAdminMusicCategory, type MusicCategory } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const items = ref<MusicCategory[]>([])
const loading = ref(true)
const saving = ref(false)
const form = reactive({ id: 0, name: '', sort_order: 0, enabled: true })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminMusicCategories()
  } catch (error) {
    Notify.danger(errorMessage(error, '音乐分区加载失败'))
  } finally {
    loading.value = false
  }
}

function reset() {
  Object.assign(form, { id: 0, name: '', sort_order: items.value.length ? Math.max(...items.value.map((item) => item.sort_order)) + 10 : 0, enabled: true })
}

function edit(item: MusicCategory) {
  Object.assign(form, { id: item.id, name: item.name, sort_order: item.sort_order, enabled: item.enabled })
}

async function save() {
  if (!form.name.trim()) { Notify.warn('请输入分区名称'); return }
  saving.value = true
  const input = { name: form.name.trim(), sort_order: Number(form.sort_order) || 0, enabled: form.enabled }
  try {
    if (form.id) await updateAdminMusicCategory(form.id, input)
    else await createAdminMusicCategory(input)
    Notify.success(form.id ? '音乐分区已更新' : '音乐分区已创建')
    await load()
    reset()
  } catch (error) {
    Notify.danger(errorMessage(error, '音乐分区保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(item: MusicCategory) {
  if (!window.confirm(`确定删除音乐分区“${item.name}”吗？有关联音乐的分区请改为下架。`)) return
  try {
    await deleteAdminMusicCategory(item.id)
    Notify.success('音乐分区已删除')
    if (form.id === item.id) reset()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '音乐分区删除失败'))
  }
}

onMounted(async () => { await load(); reset() })
</script>

<template>
  <PageContainer title="音乐分区管理">
    <div class="admin-grid music-category-admin">
      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="category" size="18" />{{ form.id ? '编辑分区' : '新增分区' }}</h3><p>排序值越小，前台展示越靠前。</p></div><span class="rf-kicker">{{ form.id ? `#${form.id}` : 'NEW' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="save">
          <label class="rf-field-label"><span>分区名称</span><input v-model="form.name" class="rf-control" maxlength="40" placeholder="例如：流行、纯音乐、Remix" required /></label>
          <label class="rf-field-label"><span>排序值</span><input v-model.number="form.sort_order" class="rf-control" type="number" min="-9999" max="9999" step="1" required /></label>
          <label class="rf-switch-row"><input v-model="form.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>前台启用</span></label>
          <div class="rf-form-actions"><button type="submit" class="rf-primary-button" :disabled="saving">{{ saving ? '保存中' : '保存分区' }}</button><button type="button" class="rf-secondary-button" @click="reset">{{ form.id ? '取消编辑' : '清空' }}</button></div>
        </form>
      </section>

      <section class="pro-card">
        <header class="rf-panel-heading"><div><h3>分区列表</h3><p>下架分区不会删除已关联的音乐数据。</p></div><span class="rf-count">{{ items.length }}</span></header>
        <div v-if="loading" class="rf-list-loading"><span v-for="n in 4" :key="n" /></div>
        <div v-else-if="!items.length" class="rf-empty"><AppIcon name="category" size="28" /><strong>暂无音乐分区</strong></div>
        <div v-else class="rf-admin-list">
          <article v-for="item in items" :key="item.id" class="rf-admin-list-row">
            <div class="category-order">{{ item.sort_order }}</div>
            <div class="rf-admin-list-copy"><div class="rf-list-title"><strong>{{ item.name }}</strong><span class="rf-status-chip" :class="item.enabled ? 'is-on' : ''">{{ item.enabled ? '已启用' : '已下架' }}</span></div><small>更新于 {{ new Date(item.updated_at).toLocaleString('zh-CN') }}</small></div>
            <div class="rf-row-actions"><button type="button" class="rf-secondary-button rf-button-small" @click="edit(item)">编辑</button><button type="button" class="rf-danger-button rf-button-small" @click="remove(item)">删除</button></div>
          </article>
        </div>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.category-order { display: grid; width: 38px; height: 32px; flex: 0 0 38px; place-items: center; border: 1px solid var(--rf-line); border-radius: 5px; color: var(--rf-muted); background: var(--rf-bg-subtle); font-size: 11px; font-variant-numeric: tabular-nums; }
@media (max-width: 650px) { .music-category-admin .rf-admin-list-row { align-items: flex-start; flex-wrap: wrap; }.music-category-admin .rf-row-actions { width: 100%; padding-left: 50px; } }
</style>
