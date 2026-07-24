<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchAdminContentReports, fetchAdminPosts, moderateAdminPost, reviewAdminContentReport, type ContentReport, type Post } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import PostMediaGrid from '@/components/PostMediaGrid.vue'
import PostTagList from '@/components/PostTagList.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'

type ModerationTab = 'posts' | 'reports'
type PostAction = 'published' | 'rejected' | 'hidden' | 'deleted'
type ReportAction = 'accepted' | 'rejected'

const route = useRoute()
const router = useRouter()
const tab = ref<ModerationTab>(route.query.post_id ? 'posts' : route.query.tab === 'reports' ? 'reports' : 'posts')
const items = ref<Post[]>([])
const reports = ref<ContentReport[]>([])
const loading = ref(true)
const query = ref('')
const status = ref(String(route.query.status || (route.query.post_id ? '' : 'pending')))
const reportStatus = ref<'pending' | 'accepted' | 'rejected'>('pending')
const action = reactive<{ item: Post | null; type: PostAction; reason: string; confirmation: string }>({ item: null, type: 'published', reason: '', confirmation: '' })
const reportReview = reactive<{ item: ContentReport | null; type: ReportAction; note: string }>({ item: null, type: 'accepted', note: '' })
const submitting = ref(false)
const reportSubmitting = ref(false)
const dialogOpen = computed(() => action.item !== null)
const reportDialogOpen = computed(() => reportReview.item !== null)
const reasonRequired = computed(() => action.type !== 'published')
const actionReady = computed(() => (!reasonRequired.value || action.reason.trim().length >= 2) && (action.type !== 'deleted' || action.confirmation === '删除'))
const reportActionReady = computed(() => reportReview.note.trim().length >= 2)
const actionTitle = computed(() => ({ published: action.item?.status === 'pending' ? '通过帖子' : '恢复并公开帖子', rejected: '驳回帖子', hidden: '隐藏帖子', deleted: '删除帖子' })[action.type])
const reportActionTitle = computed(() => reportReview.type === 'accepted' ? '确认举报成立' : '驳回举报')

async function load() {
  loading.value = true
  try {
    if (tab.value === 'reports') {
      reports.value = await fetchAdminContentReports(reportStatus.value)
      return
    }
    const postID = Number(route.query.post_id || 0)
    items.value = await fetchAdminPosts({ status: status.value || undefined, q: query.value.trim() || undefined, post_id: postID > 0 ? postID : undefined })
  } catch (error) {
    Notify.danger(errorMessage(error, tab.value === 'reports' ? '举报审核列表加载失败' : '内容审核列表加载失败'))
  } finally {
    loading.value = false
  }
}

async function switchTab(next: ModerationTab) {
  if (tab.value === next) return
  tab.value = next
  if (next === 'reports') await router.replace({ path: '/admin/posts', query: { tab: 'reports' } })
  else await router.replace('/admin/posts')
  await load()
}

function openAction(item: Post, type: PostAction) {
  action.item = item
  action.type = type
  action.reason = ''
  action.confirmation = ''
}

function closeAction() {
  if (!submitting.value) resetAction()
}

function resetAction() {
  action.item = null
  action.reason = ''
  action.confirmation = ''
}

async function submitAction() {
  if (!action.item || !actionReady.value) {
    Notify.warn(action.type === 'deleted' ? '请填写原因并输入“删除”确认' : '请填写至少 2 个字符的审核原因')
    return
  }
  submitting.value = true
  try {
    await moderateAdminPost(action.item.id, action.type, action.reason.trim())
    Notify.success(({ published: '帖子已公开', rejected: '帖子已驳回', hidden: '帖子已隐藏', deleted: '帖子已删除' })[action.type])
    resetAction()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '帖子处理失败'))
  } finally {
    submitting.value = false
  }
}

function openReportReview(item: ContentReport, type: ReportAction) {
  reportReview.item = item
  reportReview.type = type
  reportReview.note = ''
}

