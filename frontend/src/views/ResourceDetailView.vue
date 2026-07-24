<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ImagePreview, Notify } from '@nutui/nutui'
import { createResourceOrder, createWalletResourceOrder, errorMessage, fetchMyOrders, fetchResource, resourceDownloadURL, type Resource } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'
import PageContainer from '@/components/PageContainer.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const resource = ref<Resource | null>(null)
const loading = ref(true)
const current = ref(0)
const buying = ref(false)
const walletBuying = ref(false)
const purchased = ref(false)
let timer: number | undefined

const media = computed(() => resource.value?.media || [])
const currentMedia = computed(() => media.value[current.value])

function formatSize(value?: number) {
  if (!value) return '未知大小'
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function money(value: number) {
  return `¥${(value / 100).toFixed(2)}`
}

function setSlide(index: number) {
  const count = media.value.length
  if (!count) return
  current.value = (index + count) % count
}

function openPreview() {
  if (!media.value.length) return
  ImagePreview({ show: true, images: media.value.map((item) => ({ src: item.url })), initNo: current.value, contentClose: true, closeable: true, isLoop: media.value.length > 1, maxZoom: 4 })
}

async function load() {
  loading.value = true
  try {
    const id = Number(route.params.id)
    if (!Number.isInteger(id) || id < 1) throw new Error('资源编号无效')
    resource.value = await fetchResource(id)
    if (auth.user) {
      const orders = await fetchMyOrders()
      purchased.value = resource.value.creator_id === auth.user.id || orders.some((order) => order.resource_id === id && order.status === 'paid')
    }
    current.value = 0
    if (media.value.length > 1) {
      timer = window.setInterval(() => setSlide(current.value + 1), 5000)
    }
  } catch (error) {
    Notify.danger(errorMessage(error, '资源不存在或尚未通过审核'))
    router.push('/resources')
  } finally {
    loading.value = false
  }
}

async function buy() {
  if (!resource.value) return
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  buying.value = true
  try {
    const order = await createResourceOrder(resource.value.id)
    if (order.status === 'paid') {
	  purchased.value = true
      window.location.href = resourceDownloadURL(resource.value.id)
      return
    }
    if (!order.payment_url) throw new Error('支付跳转地址为空')
    window.location.href = order.payment_url
  } catch (error) {
    Notify.danger(errorMessage(error, '订单创建失败'))
  } finally {
    buying.value = false
  }
}

async function buyWithWallet() {
  if (!resource.value) return
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  walletBuying.value = true
  try {
    const order = await createWalletResourceOrder(resource.value.id)
    purchased.value = order.status === 'paid'
    Notify.success('已使用钱包余额购买资源')
    window.location.href = resourceDownloadURL(resource.value.id)
  } catch (error) {
    Notify.danger(errorMessage(error, '余额支付失败'))
  } finally {
    walletBuying.value = false
  }
}

onMounted(load)
onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
</script>

<template>
  <PageContainer>
    <div v-if="loading" class="rf-list-loading rf-list-loading--wide"><span v-for="n in 4" :key="n" /></div>
    <article v-else-if="resource" class="rf-resource-detail">
      <button type="button" class="rf-icon-text-button rf-resource-back" @click="router.back()"><AppIcon name="back" size="18" />返回</button>
      <section class="rf-resource-hero" :class="{ 'rf-resource-hero--empty': !currentMedia }" :style="currentMedia ? { backgroundImage: `url(${currentMedia.url})` } : undefined">
        <div class="rf-resource-hero-scrim" />
        <div class="rf-resource-hero-copy"><span class="rf-resource-type">{{ resource.resource_type }}</span><h1>{{ resource.title }}</h1><p>{{ resource.game }}{{ resource.version ? ` · ${resource.version}` : '' }}</p></div>
        <button v-if="currentMedia" type="button" class="rf-resource-hero-preview" aria-label="预览资源图片" @click="openPreview"><img :src="currentMedia.url" :alt="resource.title" /></button>
        <div v-if="media.length > 1" class="rf-resource-carousel-controls"><button type="button" aria-label="上一张" @click="setSlide(current - 1)">‹</button><div><button v-for="(_, index) in media" :key="index" type="button" :class="{ active: index === current }" :aria-label="`第 ${index + 1} 张`" @click="setSlide(index)" /></div><button type="button" aria-label="下一张" @click="setSlide(current + 1)">›</button></div>
      </section>

      <div class="rf-resource-detail-grid">
        <main>
          <header class="rf-resource-detail-author"><RouterLink :to="`/users/${resource.creator_id}`"><strong>{{ resource.creator_name }}</strong></RouterLink><VerifiedBadge :verified="resource.creator_verified" :label="resource.creator_verification_label" /><MembershipBadge :active="resource.creator_member" :tier-id="resource.creator_membership_tier_id" /><time>{{ new Date(resource.created_at).toLocaleDateString('zh-CN') }}</time></header>
          <section class="rf-resource-description"><h2>详细介绍</h2><p>{{ resource.description }}</p></section>
          <section class="rf-resource-facts"><div><span>文件</span><strong>{{ resource.file?.original_name || '资源文件' }}</strong><small>{{ formatSize(resource.file?.size_bytes) }}</small></div><div><span>下载</span><strong>{{ resource.download_count }}</strong></div><div><span>购买</span><strong>{{ resource.sales_count }}</strong></div><div><span>版本</span><strong>{{ resource.version || '未填写' }}</strong></div></section>
        </main>
        <aside class="rf-resource-purchase"><span class="rf-resource-purchase-label">资源价格</span><strong>{{ resource.price_cents ? money(resource.price_cents) : '免费' }}</strong><a v-if="!resource.price_cents || purchased || resource.creator_id === auth.user?.id" class="rf-primary-button" :href="resourceDownloadURL(resource.id)"><AppIcon name="download" size="17" />{{ resource.price_cents ? '下载资源' : '免费下载' }}</a><template v-else><button v-if="auth.user" type="button" class="rf-secondary-button" :disabled="walletBuying" @click="buyWithWallet">{{ walletBuying ? '支付中…' : '余额支付' }}</button><button type="button" class="rf-primary-button" :disabled="buying" @click="buy">{{ buying ? '处理中…' : '购买资源' }}</button></template><small>通过审核的资源才会提供下载。请先阅读介绍和版本信息。</small></aside>
      </div>
    </article>
  </PageContainer>
</template>

<style scoped>
.rf-resource-back { margin-bottom: 12px; padding-left: 0; }
.rf-resource-detail { min-width: 0; }
.rf-resource-hero { position: relative; display: flex; min-height: 360px; align-items: flex-end; overflow: hidden; padding: 34px; border-radius: 14px; background-color: #20242a; background-position: center; background-size: cover; color: #fff; }
.rf-resource-hero--empty { background: var(--rf-bg-subtle); color: var(--rf-text); }
.rf-resource-hero-scrim { position: absolute; inset: 0; background: linear-gradient(180deg, rgba(0,0,0,.05), rgba(0,0,0,.76)); }
.rf-resource-hero--empty .rf-resource-hero-scrim { display: none; }
.rf-resource-hero-copy { position: relative; z-index: 1; max-width: 680px; }
.rf-resource-type { color: rgba(255,255,255,.78); font-size: 12px; text-transform: uppercase; }
.rf-resource-hero--empty .rf-resource-type { color: var(--rf-muted); }
.rf-resource-hero h1 { margin: 7px 0 5px; font-size: clamp(28px, 5vw, 46px); line-height: 1.08; letter-spacing: -.035em; }
.rf-resource-hero p { margin: 0; color: rgba(255,255,255,.82); }
.rf-resource-hero--empty p { color: var(--rf-muted); }
.rf-resource-hero-preview { position: absolute; z-index: 1; top: 22px; right: 22px; width: 96px; height: 72px; overflow: hidden; padding: 0; border: 2px solid rgba(255,255,255,.75); border-radius: 8px; background: rgba(0,0,0,.2); cursor: zoom-in; }
.rf-resource-hero-preview img { width: 100%; height: 100%; object-fit: cover; }
.rf-resource-carousel-controls { position: absolute; z-index: 2; right: 22px; bottom: 22px; display: flex; align-items: center; gap: 10px; }
.rf-resource-carousel-controls > button { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 50%; color: #fff; background: rgba(0,0,0,.45); font-size: 22px; line-height: 1; }
.rf-resource-carousel-controls > div { display: flex; gap: 5px; }
.rf-resource-carousel-controls > div button { width: 7px; height: 7px; padding: 0; border-radius: 50%; background: rgba(255,255,255,.48); }
.rf-resource-carousel-controls > div button.active { background: #fff; }
.rf-resource-detail-grid { display: block; min-width: 0; padding-top: 24px; }
.rf-resource-detail-author { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; color: var(--rf-muted); }
.rf-resource-detail-author strong { color: var(--rf-text); }
.rf-resource-detail-author time { margin-left: 4px; font-size: 13px; }
.rf-resource-description { padding: 28px 0; border-bottom: 1px solid var(--rf-line); }
.rf-resource-description h2 { margin: 0 0 12px; font-size: 18px; }
.rf-resource-description p { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; }
.rf-resource-facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; padding: 22px 0; }
.rf-resource-facts div { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.rf-resource-facts span, .rf-resource-facts small { color: var(--rf-muted); font-size: 12px; }
.rf-resource-facts strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rf-resource-purchase { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 10px 12px; margin-top: 24px; padding: 18px 20px; border: 1px solid var(--rf-line); border-right: 0; border-left: 0; border-radius: 0; background: var(--rf-bg-subtle); }
.rf-resource-purchase-label { flex-basis: 100%; color: var(--rf-muted); font-size: 12px; }
.rf-resource-purchase > strong { margin-right: auto; font-size: 30px; letter-spacing: -.02em; }
.rf-resource-purchase > .rf-primary-button, .rf-resource-purchase > .rf-secondary-button { flex: 0 0 auto; }
.rf-resource-purchase small { flex-basis: 100%; color: var(--rf-muted); font-size: 12px; line-height: 1.5; }
@media (max-width: 760px) { .rf-resource-hero { min-height: 300px; padding: 22px 18px; border-radius: 10px; } .rf-resource-hero-preview { top: 14px; right: 14px; width: 76px; height: 58px; } .rf-resource-carousel-controls { right: 14px; bottom: 14px; } .rf-resource-purchase { margin-top: 12px; padding-inline: 12px; } .rf-resource-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
