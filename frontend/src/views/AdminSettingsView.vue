<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Notify } from '@nutui/nutui'
import {
  errorMessage,
  deleteVerificationBadge,
  fetchAdminSite,
  fetchCaptcha,
  fetchOAuth,
  fetchSMTP,
  testSMTP,
  updateAdminSite,
  updateCaptcha,
  updateOAuth,
  updateSMTP,
  type CaptchaConfig,
  type OAuthConfig,
  type SMTPConfig,
  type SiteSettings,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import MembershipSettingsPanel from '@/components/MembershipSettingsPanel.vue'
import PaymentSettingsCard from '@/components/PaymentSettingsCard.vue'
import { useSiteStore } from '@/stores/site'

const siteStore = useSiteStore()
const site = reactive<SiteSettings>({
  site_name: '', site_description: '', logo_url: '', avatar_url: '', verification_badge_url: '', primary_color: '#1d9bf0', public_url: '',
  allow_register: true, require_email_verification: false, post_review_required: true, allowed_email_domains: [],
  banner_enabled: false, banner_text: '', banner_link: '', updated_at: '',
})
const smtp = reactive<SMTPConfig>({ enabled: false, host: '', port: 587, username: '', password: '', has_password: false, from_email: '', from_name: '', tls_mode: 'starttls' })
const captcha = reactive<CaptchaConfig>({ enabled: false, provider: 'gt4', site_key: '', endpoint: '', secret: '', has_secret: false, adapter_status: 'disabled' })
const oauth = reactive<OAuthConfig>({ enabled: false, provider_key: 'oidc', provider_name: 'OAuth', client_id: '', client_secret: '', has_client_secret: false, authorization_url: '', token_url: '', userinfo_url: '', scopes: 'openid email profile', token_auth_method: 'client_secret_post', require_verified_email: true, updated_at: '' })
const loading = ref(true)
const savingSite = ref(false)
const savingSMTP = ref(false)
const savingCaptcha = ref(false)
const savingOAuth = ref(false)
const sendingTest = ref(false)
const testRecipient = ref('')
const allowedDomainsText = ref('')
const badgeFiles = ref<any[]>([])
const uploadHeaders = computed(() => ({ 'X-CSRF-Token': readCookie('roblox_csrf') }))

onMounted(async () => {
  try {
    const [siteData, smtpData, captchaData, oauthData] = await Promise.all([fetchAdminSite(), fetchSMTP(), fetchCaptcha(), fetchOAuth()])
    Object.assign(site, siteData)
    badgeFiles.value = siteData.verification_badge_url ? [{ uid: 'verification-badge', name: '认证标志', url: siteData.verification_badge_url, status: 'success', type: 'image/png' }] : []
    allowedDomainsText.value = siteData.allowed_email_domains.join(', ')
    Object.assign(smtp, smtpData, { password: '' })
    Object.assign(captcha, captchaData, { secret: '' })
    Object.assign(oauth, oauthData, { client_secret: '', clear_client_secret: false })
  } catch (error) {
    Notify.danger(errorMessage(error, '管理配置加载失败'))
  } finally {
    loading.value = false
  }
})

function normalizedDomains() {
  return [...new Set(allowedDomainsText.value.split(/[\s,，;；]+/).map((item) => item.trim().toLowerCase().replace(/^@/, '')).filter(Boolean))]
}

async function saveSite() {
  if (!site.site_name.trim()) { Notify.warn('社区名称不能为空'); return }
  savingSite.value = true
  try {
    site.allowed_email_domains = normalizedDomains()
    const updated = await updateAdminSite({ ...site })
    Object.assign(site, updated)
    siteStore.setSettings(updated)
    allowedDomainsText.value = site.allowed_email_domains.join(', ')
    if (/^#[0-9a-f]{6}$/i.test(site.primary_color)) document.documentElement.style.setProperty('--primary', site.primary_color)
    Notify.success('站点设置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, '站点设置保存失败'))
  } finally {
    savingSite.value = false
  }
}

function readCookie(name: string) {
  if (typeof document === 'undefined') return ''
  const value = document.cookie.split('; ').find((item) => item.startsWith(`${name}=`))
  return value ? decodeURIComponent(value.slice(name.length + 1)) : ''
}

function parseUploadSettings(payload: any): SiteSettings {
  const parsed = typeof payload?.responseText === 'string' ? JSON.parse(payload.responseText) : payload
  if (!parsed?.data?.verification_badge_url && !parsed?.data?.site_name) throw new Error('认证标志上传响应无效')
  return parsed.data
}

function handleBadgeSuccess(payload: any) {
  try {
    const updated = parseUploadSettings(payload)
    Object.assign(site, updated)
    siteStore.setSettings(updated)
    Notify.success('认证标志已上传')
  } catch (error) {
    Notify.danger(errorMessage(error, '认证标志上传响应无效'))
  }
}

function handleBadgeFailure(payload: any) {
  let message = '认证标志上传失败'
  try {
    const parsed = typeof payload?.responseText === 'string' ? JSON.parse(payload.responseText) : payload
    message = parsed?.error?.message || parsed?.data?.error?.message || message
  } catch { /* keep the safe fallback */ }
  Notify.danger(message)
}

async function clearBadge() {
  if (!site.verification_badge_url) return
  try {
    const updated = await deleteVerificationBadge()
    Object.assign(site, updated)
    siteStore.setSettings(updated)
    badgeFiles.value = []
    Notify.success('认证标志已移除')
  } catch (error) {
    Notify.danger(errorMessage(error, '认证标志移除失败'))
  }
}

function handleBadgeOversize() { Notify.warn('认证标志不能超过 2 MB') }

async function saveSMTP() {
  savingSMTP.value = true
  try {
    Object.assign(smtp, await updateSMTP({ ...smtp, password: smtp.password || undefined }))
    smtp.password = ''
    Notify.success('SMTP 设置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, 'SMTP 设置保存失败'))
  } finally {
    savingSMTP.value = false
  }
}

async function sendTest() {
  if (!testRecipient.value.trim()) { Notify.warn('请输入测试收件邮箱'); return }
  sendingTest.value = true
  try {
    await testSMTP(testRecipient.value.trim())
    Notify.success('测试邮件已发送')
  } catch (error) {
    Notify.danger(errorMessage(error, '测试邮件发送失败'))
  } finally {
    sendingTest.value = false
  }
}

async function saveCaptcha() {
  savingCaptcha.value = true
  try {
    Object.assign(captcha, await updateCaptcha({
      enabled: captcha.enabled,
      provider: captcha.provider,
      site_key: captcha.site_key.trim(),
      endpoint: captcha.endpoint.trim() || (captcha.provider === 'gt4' ? 'https://gcaptcha4.geetest.com/validate' : ''),
      masked_secret: captcha.secret?.trim() || captcha.masked_secret,
    }))
    captcha.secret = ''
    Notify.success('验证码配置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, '验证码配置保存失败'))
  } finally {
    savingCaptcha.value = false
  }
}

async function saveOAuth() {
  savingOAuth.value = true
  try {
    const updated = await updateOAuth({ ...oauth, client_secret: oauth.client_secret?.trim() || undefined })
    Object.assign(oauth, updated, { client_secret: '', clear_client_secret: false })
    Notify.success('OAuth 登录配置已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, 'OAuth 配置保存失败'))
  } finally {
    savingOAuth.value = false
  }
}

function adapterLabel(value: string) {
  return ({ active: '已接入', disabled: '未启用', reserved_adapter: '配置已保存，适配器待接入', unsupported: '当前服务商暂不支持' } as Record<string, string>)[value] || value
}
</script>

<template>
  <PageContainer title="系统设置">
    <div v-if="loading" class="rf-settings-loading"><span v-for="n in 4" :key="n" /></div>
    <div v-else class="admin-grid">
      <section class="setting-card wide-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="settings" size="18" />社区品牌与注册策略</h3><p>这些信息会显示在首页、登录页、导航和首次加载画面中。</p></div><span v-if="site.logo_url || site.avatar_url" class="rf-brand-preview"><img :src="site.logo_url || site.avatar_url" alt="社区标志预览" /></span></header>
        <form class="rf-editor-form" @submit.prevent="saveSite">
          <div class="rf-field-label rf-verification-upload"><span>认证标志</span><small>认证用户旁边显示的标志。支持 PNG、JPG，最大 2 MB。</small><nut-uploader v-model:file-list="badgeFiles" url="/api/v1/admin/site/verification-badge" accept="image/png,image/jpeg" list-type="list" maximum="1" :maximize="2 * 1024 * 1024" :headers="uploadHeaders" with-credentials @success="handleBadgeSuccess" @failure="handleBadgeFailure" @oversize="handleBadgeOversize" @delete="clearBadge"><nut-button type="primary" size="small">上传认证标志</nut-button></nut-uploader></div>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>社区名称</span><input v-model="site.site_name" class="rf-control" placeholder="罗布玩家社区" /></label><label class="rf-field-label"><span>主色</span><span class="rf-color-control"><input v-model="site.primary_color" class="rf-control" placeholder="#1d9bf0" /><input v-model="site.primary_color" type="color" aria-label="选择主色" /></span></label></div>
          <label class="rf-field-label"><span>站点简介</span><input v-model="site.site_description" class="rf-control" maxlength="240" /></label>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>Logo URL</span><input v-model="site.logo_url" class="rf-control" type="url" placeholder="https://.../logo.png" /></label><label class="rf-field-label"><span>社区头像 URL</span><input v-model="site.avatar_url" class="rf-control" type="url" placeholder="https://.../avatar.png" /></label></div>
          <label class="rf-field-label"><span>公网地址</span><input v-model="site.public_url" class="rf-control" type="url" placeholder="https://community.example.com" /></label>

          <div class="rf-form-section"><div><strong>全站横幅</strong><p>用于展示一条短公告，可链接到活动或规则页面。</p></div><label class="rf-switch-row"><input v-model="site.banner_enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>{{ site.banner_enabled ? '已开启' : '已关闭' }}</span></label></div>
          <label class="rf-field-label"><span>横幅文字</span><input v-model="site.banner_text" class="rf-control" maxlength="240" placeholder="例如：社区活动正在进行中" /></label>
          <label class="rf-field-label"><span>点击跳转链接</span><input v-model="site.banner_link" class="rf-control" type="url" placeholder="可留空，仅展示横幅" /></label>

          <div class="rf-choice-row"><label class="rf-switch-row"><input v-model="site.allow_register" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>允许注册</span></label><label class="rf-switch-row"><input v-model="site.require_email_verification" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>注册需要邮箱验证</span></label><label class="rf-switch-row"><input v-model="site.post_review_required" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>发帖先审后发</span></label></div>
          <div class="rf-inline-note rf-inline-note--warning"><AppIcon name="notice" size="16" /><span>开启后，普通用户新发的帖子会进入待审核队列；管理员发布的帖子直接公开。审核通过前不会出现在公共首页、搜索或帖子详情。</span></div>
          <label class="rf-field-label"><span>允许注册的邮箱域名</span><textarea v-model="allowedDomainsText" class="rf-control rf-textarea" rows="3" placeholder="qq.com, 163.com, outlook.com, gmail.com" /><small>使用逗号、分号或换行分隔；留空表示不限制域名。</small></label>
          <div class="rf-form-actions"><nut-button type="primary" :loading="savingSite" @click="saveSite">保存社区设置</nut-button></div>
        </form>
      </section>

      <MembershipSettingsPanel />

      <section class="setting-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="message" size="18" />SMTP 邮件</h3><p>用于注册验证码、密码重置和后台测试邮件。</p></div><span class="rf-status-chip" :class="smtp.enabled ? 'is-on' : ''">{{ smtp.enabled ? '已启用' : '未启用' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="saveSMTP">
          <label class="rf-switch-row"><input v-model="smtp.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>启用 SMTP</span></label>
          <label class="rf-field-label"><span>SMTP 主机</span><input v-model="smtp.host" class="rf-control" placeholder="smtp.example.com" /></label>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>端口</span><input v-model.number="smtp.port" class="rf-control" type="number" min="1" max="65535" /></label><label class="rf-field-label"><span>加密</span><select v-model="smtp.tls_mode" class="rf-control"><option value="starttls">STARTTLS</option><option value="tls">SSL/TLS</option><option value="none">无加密</option></select></label></div>
          <label class="rf-field-label"><span>用户名</span><input v-model="smtp.username" class="rf-control" autocomplete="username" /></label>
          <label class="rf-field-label"><span>{{ smtp.has_password ? '密码（留空保持不变）' : '密码' }}</span><input v-model="smtp.password" class="rf-control" type="password" autocomplete="new-password" :placeholder="smtp.masked_password || 'SMTP 密码'" /></label>
          <label class="rf-field-label"><span>发件邮箱</span><input v-model="smtp.from_email" class="rf-control" type="email" /></label>
          <label class="rf-field-label"><span>发件人名称</span><input v-model="smtp.from_name" class="rf-control" maxlength="120" /></label>
          <div class="rf-form-actions"><nut-button type="primary" :loading="savingSMTP" @click="saveSMTP">保存 SMTP</nut-button></div>
          <div class="rf-test-row"><input v-model="testRecipient" class="rf-control" type="email" placeholder="测试收件邮箱" /><nut-button :loading="sendingTest" :disabled="!smtp.enabled" @click="sendTest">发送测试</nut-button></div>
        </form>
      </section>

      <section class="setting-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="success" size="18" />人机验证</h3><p>预留 GT4、GT3 和阿里云配置；当前生产适配器只支持极验 GT4。</p></div><span class="rf-status-chip" :class="captcha.enabled ? 'is-on' : ''">{{ captcha.enabled ? '已启用' : '未启用' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="saveCaptcha">
          <label class="rf-switch-row"><input v-model="captcha.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>启用人机验证</span></label>
          <label class="rf-field-label"><span>服务商</span><select v-model="captcha.provider" class="rf-control"><option value="gt4">极验 GT4</option><option value="gt3">极验 GT3（预留）</option><option value="aliyun">阿里云验证码（预留）</option></select></label>
          <label class="rf-field-label"><span>Captcha ID / App Key</span><input v-model="captcha.site_key" class="rf-control" /></label>
          <label class="rf-field-label"><span>服务端校验地址</span><input v-model="captcha.endpoint" class="rf-control" type="url" placeholder="https://gcaptcha4.geetest.com/validate" /></label>
          <label class="rf-field-label"><span>{{ captcha.has_secret ? 'Captcha Key（留空保持不变）' : 'Captcha Key' }}</span><input v-model="captcha.secret" class="rf-control" type="password" :placeholder="captcha.masked_secret || '服务端密钥'" /></label>
          <div class="rf-inline-note rf-inline-note--info"><AppIcon name="notice" size="16" />当前状态：{{ adapterLabel(captcha.adapter_status) }}</div>
          <div class="rf-form-actions"><nut-button type="primary" :loading="savingCaptcha" @click="saveCaptcha">保存验证码设置</nut-button></div>
        </form>
      </section>

      <section class="setting-card wide-card">
        <header class="rf-panel-heading"><div><h3><AppIcon name="link" size="18" />OAuth / OIDC 登录</h3><p>接入支持 Authorization Code 的第三方身份提供商，登录使用 state 与 PKCE S256。</p></div><span class="rf-status-chip" :class="oauth.enabled ? 'is-on' : ''">{{ oauth.enabled ? '已启用' : '未启用' }}</span></header>
        <form class="rf-editor-form" @submit.prevent="saveOAuth">
          <label class="rf-switch-row"><input v-model="oauth.enabled" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>启用 OAuth 登录</span></label>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>提供商标识</span><input v-model="oauth.provider_key" class="rf-control" maxlength="64" placeholder="例如 github 或 oidc" /><small>保存后请保持不变，用于识别已绑定账号。</small></label><label class="rf-field-label"><span>登录按钮名称</span><input v-model="oauth.provider_name" class="rf-control" maxlength="80" placeholder="例如 GitHub" /></label></div>
          <label class="rf-field-label"><span>Client ID</span><input v-model="oauth.client_id" class="rf-control" autocomplete="off" /></label>
          <label class="rf-field-label"><span>{{ oauth.has_client_secret ? 'Client Secret（留空保持不变）' : 'Client Secret（公开客户端可留空）' }}</span><input v-model="oauth.client_secret" class="rf-control" type="password" autocomplete="new-password" :placeholder="oauth.masked_client_secret || 'Client Secret'" /></label>
          <label v-if="oauth.has_client_secret" class="rf-switch-row"><input v-model="oauth.clear_client_secret" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>清除已保存的 Client Secret</span></label>
          <label class="rf-field-label"><span>Authorization URL</span><input v-model="oauth.authorization_url" class="rf-control" type="url" placeholder="https://provider.example.com/oauth/authorize" /></label>
          <label class="rf-field-label"><span>Token URL</span><input v-model="oauth.token_url" class="rf-control" type="url" placeholder="https://provider.example.com/oauth/token" /></label>
          <label class="rf-field-label"><span>UserInfo URL</span><input v-model="oauth.userinfo_url" class="rf-control" type="url" placeholder="https://provider.example.com/oauth/userinfo" /></label>
          <div class="rf-form-grid rf-form-grid--two"><label class="rf-field-label"><span>Scopes</span><input v-model="oauth.scopes" class="rf-control" placeholder="openid email profile" /></label><label class="rf-field-label"><span>Token 鉴权方式</span><select v-model="oauth.token_auth_method" class="rf-control"><option value="client_secret_post">client_secret_post</option><option value="client_secret_basic">client_secret_basic</option></select></label></div>
          <label class="rf-switch-row"><input v-model="oauth.require_verified_email" type="checkbox" /><span class="rf-toggle" aria-hidden="true" /><span>只接受提供商明确验证过的邮箱</span></label>
          <div class="rf-inline-note rf-inline-note--info"><AppIcon name="notice" size="16" />回调地址：{{ (site.public_url || 'https://你的域名').replace(/\/$/, '') }}/api/v1/oauth/callback</div>
          <div class="rf-form-actions"><nut-button type="primary" :loading="savingOAuth" @click="saveOAuth">保存 OAuth 设置</nut-button></div>
        </form>
      </section>

      <PaymentSettingsCard />
    </div>
  </PageContainer>
</template>
