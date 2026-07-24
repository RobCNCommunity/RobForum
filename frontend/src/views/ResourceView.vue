<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createResourceOrder,
  createWalletResourceOrder,
  errorMessage,
  fetchMyOrders,
  fetchMyResources,
  fetchResources,
  fetchWallet,
  resourceDownloadURL,
  type CommerceOrder,
  type Resource,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import PageContainer from '@/components/PageContainer.vue'

const auth = useAuthStore()
const router = useRouter()
const resources = ref<Resource[]>([])
const mine = ref<Resource[]>([])
const orders = ref<CommerceOrder[]>([])
const walletBalance = ref(0)
const query = ref('')
const loading = ref(true)
const buyingID = ref<number | null>(null)
const walletBuyingID = ref<number | null>(null)

const purchasedResourceIDs = computed(() => new Set(orders.value.filter((order) => order.status === 'paid').map((order) => order.resource_id)))

function canDownload(item: Resource) {
  return item.price_cents === 0 || item.creator_id === auth.user?.id || purchasedResourceIDs.value.has(item.id)
}

async function load() {
  loading.value = true
  try {
    resources.value = await fetchResources(query.value.trim() || undefined)
    if (!auth.user) return
    const [mineResult, orderResult, walletResult] = await Promise.all([
      fetchMyResources(),
      fetchMyOrders(),
      fetchWallet(),
    ])
    mine.value = mineResult
    orders.value = orderResult
    walletBalance.value = walletResult.available_cents
  } catch (error) {
    Notify.danger(errorMessage(error, '资源加载失败'))
  } finally {
    loading.value = false
  }
}

function formatSize(value?: number) {
  if (!value) return '未知大小'
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function money(value: number) {
  return `¥${(value / 100).toFixed(2)}`
}

function status(value: string) {
  return ({ pending: '待审核', approved: '已发布', rejected: '已驳回', takedown: '已下架' } as Record<string, string>)[value] || value
}

function openResource(item: Resource) {
  router.push(`/resources/${item.id}`)
}

async function buy(item: Resource) {
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: `/resources/${item.id}` } })
    return
  }
  buyingID.value = item.id
  try {
    const order = await createResourceOrder(item.id)
    if (order.status === 'paid') {
      window.location.href = resourceDownloadURL(item.id)
      return
    }
    if (!order.payment_url) throw new Error('支付跳转地址为空')
    window.location.href = order.payment_url
  } catch (error) {
    Notify.danger(errorMessage(error, '订单创建失败'))
  } finally {
    buyingID.value = null
  }
}

async function buyWithWallet(item: Resource) {
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: `/resources/${item.id}` } })
    return
  }
  walletBuyingID.value = item.id
  try {
    await createWalletResourceOrder(item.id)
    Notify.success('已使用钱包余额购买资源')
    window.location.href = resourceDownloadURL(item.id)
  } catch (error) {
    Notify.danger(errorMessage(error, '余额支付失败'))
  } finally {
    walletBuyingID.value = null
  }
}

onMounted(load)
</script>

