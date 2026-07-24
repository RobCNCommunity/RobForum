<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchAdminPayouts, reviewCreatorPayout, type CreatorPayout } from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'

const items = ref<CreatorPayout[]>([])
const loading = ref(true)
const reviewingID = ref<number | null>(null)
const status = ref('pending')
const modalOpen = ref(false)
const review = reactive<{ item: CreatorPayout | null; status: 'paid' | 'rejected'; note: string }>({ item: null, status: 'paid', note: '' })

async function load() {
  loading.value = true
  try {
    items.value = await fetchAdminPayouts(status.value)
  } catch (error) {
    Notify.danger(errorMessage(error, '提现记录加载失败'))
  } finally {
    loading.value = false
  }
}

function openReview(item: CreatorPayout, nextStatus: 'paid' | 'rejected') {
  review.item = item
  review.status = nextStatus
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
  reviewingID.value = review.item.id
  try {
    await reviewCreatorPayout(review.item.id, review.status, review.note.trim())
    Notify.success(review.status === 'paid' ? '已确认完成打款' : '提现申请已驳回')
    modalOpen.value = false
    review.item = null
    await load()
  } catch (error) {
    Notify.danger(errorMessage(error, '提现审核失败'))
  } finally {
    reviewingID.value = null
  }
}

function formatMoney(value: number) { return `¥${(value / 100).toFixed(2)}` }
function formatPercent(value: number) { return `${(Number(value || 0) / 100).toFixed(2).replace(/\.00$/, '')}%` }
function methodLabel(value: string) { return ({ alipay: '支付宝', bank: '银行卡', paypal: 'PayPal', other: '其他' } as Record<string, string>)[value] || value }
function statusLabel(value: string) { return ({ pending: '待处理', paid: '已打款', rejected: '已驳回' } as Record<string, string>)[value] || value }
onMounted(load)
</script>

<template>
  <PageContainer title="创作者提现">
    <template #extra>
      <div class="rf-filter-actions"><select v-model="status" class="rf-control rf-control--compact" aria-label="筛选提现状态" @change="load"><option value="pending">待处理</option><option value="paid">已打款</option><option value="rejected">已驳回</option><option value="">全部</option></select><button type="button" class="rf-icon-text-button" @click="load"><AppIcon name="repost" size="16" />刷新</button></div>
    </template>

    <div class="rf-inline-note rf-inline-note--warning"><AppIcon name="notice" size="18" /><div><strong>确认已经实际完成转账后再标记为“已打款”</strong><span>系统只负责冻结余额、记录审核和防止重复提现，不会自动向第三方账户转账。</span></div></div>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
    <div v-else-if="!items.length" class="rf-empty"><AppIcon name="shop" size="28" /><strong>暂无提现申请</strong><span>当前筛选条件下没有记录。</span></div>
    <div v-else class="rf-data-table" role="table" aria-label="创作者提现列表">
      <div class="rf-data-row rf-data-head" role="row"><span>创作者</span><span>申请 / 到账</span><span>收款方式</span><span>状态</span><span>申请时间</span><span>操作</span></div>
      <article v-for="item in items" :key="item.id" class="rf-data-row" role="row">
        <div data-label="创作者"><strong>{{ item.creator_name || `用户 #${item.creator_id}` }}</strong><small>{{ item.creator_email }}</small></div>
        <div data-label="申请 / 到账"><b class="rf-money">到账 {{ formatMoney(item.net_amount_cents || item.amount_cents) }}</b><small>申请 {{ formatMoney(item.amount_cents) }} · 手续费 {{ formatMoney(item.withdrawal_fee_cents || 0) }}（{{ formatPercent(item.withdrawal_fee_bps) }}）<template v-if="item.membership_tier_name"> · {{ item.membership_tier_name }}</template></small></div>
        <div data-label="收款方式"><span>{{ methodLabel(item.payout_method) }} · {{ item.account_name }}</span><small class="rf-break-all">{{ item.payout_account }}</small></div>
        <div data-label="状态"><span class="rf-status-chip" :class="item.status === 'paid' ? 'is-on' : item.status === 'rejected' ? 'is-danger' : 'is-warning'">{{ statusLabel(item.status) }}</span></div>
        <div data-label="申请时间"><time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time><small v-if="item.note">{{ item.note }}</small></div>
        <div data-label="操作" class="rf-row-actions" v-if="item.status === 'pending'"><button type="button" class="rf-primary-button rf-button-small" @click="openReview(item, 'paid')">已打款</button><button type="button" class="rf-danger-button rf-button-small" @click="openReview(item, 'rejected')">驳回</button></div>
        <div v-else data-label="处理结果"><small>{{ item.review_note || '已处理' }}</small></div>
      </article>
    </div>

    <div v-if="modalOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeReview">
      <section class="rf-dialog" role="dialog" aria-modal="true" :aria-label="review.status === 'paid' ? '确认已完成打款' : '驳回提现申请'">
        <header><div><h2>{{ review.status === 'paid' ? '确认已完成打款' : '驳回提现申请' }}</h2><p>{{ review.status === 'paid' ? '此操作表示管理员已经完成实际转账。' : '驳回后冻结金额会恢复为可提现余额。' }}</p></div><button type="button" class="rf-icon-button" aria-label="关闭" @click="closeReview"><AppIcon name="close" size="18" /></button></header>
        <div v-if="review.item" class="rf-review-summary"><span>{{ review.item.creator_name || `用户 #${review.item.creator_id}` }}<br /><small>申请 {{ formatMoney(review.item.amount_cents) }} · 手续费 {{ formatMoney(review.item.withdrawal_fee_cents || 0) }}</small></span><strong>实际打款 {{ formatMoney(review.item.net_amount_cents || review.item.amount_cents) }}</strong></div>
        <label class="rf-field-label"><span>审核备注</span><textarea v-model="review.note" class="rf-control rf-textarea" rows="4" maxlength="500" placeholder="填写转账流水号或驳回原因" /></label>
        <footer><button type="button" class="rf-secondary-button" :disabled="reviewingID !== null" @click="closeReview">取消</button><nut-button :type="review.status === 'paid' ? 'primary' : 'danger'" :loading="reviewingID !== null" @click="submitReview">{{ review.status === 'paid' ? '确认已打款' : '确认驳回' }}</nut-button></footer>
      </section>
    </div>
  </PageContainer>
</template>
