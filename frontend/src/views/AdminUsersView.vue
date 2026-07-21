<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  deleteAdminUser,
  errorMessage,
  fetchAdminUsers,
  updateAdminUserStatus,
  type AdminUser,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'

type UserAction = 'banned' | 'active' | 'deleted'

const auth = useAuthStore()
const router = useRouter()
const items = ref<AdminUser[]>([])
const loading = ref(true)
const query = ref('')
const status = ref('')
const action = reactive<{ item: AdminUser | null; type: UserAction; reason: string; confirmation: string }>({
  item: null,
  type: 'banned',
  reason: '',
  confirmation: '',
})
const submitting = ref(false)
const dialogOpen = computed(() => action.item !== null)
const actionTitle = computed(() => ({ banned: '封禁用户', active: '解除封禁', deleted: '删除用户' })[action.type])
const actionButton = computed(() => ({ banned: '确认封禁', active: '确认解封', deleted: '永久删除账号' })[action.type])
const actionReady = computed(() => action.reason.trim().length >= 2 && (action.type !== 'deleted' || action.confirmation === '删除'))

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminUsers({ q: query.value.trim() || undefined, status: status.value || undefined })
  } catch (error) {
    Notify.danger(errorMessage(error, '用户列表加载失败'))
  } finally {
    loading.value = false
  }
}

function openAction(item: AdminUser, type: UserAction) {
  action.item = item
  action.type = type
  action.reason = ''
  action.confirmation = ''
}

function closeAction() {
  if (submitting.value) return
  resetAction()
}

function resetAction() {
  action.item = null
  action.reason = ''
  action.confirmation = ''
}

async function submitAction() {
  if (!action.item || !actionReady.value) {
    Notify.warn(action.type === 'deleted' ? '请填写处理原因并输入“删除”确认' : '请填写至少 2 个字符的处理原因')
    return
  }
  submitting.value = true
  try {
    if (action.type === 'deleted') {
      await deleteAdminUser(action.item.id, action.reason.trim())
      Notify.success('账号已匿名化删除，相关公开内容已下线')
    } else {
      await updateAdminUserStatus(action.item.id, action.type, action.reason.trim())
      Notify.success(action.type === 'banned' ? '用户已封禁并注销全部会话' : '用户已解除封禁')
    }
    resetAction()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, `${actionTitle.value}失败`))
  } finally {
    submitting.value = false
  }
}

function statusLabel(value: string) {
  return ({ active: '正常', banned: '已封禁', deleted: '已删除' } as Record<string, string>)[value] || value
}

function protectedAccount(item: AdminUser) {
  return item.role === 'admin' || item.id === auth.user?.id
}

onMounted(load)
</script>

<template>
  <PageContainer title="用户管理">
    <form class="rf-admin-filter" @submit.prevent="load">
      <label class="rf-admin-search"><AppIcon name="search" size="18" /><input v-model="query" maxlength="100" placeholder="搜索昵称、邮箱或用户 ID" /><button type="submit">搜索</button></label>
      <select v-model="status" class="rf-control rf-control--compact" aria-label="筛选账号状态" @change="load">
        <option value="">全部状态</option><option value="active">正常</option><option value="banned">已封禁</option><option value="deleted">已删除</option>
      </select>
      <button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button>
    </form>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
    <div v-else-if="!items.length" class="rf-empty"><AppIcon name="people" size="28" /><strong>没有匹配的用户</strong></div>
    <section v-else class="rf-admin-user-list">
      <article v-for="item in items" :key="item.id" class="rf-admin-user-row">
        <UserAvatar :src="item.avatar_url" :name="item.display_name" :size="44" />
        <div class="rf-admin-user-main">
          <div class="rf-admin-user-name"><strong>{{ item.display_name }}</strong><VerifiedBadge :verified="item.blue_verified" :label="item.verification_label" /><span v-if="item.role === 'admin'" class="rf-status-chip is-on">管理员</span></div>
          <span>{{ item.email }}</span><small>#{{ item.id }} · 注册于 {{ new Date(item.created_at).toLocaleString('zh-CN') }}</small>
        </div>
        <div class="rf-admin-user-counts"><span><strong>{{ item.post_count }}</strong>帖子</span><span><strong>{{ item.comment_count }}</strong>评论</span><span><strong>{{ item.resource_count }}</strong>资源</span></div>
        <span class="rf-status-chip" :class="item.status === 'active' ? 'is-on' : item.status === 'banned' || item.status === 'deleted' ? 'is-danger' : ''">{{ statusLabel(item.status) }}</span>
        <div class="rf-row-actions">
          <button v-if="item.status !== 'deleted'" type="button" class="rf-secondary-button rf-button-small" @click="router.push(`/users/${item.id}`)">主页</button>
          <template v-if="!protectedAccount(item) && item.status !== 'deleted'">
            <button v-if="item.status === 'active'" type="button" class="rf-danger-button rf-button-small" @click="openAction(item, 'banned')">封禁</button>
            <button v-else-if="item.status === 'banned'" type="button" class="rf-secondary-button rf-button-small" @click="openAction(item, 'active')">解封</button>
            <button type="button" class="rf-danger-button rf-button-small" @click="openAction(item, 'deleted')">删除</button>
          </template>
          <small v-else-if="protectedAccount(item)">受保护账号</small>
        </div>
      </article>
    </section>

    <div v-if="dialogOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeAction">
      <section class="rf-dialog" role="dialog" aria-modal="true" :aria-label="actionTitle">
        <header><div><h2>{{ actionTitle }}</h2><p>{{ action.item?.display_name }} · {{ action.item?.email }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="closeAction"><AppIcon name="close" size="18" /></button></header>
        <div class="rf-admin-risk-note" :class="{ danger: action.type === 'deleted' }">
          <AppIcon name="notice" size="18" /><span v-if="action.type === 'banned'">封禁会立即注销该用户全部会话，并隐藏其公开帖子、评论和资源。</span><span v-else-if="action.type === 'active'">解封只恢复登录权限，不会自动恢复此前被隐藏的内容。</span><span v-else>删除会匿名化账号并永久下线其帖子、评论和资源；此操作不可撤销。</span>
        </div>
        <label class="rf-field-label"><span>处理原因</span><textarea v-model="action.reason" class="rf-control rf-textarea" rows="4" maxlength="500" placeholder="至少填写 2 个字符，操作会写入审计日志" /></label>
        <label v-if="action.type === 'deleted'" class="rf-field-label"><span>输入“删除”确认</span><input v-model="action.confirmation" class="rf-control" autocomplete="off" /></label>
        <footer><button type="button" class="rf-secondary-button" :disabled="submitting" @click="closeAction">取消</button><nut-button :type="action.type === 'active' ? 'primary' : 'danger'" :loading="submitting" :disabled="!actionReady" @click="submitAction">{{ actionButton }}</nut-button></footer>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-admin-filter { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); }