<template>
  <PageContainer>
    <header class="rf-resource-heading">
      <div>
        <p class="rf-kicker">RobForum 资源</p>
        <h1>资源中心</h1>
        <p>发现玩家分享的地图、脚本、素材和教程。每份资源都经过人工审核。</p>
      </div>
      <button v-if="auth.user" type="button" class="rf-primary-button" @click="router.push('/resources/new')"><AppIcon name="add" size="18" />发布资源</button>
    </header>

    <div class="rf-resource-toolbar">
      <label class="rf-resource-search"><AppIcon name="search" size="18" /><input v-model="query" placeholder="搜索资源、游戏或作者" @keydown.enter.prevent="load" /><button v-if="query" type="button" aria-label="清空搜索" @click="query = ''; load()">×</button></label>
      <button type="button" class="rf-secondary-button rf-button-small" :disabled="loading" @click="load">{{ loading ? '加载中…' : '刷新' }}</button>
    </div>

    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 5" :key="n" /></div>
    <div v-else class="rf-resource-layout rf-resource-layout--feed">
      <main>
        <div class="rf-section-label"><span>最新资源</span><small>{{ resources.length }} 项</small></div>
        <div v-if="resources.length" class="rf-resource-feed">
          <article v-for="item in resources" :key="item.id" class="rf-resource-row rf-resource-row--feed" :class="{ 'rf-resource-row--with-thumb': !!item.media?.[0] }" @click="openResource(item)">
            <div v-if="item.media?.[0]" class="rf-resource-thumb"><img :src="item.media[0].url" :alt="item.title" loading="lazy" /></div>
            <div class="rf-resource-row-content">
              <div class="rf-post-meta"><strong>{{ item.creator_name }}</strong><VerifiedBadge :verified="item.creator_verified" :label="item.creator_verification_label" /><MembershipBadge :active="item.creator_member" :tier-id="item.creator_membership_tier_id" /><span>·</span><span>{{ item.resource_type }}</span><span>·</span><span>{{ item.game }}{{ item.version ? ` · ${item.version}` : '' }}</span></div>
              <h2>{{ item.title }}</h2>
              <p>{{ item.description }}</p>
              <div class="rf-resource-meta"><span>{{ item.file?.original_name || '资源文件' }} · {{ formatSize(item.file?.size_bytes) }}</span><span>{{ item.download_count }} 下载 · {{ item.sales_count }} 购买</span><strong>{{ item.price_cents > 0 ? money(item.price_cents) : '免费' }}</strong></div>
              <div class="rf-resource-actions" @click.stop>
                <a class="rf-secondary-button rf-button-small" :href="`/resources/${item.id}`">查看详情</a>
                <a v-if="canDownload(item)" class="rf-primary-button rf-button-small" :href="resourceDownloadURL(item.id)">{{ item.price_cents === 0 ? '免费下载' : '下载资源' }}</a>
                <template v-else>
                  <button v-if="auth.user && walletBalance >= item.price_cents" type="button" class="rf-secondary-button rf-button-small" :disabled="walletBuyingID === item.id" @click="buyWithWallet(item)">{{ walletBuyingID === item.id ? '支付中…' : '余额支付' }}</button>
                  <button type="button" class="rf-primary-button rf-button-small" :disabled="buyingID === item.id" @click="buy(item)">{{ buyingID === item.id ? '处理中…' : '购买' }}</button>
                </template>
              </div>
            </div>
          </article>
        </div>
        <div v-else class="rf-empty"><AppIcon name="folder" size="28" /><strong>没有找到资源</strong><span>换个关键词再试试。</span></div>

        <section v-if="auth.user && mine.length" class="rf-panel rf-resource-history">
          <div class="rf-section-label"><span>我的投稿</span><RouterLink to="/wallet">收益与提现</RouterLink></div>
          <RouterLink v-for="item in mine" :key="item.id" :to="`/resources/${item.id}`" class="rf-history-row"><span>{{ item.title }}</span><b :class="`rf-status-${item.status}`">{{ status(item.status) }}</b></RouterLink>
        </section>
        <section v-if="auth.user && orders.length" class="rf-panel rf-resource-history">
          <div class="rf-section-label"><span>最近订单</span></div>
          <div v-for="order in orders.slice(0, 8)" :key="order.id" class="rf-history-row"><span>{{ order.resource_title }}</span><small>{{ money(order.amount_cents) }} · {{ order.status }}</small></div>
        </section>
      </main>

      <aside>
        <div v-if="auth.user" class="rf-panel rf-resource-cta"><AppIcon name="upload" size="22" /><h2>分享你的作品</h2><p>上传文件和预览图，提交后由管理员人工审核。</p><button type="button" class="rf-primary-button" @click="router.push('/resources/new')">开始投稿</button></div>
        <div v-else class="rf-panel rf-resource-cta"><AppIcon name="lock" size="22" /><h2>登录后投稿</h2><p>登录后可以发布资源、购买付费资源并查看投稿状态。</p><button type="button" class="rf-primary-button" @click="router.push('/login')">登录</button></div>
      </aside>
    </div>
  </PageContainer>
</template>