function closeReportReview() {
  if (reportSubmitting.value) return
	resetReportReview()
}

function resetReportReview() {
  reportReview.item = null
  reportReview.note = ''
}

async function submitReportReview() {
  if (!reportReview.item || !reportActionReady.value) {
    Notify.warn('请填写至少 2 个字符的审核说明')
    return
  }
  reportSubmitting.value = true
  try {
    await reviewAdminContentReport(reportReview.item.id, reportReview.type, reportReview.note.trim())
    Notify.success(reportReview.type === 'accepted' ? '举报已确认，相关内容已隐藏' : '举报已驳回，已通知举报人')
    resetReportReview()
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '举报审核失败'))
  } finally {
    reportSubmitting.value = false
  }
}

function statusLabel(value: string) {
  return ({ pending: '待审核', published: '已公开', hidden: '已隐藏', rejected: '已驳回', deleted: '已删除' } as Record<string, string>)[value] || value
}

function statusClass(value: string) {
  return value === 'published' ? 'is-on' : value === 'pending' ? 'is-warning' : 'is-danger'
}

function reportStatusLabel(value: string) {
  return ({ pending: '被举报', accepted: '举报成立', rejected: '未成立' } as Record<string, string>)[value] || value
}

function reportStatusClass(value: string) {
  return value === 'pending' ? 'is-warning' : value === 'accepted' ? 'is-danger' : 'is-on'
}

function reportTargetLabel(value: ContentReport['target_type']) {
  return ({ post: '帖子', comment: '回复', profile: '个人资料' } as Record<string, string>)[value] || value
}

function reportTargetLink(item: ContentReport) {
  if (item.target_type === 'post') return `/posts/${item.target_id}`
  if (item.target_type === 'comment' && item.target_post_id) return `/posts/${item.target_post_id}#reply-${item.target_id}`
  return `/users/${item.target_user_id}`
}

function targetStatusLabel(item: ContentReport) {
  if (item.target_type === 'profile') return item.target_status === 'hidden' ? '资料已隐藏' : '资料公开中'
  return statusLabel(item.target_status)
}

async function clearPostFilter() {
  await router.replace('/admin/posts')
  tab.value = 'posts'
  status.value = 'pending'
  await load()
}

onMounted(load)
</script>

