import path from 'node:path'
import { chromium } from 'playwright'

const baseURL = process.env.QA_BASE_URL || 'http://127.0.0.1:4173'
const evidenceDir = path.resolve('.omo/evidence/login-policy')
const viewports = [
  { name: 'mobile-375', width: 375, height: 812 },
  { name: 'tablet-768', width: 768, height: 900 },
  { name: 'desktop-1280', width: 1280, height: 900 },
]
const settings = {
  site_name: '罗布玩家社区',
  site_description: 'Roblox 中国玩家的独立交流社区',
  logo_url: '',
  avatar_url: '',
  verification_badge_url: '',
  primary_color: '#1d9bf0',
  public_url: 'https://community.example.com',
  user_agreement_url: '/legal/user-agreement',
  cookies_policy_url: 'https://example.com/legal/cookies',
  allow_register: true,
  require_email_verification: false,
  post_review_required: true,
  allowed_email_domains: [],
  banner_enabled: false,
  banner_text: '',
  banner_link: '',
  updated_at: '2026-07-25T00:00:00Z',
}
const admin = {
  id: 1,
  email: 'admin@example.com',
  display_name: '管理员',
  avatar_url: '',
  cover_url: '',
  bio: '',
  role: 'admin',
  status: 'active',
  blue_verified: false,
  member_active: false,
  roblox_verified: false,
  created_at: '2026-07-25T00:00:00Z',
}
const membership = { enabled: false, default_withdrawal_fee_bps: 300, default_service_fee_bps: 500, tiers: [], updated_at: '' }
const smtp = { enabled: false, host: '', port: 587, username: '', has_password: false, from_email: '', from_name: '', tls_mode: 'starttls' }
const captcha = { enabled: false, provider: 'gt4', site_key: '', endpoint: '', has_secret: false, adapter_status: 'disabled' }
const oauth = { enabled: false, provider_key: 'oidc', provider_name: 'OAuth', client_id: '', has_client_secret: false, authorization_url: '', token_url: '', userinfo_url: '', scopes: 'openid email profile', token_auth_method: 'client_secret_post', require_verified_email: true, updated_at: '' }
const payment = { enabled: false, kind: 'epay', gateway_url: '', merchant_id: '', has_secret: false, pay_type: 'alipay', notify_url: '', return_url: '' }

function responseFor(pathname, adminMode) {
  if (pathname === '/api/v1/site/settings' || pathname === '/api/v1/admin/site') return settings
  if (pathname === '/api/v1/me') return adminMode ? admin : null
  if (pathname === '/api/v1/membership/config' || pathname === '/api/v1/admin/membership') return membership
  if (pathname === '/api/v1/oauth/config') return { enabled: false, provider_name: 'OAuth' }
  if (pathname === '/api/v1/admin/smtp') return smtp
  if (pathname === '/api/v1/admin/captcha') return captcha
  if (pathname === '/api/v1/admin/oauth') return oauth
  if (pathname === '/api/v1/admin/payment') return payment
  return {}
}

const browser = await chromium.launch({ channel: 'chrome', headless: true })
const results = []
try {
  for (const viewport of viewports) {
    for (const surface of ['login', 'admin-settings']) {
      const adminMode = surface === 'admin-settings'
      const context = await browser.newContext({
        viewport: { width: viewport.width, height: viewport.height },
        colorScheme: 'light',
      })
      const page = await context.newPage()
      const consoleErrors = []
      page.on('console', (message) => {
        if (message.type() === 'error') consoleErrors.push(message.text())
      })
      page.on('pageerror', (error) => consoleErrors.push(error.message))
      await page.route('**/api/v1/**', async (route) => {
        const url = new URL(route.request().url())
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: responseFor(url.pathname, adminMode) }),
        })
      })
      const routePath = surface === 'login' ? 'login' : 'admin/settings'
      await page.goto(`${baseURL}/${routePath}`, { waitUntil: 'networkidle' })
      if (surface === 'login') {
        const consent = page.locator('.rf-login-consent')
        await consent.waitFor({ state: 'visible' })
        const userAgreement = consent.getByRole('link', { name: '用户协议' })
        const cookiesPolicy = consent.getByRole('link', { name: 'cookies政策' })
        const text = (await consent.innerText()).replace(/\s+/g, ' ').trim()
        if (text !== '登录即代表您同意我们的用户协议和cookies政策') throw new Error(`Unexpected consent text: ${text}`)
        if (await userAgreement.getAttribute('href') !== settings.user_agreement_url) throw new Error('User agreement href was not rendered from settings')
        if (await cookiesPolicy.getAttribute('href') !== settings.cookies_policy_url) throw new Error('Cookies href was not rendered from settings')
        const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
        if (horizontalOverflow) throw new Error(`Horizontal overflow on ${viewport.name}`)
        await page.screenshot({ path: path.join(evidenceDir, `login-${viewport.name}.png`) })
        if (viewport.width === 1280) {
          await userAgreement.focus()
          await page.screenshot({ path: path.join(evidenceDir, 'login-desktop-focus.png') })
          await cookiesPolicy.hover()
          await page.waitForTimeout(100)
          await page.screenshot({ path: path.join(evidenceDir, 'login-desktop-hover-mid.png') })
          await page.waitForTimeout(180)
          await page.screenshot({ path: path.join(evidenceDir, 'login-desktop-hover-settled.png') })
        }
        results.push({ surface, viewport: viewport.name, consent: text, links: [await userAgreement.getAttribute('href'), await cookiesPolicy.getAttribute('href')], consoleErrors })
      } else {
        const agreementInput = page.getByLabel('用户协议链接')
        const cookiesInput = page.getByLabel('Cookies 政策链接')
        await agreementInput.waitFor({ state: 'attached' })
        if (await agreementInput.inputValue() !== settings.user_agreement_url) throw new Error('Admin agreement input was not populated')
        if (await cookiesInput.inputValue() !== settings.cookies_policy_url) throw new Error('Admin cookies input was not populated')
        await agreementInput.scrollIntoViewIfNeeded()
        await page.waitForTimeout(200)
        await page.screenshot({ path: path.join(evidenceDir, `admin-settings-policy-${viewport.name}.png`) })
        results.push({ surface, viewport: viewport.name, values: [await agreementInput.inputValue(), await cookiesInput.inputValue()], consoleErrors })
      }
      if (consoleErrors.length) throw new Error(`${surface} ${viewport.name} console errors: ${consoleErrors.join(' | ')}`)
      await context.close()
    }
  }
  process.stdout.write(JSON.stringify(results, null, 2))
} finally {
  await browser.close()
}
