<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchAdminVerifications, reviewVerificationApplication, type VerificationApplication } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const items = ref<VerificationApplication[]>([])
const loading = ref(true)
const status = ref('pending')
const modalOpen = ref(false)
const reviewingID = ref<number | null>(null)
const review = reactive<{ item: VerificationApplication | null; status: 'approved' | 'rejected'; label: string; note: string }>({ item: null, status: 'approved', label: '', note: '' })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminVerifications(status.value)
  } catch (error) {
    Notify.danger(errorMessage(error, '认证申请加载失败'))
  } finally {
    loading.value = false
  }
}

function openReview(item: VerificationApplication, nextStatus: 'approved' | 'rejected') {
  review.item = item
  review.status = nextStatus
  review.label = item.requested_label
  review.note = ''
  modalOpen.value = true
}

function closeReview() {
  if (reviewingID.value !== null) return
  modalOpen.value = false
  review.item = null
}

async function submitReview() {
  if (!review.item) return
  if (review.status === 'approved' && !review.label.trim()) {
    Notify.warn('请填写认证显示名称')
    return
  }
  reviewingID.value = review.item.id
  try {
    await reviewVerificationApplication(review.item.id, review.status, review.label.trim(), review.note.trim())
    Notify.success(review.status === 'approved' ? '认证已通过，蓝标已生效' : review.item.status === 'approved' ? '认证已撤销，用户蓝标已取消' : '认证申请已驳回')
    modalOpen.value = false
    review.item = null
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '认证审核失败'))
  } finally {
    reviewingID.value = null
  }
}

function typeLabel(value: string) { return ({ creator: '创作者', developer: '开发者/工作室', organization: '组织/社群', community: '社区贡献者' } as Record<string, string>)[value] || value }
function statusLabel(value: string) { return ({ pending: '待审核', approved: '已通过', rejected: '已驳回' } as Record<string, string>)[value] || value }
onMounted(load)
</script>

<template>
  <PageContainer title="蓝微认证审核">
    <template #extra><div class="rf-filter-actions"><select v-model="status" class="rf-control rf-control--compact" aria-label="筛选认证状态" @change="load"><option value="pending">待审核</option><option value="approved">已通过</option><option value="rejected">已驳回</option><option value="">全部</option></select><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></div></template>
    <div class="rf-inline-note rf-inline-note--warning"><AppIcon name="notice" size="18" /><div><strong>请核验材料真实性后再通过</strong><span>审核结果和管理员备注会写入审计日志，用户邮箱只对后台管理员可见。</span></div></div>
    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
    <div v-else-if="!items.length" class="rf-empty"><AppIcon name="success" size="28" /><strong>暂无认证申请</strong><span>当前筛选条件下没有记录。</span></div>
    <div v-else class="rf-data-table rf-data-table--verification" role="table" aria-label="蓝微认证申请">
      <div class="rf-data-row rf-data-head" role="row"><span>申请人</span><span>类型</span><span>认证名称</span><span>证明材料</span><span>状态</span><span>操作</span></div>
      <article v-for="item in items" :key="item.id" class="rf-data-row" role="row">
        <div data-label="申请人"><strong>{{ item.user_name }}</strong><small>{{ item.user_email }}</small></div>
        <div data-label="类型">{{ typeLabel(item.verification_type) }}</div>
        <div data-label="认证名称"><strong>{{ item.requested_label }}</strong><small>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</small></div>
        <div data-label="证明材料"><a :href="item.evidence_url" target="_blank" rel="noreferrer">打开公开材料 <AppIcon name="arrow" size="14" /></a><small>{{ item.statement }}</small></div>
        <div data-label="状态"><span class="rf-status-chip" :class="item.status === 'approved' ? 'is-on' : item.status === 'rejected' ? 'is-danger' : 'is-warning'">{{ statusLabel(item.status) }}</span></div>
        <div v-if="item.status === 'pending'" data-label="操作" class="rf-row-actions"><button type="button" class="rf-primary-button rf-button-small" @click="openReview(item, 'approved')">通过</button><button type="button" class="rf-danger-button rf-button-small" @click="openReview(item, 'rejected')">驳回</button></div>
        <div v-else-if="item.status === 'approved'" data-label="操作" class="rf-row-actions"><button type="button" class="rf-danger-button rf-button-small" @click="openReview(item, 'rejected')">撤销认证</button></div>
        <div v-else data-label="处理结果"><small>{{ item.review_note || '已处理' }}</small></div>
      </article>
    </div>

    <div v-if="modalOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeReview"><section class="rf-dialog" role="dialog" aria-modal="true" :aria-label="review.status === 'approved' ? '通过蓝微认证' : review.item?.status === 'approved' ? '撤销蓝微认证' : '驳回认证申请'"><header><div><h2>{{ review.status === 'approved' ? '通过蓝微认证' : review.item?.status === 'approved' ? '撤销蓝微认证' : '驳回认证申请' }}</h2><p v-if="review.item">{{ review.item.user_name }} · {{ typeLabel(review.item.verification_type) }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="closeReview"><AppIcon name="close" size="18" /></button></header><label v-if="review.status === 'approved'" class="rf-field-label"><span>显示认证名称</span><input v-model="review.label" class="rf-control" maxlength="80" /></label><label class="rf-field-label"><span>{{ review.item?.status === 'approved' ? '撤销原因' : '审核备注' }}</span><textarea v-model="review.note" class="rf-control rf-textarea" rows="4" maxlength="500" :placeholder="review.item?.status === 'approved' ? '请填写撤销认证的原因' : '填写通过说明或驳回原因'" /></label><footer><button type="button" class="rf-secondary-button" :disabled="reviewingID !== null" @click="closeReview">取消</button><nut-button :type="review.status === 'approved' ? 'primary' : 'danger'" :loading="reviewingID !== null" @click="submitReview">{{ review.status === 'approved' ? '确认通过' : review.item?.status === 'approved' ? '确认撤销' : '确认驳回' }}</nut-button></footer></section></div>
  </PageContainer>
</template>
