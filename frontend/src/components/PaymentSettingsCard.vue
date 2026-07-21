<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import { errorMessage, fetchPaymentConfig, updatePaymentConfig, type PaymentConfig } from '@/api'
import AppIcon from '@/components/AppIcon.vue'

const config = reactive<PaymentConfig>({
  enabled: false,
  kind: 'epay',
  gateway_url: '',
  merchant_id: '',
  secret: '',
  has_secret: false,
  pay_type: 'alipay',
  notify_url: '',
  return_url: '',
})
const loading = ref(true)
const saving = ref(false)

onMounted(async () => {
  try {
    Object.assign(config, await fetchPaymentConfig(), { secret: '' })
  } catch (error) {
    Notify.danger(errorMessage(error, '支付配置加载失败'))
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value = true
  try {
    Object.assign(config, await updatePaymentConfig({ ...config, secret: config.secret || undefined }), { secret: '' })
    Notify.success('支付配置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, '支付配置保存失败'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="setting-card">
    <div v-if="loading" class="rf-loading-block" aria-live="polite">
      <span class="rf-loading-bar" />
      <span>正在读取支付配置</span>
    </div>
    <template v-else>
      <header class="rf-panel-heading">
        <div>
          <h3><AppIcon name="shop" size="18" />支付网关</h3>
          <p>兼容 EPay 风格网关。密钥只会在服务端加密保存，回调仍由后端校验签名与订单幂等性。</p>
        </div>
        <span class="rf-status-chip" :class="config.enabled ? 'is-on' : ''">{{ config.enabled ? '已启用' : '未启用' }}</span>
      </header>

      <form class="rf-editor-form" @submit.prevent="save">
        <label class="rf-switch-row">
          <input v-model="config.enabled" type="checkbox" />
          <span class="rf-toggle" aria-hidden="true" />
          <span>启用支付网关</span>
        </label>

        <label class="rf-field-label">
          <span>网关类型</span>
          <select v-model="config.kind" class="rf-control">
            <option value="epay">EPay 兼容</option>
          </select>
        </label>
        <label class="rf-field-label"><span>网关地址</span><input v-model="config.gateway_url" class="rf-control" placeholder="https://pay.example.com" /></label>
        <label class="rf-field-label"><span>商户号</span><input v-model="config.merchant_id" class="rf-control" placeholder="商户 ID" /></label>
        <label class="rf-field-label"><span>{{ config.has_secret ? '密钥（留空保持不变）' : '密钥' }}</span><input v-model="config.secret" class="rf-control" type="password" placeholder="支付网关密钥" /></label>

        <div class="rf-form-grid rf-form-grid--two">
          <label class="rf-field-label"><span>支付类型</span><select v-model="config.pay_type" class="rf-control"><option value="alipay">支付宝</option><option value="wxpay">微信支付</option><option value="qqpay">QQ 钱包</option></select></label>
          <label class="rf-field-label"><span>通知地址</span><input v-model="config.notify_url" class="rf-control" placeholder="留空使用默认回调" /></label>
        </div>
        <label class="rf-field-label"><span>支付完成返回地址</span><input v-model="config.return_url" class="rf-control" placeholder="留空返回资源中心" /></label>

        <div v-if="config.has_secret" class="rf-inline-note rf-inline-note--info"><AppIcon name="success" size="16" />密钥已配置。重新输入并保存才会替换现有密钥。</div>
        <div class="rf-form-actions"><nut-button type="primary" :loading="saving" @click="save">保存支付设置</nut-button></div>
      </form>
    </template>
  </section>
</template>
