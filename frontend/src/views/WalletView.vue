<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchWallet, type WalletSummary } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
const loading = ref(true); const wallet = ref<WalletSummary | null>(null); const yuan = (cents: number) => (Number(cents || 0) / 100).toFixed(2)
async function load() { loading.value = true; try { wallet.value = await fetchWallet() } catch (e) { Notify.danger(errorMessage(e, '钱包加载失败')) } finally { loading.value = false } }; onMounted(load)
</script>
<template><PageContainer title="我的钱包"><div v-if="loading" class="rf-loading-block">正在加载钱包…</div><div v-else class="rf-wallet"><section class="rf-balance"><div><span><AppIcon name="shop" size="18" />可用余额</span><strong>¥ {{ yuan(wallet?.available_cents || 0) }}</strong><small>余额来自资源售卖分成，提现请走创作者提现流程。</small></div></section><section class="rf-panel"><div class="rf-section-label"><span>流水记录</span><small>{{ wallet?.entries.length || 0 }} 条</small></div><div v-if="wallet?.entries.length" class="rf-ledger"><div v-for="entry in wallet.entries" :key="entry.id" class="rf-ledger-row"><span><strong>{{ entry.entry_type }}</strong><small>{{ entry.note || '账本记录' }}</small></span><b :class="{positive:entry.amount_cents >= 0}">{{ entry.amount_cents >= 0 ? '+' : '' }}¥{{ yuan(entry.amount_cents) }}</b><time>{{ new Date(entry.created_at).toLocaleString('zh-CN') }}</time></div></div><div v-else class="rf-empty">暂无流水记录</div></section></div></PageContainer></template>