.rf-admin-search { display: flex; min-width: 0; min-height: 42px; flex: 1; align-items: center; gap: 9px; padding-left: 12px; border: 1px solid var(--rf-line); border-radius: 999px; background: var(--rf-bg-subtle); }
.rf-admin-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.rf-admin-search input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; }
.rf-admin-search button { align-self: stretch; padding: 0 16px; border-radius: 999px; color: #fff; background: var(--primary); font-size: 12px; font-weight: 700; }
.rf-admin-user-list { border-top: 1px solid var(--rf-line); }
.rf-admin-user-row { display: grid; grid-template-columns: 44px minmax(190px, 1fr) auto auto auto; align-items: center; gap: 13px; padding: 15px 16px; border-bottom: 1px solid var(--rf-line); }
.rf-admin-user-main { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.rf-admin-user-main > span, .rf-admin-user-main > small { overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rf-admin-user-name { display: flex; min-width: 0; align-items: center; gap: 6px; }
.rf-admin-user-name strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rf-admin-user-counts { display: flex; gap: 12px; color: var(--rf-muted); font-size: 11px; }
.rf-admin-user-counts span { display: flex; flex-direction: column; align-items: center; }
.rf-admin-user-counts strong { color: var(--rf-text); font-size: 14px; font-variant-numeric: tabular-nums; }
.rf-admin-user-row .rf-row-actions { justify-content: flex-end; }
.rf-admin-user-row .rf-row-actions > small { color: var(--rf-muted); font-size: 11px; }
.rf-admin-risk-note { display: flex; align-items: flex-start; gap: 9px; margin: 16px 20px 0; padding: 12px; border-radius: 8px; color: #9a6700; background: color-mix(in srgb, #f4a100 10%, transparent); font-size: 13px; line-height: 1.5; }
.rf-admin-risk-note.danger { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
@media (max-width: 760px) {
  .rf-admin-filter { flex-wrap: wrap; padding-inline: 12px; }
  .rf-admin-search { flex-basis: 100%; }
  .rf-admin-filter .rf-control { flex: 1; }
  .rf-admin-user-row { grid-template-columns: 40px minmax(0, 1fr) auto; align-items: start; padding-inline: 12px; }
  .rf-admin-user-row :deep(.rf-user-avatar) { --rf-avatar-size: 40px !important; }
  .rf-admin-user-counts { grid-column: 2 / -1; justify-content: flex-start; }
  .rf-admin-user-counts span { flex-direction: row; gap: 4px; }
  .rf-admin-user-row > .rf-status-chip { grid-column: 3; grid-row: 1; }
  .rf-admin-user-row .rf-row-actions { grid-column: 2 / -1; justify-content: flex-start; }
}
</style>