<template>
  <PageContainer title="内容审核">
    <nav class="rf-admin-moderation-tabs" aria-label="内容审核视图">
      <button type="button" :class="{ active: tab === 'posts' }" @click="switchTab('posts')">帖子审核</button>
      <button type="button" :class="{ active: tab === 'reports' }" @click="switchTab('reports')">举报队列</button>
    </nav>

    <template v-if="tab === 'posts'">
      <div v-if="route.query.post_id" class="rf-inline-note rf-inline-note--info"><AppIcon name="notice" size="17" /><div><strong>正在管理帖子 #{{ route.query.post_id }}</strong><span>操作后会立即影响公共访问。</span></div><button type="button" class="rf-text-button" @click="clearPostFilter">返回审核队列</button></div>
      <form class="rf-admin-filter" @submit.prevent="load">
        <label class="rf-admin-search"><AppIcon name="search" size="18" /><input v-model="query" maxlength="100" placeholder="搜索标题、正文或作者" /><button type="submit">搜索</button></label>
        <select v-model="status" class="rf-control rf-control--compact" aria-label="筛选帖子状态" @change="load">
          <option value="">全部状态</option><option value="pending">待审核</option><option value="published">已公开</option><option value="hidden">已隐藏</option><option value="rejected">已驳回</option><option value="deleted">已删除</option>
        </select>
        <button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button>
      </form>

      <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
      <div v-else-if="!items.length" class="rf-empty"><AppIcon name="success" size="28" /><strong>当前没有待处理内容</strong></div>
      <section v-else class="rf-admin-post-list">
        <article v-for="item in items" :key="item.id" class="rf-admin-post-row">
          <header>
            <UserAvatar :src="item.author_avatar" :name="item.author_name" :size="40" />
            <div><div class="rf-post-meta"><strong>{{ item.author_name }}</strong><VerifiedBadge :verified="item.author_verified" :label="item.author_verification_label" /><MembershipBadge :active="item.author_member" :tier-id="item.author_membership_tier_id" /><span>·</span><time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time></div><small>#{{ item.id }} · {{ item.board_name }}</small></div>
            <span class="rf-status-chip" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
          </header>
          <PostTagList :tags="item.tags" compact /><h2 v-if="item.title">{{ item.title }}</h2><p>{{ item.content }}</p>
          <PostMediaGrid v-if="item.media?.length" :media="item.media" />
          <footer><div class="rf-admin-post-metrics"><span>{{ item.views }} 浏览</span><span>{{ item.comment_count }} 评论</span><span>{{ item.like_count }} 点赞</span></div><div class="rf-row-actions">
            <button v-if="item.status === 'published'" type="button" class="rf-secondary-button rf-button-small" @click="router.push(`/posts/${item.id}`)">查看</button>
            <button v-if="['pending', 'hidden', 'rejected'].includes(item.status)" type="button" class="rf-primary-button rf-button-small" @click="openAction(item, 'published')">{{ item.status === 'pending' ? '通过' : '恢复公开' }}</button>
            <button v-if="item.status === 'pending' || item.status === 'published' || item.status === 'hidden'" type="button" class="rf-secondary-button rf-button-small" @click="openAction(item, 'rejected')">驳回</button>
            <button v-if="item.status === 'pending' || item.status === 'published'" type="button" class="rf-secondary-button rf-button-small" @click="openAction(item, 'hidden')">隐藏</button>
            <button v-if="item.status !== 'deleted'" type="button" class="rf-danger-button rf-button-small" @click="openAction(item, 'deleted')">删除</button>
          </div></footer>
        </article>
      </section>
    </template>

    <template v-else>
      <form class="rf-admin-filter rf-admin-filter--reports" @submit.prevent="load">
        <select v-model="reportStatus" class="rf-control rf-control--compact" aria-label="筛选举报状态" @change="load"><option value="pending">被举报</option><option value="accepted">举报成立</option><option value="rejected">未成立</option></select>
        <button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button>
      </form>
      <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 4" :key="n" /></div>
      <div v-else-if="!reports.length" class="rf-empty"><AppIcon name="success" size="28" /><strong>当前没有待处理举报</strong></div>
      <section v-else class="rf-admin-report-list">
        <article v-for="item in reports" :key="item.id" class="rf-admin-report-row">
          <header>
            <UserAvatar :src="item.reporter_avatar" :name="item.reporter_name" :size="40" />
            <div><div class="rf-post-meta"><strong>{{ item.reporter_name }}</strong><span>举报了{{ reportTargetLabel(item.target_type) }}</span><span>·</span><time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time></div><small>#{{ item.id }} · 目标用户 {{ item.target_user_name }}</small></div>
            <span class="rf-status-chip" :class="reportStatusClass(item.status)">{{ reportStatusLabel(item.status) }}</span>
          </header>
          <section class="rf-report-target"><div><span>{{ reportTargetLabel(item.target_type) }}</span><strong>{{ item.target_title }}</strong></div><p>{{ item.target_content || '目标内容已不可用。' }}</p><small>{{ targetStatusLabel(item) }}</small></section>
          <section class="rf-report-reason"><span>举报原因</span><p>{{ item.reason }}</p></section>
          <footer>
            <RouterLink v-if="item.status === 'pending'" :to="reportTargetLink(item)" class="rf-secondary-button rf-button-small">查看原内容</RouterLink>
            <div v-else class="rf-report-result"><span>{{ item.status === 'accepted' ? '已确认违规并隐藏' : '已驳回举报' }}</span><small>{{ item.review_note || '未填写审核说明' }}</small></div>
            <div v-if="item.status === 'pending'" class="rf-row-actions"><button type="button" class="rf-primary-button rf-button-small" @click="openReportReview(item, 'accepted')">确认违规</button><button type="button" class="rf-secondary-button rf-button-small" @click="openReportReview(item, 'rejected')">驳回举报</button></div>
          </footer>
        </article>
      </section>
    </template>

    <div v-if="dialogOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeAction"><section class="rf-dialog" role="dialog" aria-modal="true" :aria-label="actionTitle">
      <header><div><h2>{{ actionTitle }}</h2><p>#{{ action.item?.id }} · {{ action.item?.title }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="closeAction"><AppIcon name="close" size="18" /></button></header>
      <div class="rf-admin-risk-note" :class="{ danger: action.type === 'deleted' }"><AppIcon name="notice" size="18" /><span v-if="action.type === 'published'">通过后帖子会立即进入公共首页、搜索和作者主页。</span><span v-else-if="action.type === 'deleted'">删除会同步关闭评论并永久阻止帖子公开恢复，此操作不可撤销。</span><span v-else>帖子将立即停止公开展示，操作原因会保存在审计日志中。</span></div>
      <label class="rf-field-label"><span>处理原因{{ reasonRequired ? '' : '（可选）' }}</span><textarea v-model="action.reason" class="rf-control rf-textarea" rows="4" maxlength="500" :placeholder="reasonRequired ? '至少填写 2 个字符' : '可填写审核备注'" /></label>
      <label v-if="action.type === 'deleted'" class="rf-field-label"><span>输入“删除”确认</span><input v-model="action.confirmation" class="rf-control" autocomplete="off" /></label>
      <footer><button type="button" class="rf-secondary-button" :disabled="submitting" @click="closeAction">取消</button><nut-button :type="action.type === 'published' ? 'primary' : 'danger'" :loading="submitting" :disabled="!actionReady" @click="submitAction">确认{{ actionTitle }}</nut-button></footer>
    </section></div>

    <div v-if="reportDialogOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeReportReview"><section class="rf-dialog" role="dialog" aria-modal="true" :aria-label="reportActionTitle">
      <header><div><h2>{{ reportActionTitle }}</h2><p>#{{ reportReview.item?.id }} · {{ reportReview.item && reportTargetLabel(reportReview.item.target_type) }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="closeReportReview"><AppIcon name="close" size="18" /></button></header>
      <div class="rf-admin-risk-note" :class="{ danger: reportReview.type === 'accepted' }"><AppIcon name="notice" size="18" /><span v-if="reportReview.type === 'accepted'">确认后会隐藏被举报的{{ reportReview.item && reportTargetLabel(reportReview.item.target_type) }}，并通知举报人及目标用户。</span><span v-else>驳回后会通知举报人，本次举报将保留在审核记录中。</span></div>
      <label class="rf-field-label"><span>审核说明</span><textarea v-model="reportReview.note" class="rf-control rf-textarea" rows="4" maxlength="500" placeholder="至少填写 2 个字符，说明处理结论" /></label>
      <footer><button type="button" class="rf-secondary-button" :disabled="reportSubmitting" @click="closeReportReview">取消</button><nut-button :type="reportReview.type === 'accepted' ? 'danger' : 'primary'" :loading="reportSubmitting" :disabled="!reportActionReady" @click="submitReportReview">确认{{ reportActionTitle }}</nut-button></footer>
    </section></div>
  </PageContainer>
</template>

<style scoped>
.rf-admin-moderation-tabs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); min-height: 52px; border-bottom: 1px solid var(--rf-line); }
.rf-admin-moderation-tabs button { position: relative; color: var(--rf-text-muted); background: transparent; font-weight: 700; }
.rf-admin-moderation-tabs button:hover { background: var(--rf-bg-hover); }
.rf-admin-moderation-tabs button.active { color: var(--rf-text); }
.rf-admin-moderation-tabs button.active::after { position: absolute; right: 30%; bottom: 0; left: 30%; height: 3px; border-radius: var(--rf-pill); background: var(--primary); content: ''; }
.rf-admin-filter { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-bottom: 1px solid var(--rf-line); }
.rf-admin-filter--reports { justify-content: flex-end; }
.rf-admin-search { display: flex; min-width: 0; min-height: 42px; flex: 1; align-items: center; gap: 9px; padding-left: 12px; border: 1px solid var(--rf-line); border-radius: var(--rf-pill); background: var(--rf-bg-subtle); }
.rf-admin-search:focus-within { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.rf-admin-search input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; }
.rf-admin-search button { align-self: stretch; padding: 0 16px; border-radius: var(--rf-pill); color: #fff; background: var(--primary); font-size: 12px; font-weight: 700; }
.rf-admin-post-list, .rf-admin-report-list { border-top: 1px solid var(--rf-line); }
.rf-admin-post-row, .rf-admin-report-row { padding: 16px; border-bottom: 1px solid var(--rf-line); }
.rf-admin-post-row > header, .rf-admin-report-row > header { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: 10px; }
.rf-admin-post-row > header > div, .rf-admin-report-row > header > div { min-width: 0; }
.rf-admin-post-row > header small, .rf-admin-report-row > header small { display: block; margin-top: 3px; overflow: hidden; color: var(--rf-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.rf-admin-post-row h2 { margin: 13px 0 7px; font-size: 18px; line-height: 1.4; }
.rf-admin-post-row > p, .rf-report-target p { display: -webkit-box; max-height: 9.6em; margin: 0 0 12px; overflow: hidden; color: var(--rf-text); line-height: 1.6; white-space: pre-wrap; -webkit-box-orient: vertical; -webkit-line-clamp: 6; }
.rf-admin-post-row :deep(.rf-post-media-grid) { max-width: 540px; margin-top: 12px; }
.rf-admin-post-row > footer, .rf-admin-report-row > footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-top: 14px; padding-top: 11px; border-top: 1px solid var(--rf-line); }
.rf-admin-post-metrics { display: flex; gap: 14px; color: var(--rf-muted); font-size: 11px; }
.rf-report-target { margin-top: 14px; padding: 13px 14px 1px; border-left: 3px solid var(--rf-line); background: var(--rf-bg-subtle); }
.rf-report-target > div { display: flex; min-width: 0; align-items: baseline; gap: 8px; }
.rf-report-target span, .rf-report-reason > span { flex: 0 0 auto; color: var(--rf-muted); font-size: 11px; font-weight: 700; }
.rf-report-target strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.rf-report-target small { display: block; margin-bottom: 11px; color: var(--rf-muted); font-size: 11px; }
.rf-report-reason { margin-top: 12px; }
.rf-report-reason p { margin: 4px 0 0; color: var(--rf-text); line-height: 1.5; white-space: pre-wrap; overflow-wrap: anywhere; }
.rf-report-result { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.rf-report-result span { color: var(--rf-text); font-size: 13px; font-weight: 700; }
.rf-report-result small { overflow: hidden; color: var(--rf-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rf-admin-risk-note { display: flex; align-items: flex-start; gap: 9px; margin: 16px 20px 0; padding: 12px; border-radius: 8px; color: #9a6700; background: color-mix(in srgb, #f4a100 10%, transparent); font-size: 13px; line-height: 1.5; }
.rf-admin-risk-note.danger { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }
@media (max-width: 760px) {
  .rf-admin-filter { flex-wrap: wrap; padding-inline: 12px; }
  .rf-admin-filter--reports { justify-content: flex-start; }
  .rf-admin-search { flex-basis: 100%; }
  .rf-admin-filter .rf-control { flex: 1; }
  .rf-admin-post-row, .rf-admin-report-row { padding-inline: 12px; }
  .rf-admin-post-row > footer, .rf-admin-report-row > footer { align-items: flex-start; flex-direction: column; }
  .rf-admin-post-row .rf-row-actions, .rf-admin-report-row .rf-row-actions { width: 100%; justify-content: flex-start; }
  .rf-admin-report-row > footer > .rf-secondary-button { width: 100%; }
}
</style>
