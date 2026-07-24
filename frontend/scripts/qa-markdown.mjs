import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'

const baseURL = 'http://127.0.0.1:4173'
const evidenceDir = '.qa-markdown-evidence'
await mkdir(evidenceDir, { recursive: true })

const markdown = `## Markdown 标题

这里有 **加粗内容**、*斜体内容* 和 \`inline code\`。

- 列表第一项
- 列表第二项

> 安全引用内容

[安全链接](https://example.com)

<script>window.__markdownXss = true</script>
<p onmouseover="window.__markdownXss = true">安全清理段落</p>`

const user = {
  id: 7, email: 'qa@example.com', display_name: '测试用户', avatar_url: '', cover_url: '',
  bio: '', role: 'user', status: 'active', blue_verified: false, member_active: false,
  roblox_verified: false, created_at: '2026-07-01T00:00:00Z',
}
const post = {
  id: 1, board_id: 2, board_name: '交流大厅', author_id: 7, author_name: '测试用户',
  author_avatar: '', author_verified: false, author_member: false, title: 'Markdown 功能验证',
  content: markdown, status: 'published', pinned: false, featured: false, views: 28,
  comment_count: 0, like_count: 1, repost_count: 0, liked: false, bookmarked: false,
  reposted: false, tags: ['markdown'], media: [], created_at: '2026-07-25T08:00:00Z',
  updated_at: '2026-07-25T08:00:00Z',
}

function payload(data) {
  return JSON.stringify({ data })
}

async function mockAPI(page) {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname.replace('/api/v1', '')
    let data = []
    if (path === '/site/settings') data = { site_name: 'RobForum', site_description: '', logo_url: '', avatar_url: '', verification_badge_url: '', primary_color: '#1d9bf0', public_url: baseURL, allow_register: true, require_email_verification: false, post_review_required: false, allowed_email_domains: [], banner_enabled: false, banner_text: '', banner_link: '', updated_at: '2026-07-25T00:00:00Z' }
    else if (path === '/membership/config') data = { enabled: false, default_withdrawal_fee_bps: 0, default_service_fee_bps: 0, tiers: [] }
    else if (path === '/me') data = user
    else if (path === '/me/notifications/unread-count') data = { count: 0 }
    else if (path === '/boards') data = [{ id: 2, slug: 'general', name: '交流大厅', description: '', icon: '', post_count: 1 }]
    else if (path === '/ads' || path === '/notices') data = []
    else if (path === '/posts/1/comments') data = []
    else if (path === '/posts/1/like') data = { liked: false, like_count: 1 }
    else if (path === '/posts/1/bookmark') data = { bookmarked: false }
    else if (path === '/posts/1/repost') data = { reposted: false, repost_count: 0 }
    else if (path === '/posts/1') data = post
    else if (path === '/posts') data = { items: [post], next_offset: 1, has_more: false }
    await route.fulfill({ status: 200, contentType: 'application/json', body: payload(data) })
  })
}

async function assertRendered(page, scope) {
  const root = page.locator(scope)
  await root.getByRole('heading', { name: 'Markdown 标题' }).waitFor()
  if (await root.locator('strong').filter({ hasText: '加粗内容' }).count() !== 1) throw new Error(`${scope}: bold Markdown missing`)
  if (await root.locator('li').count() !== 2) throw new Error(`${scope}: list Markdown missing`)
  if (await root.locator('code').filter({ hasText: 'inline code' }).count() !== 1) throw new Error(`${scope}: inline code missing`)
  const link = root.getByRole('link', { name: '安全链接' })
  if (await link.getAttribute('target') !== '_blank') throw new Error(`${scope}: safe external link target missing`)
  if (!(await link.getAttribute('rel'))?.includes('noopener')) throw new Error(`${scope}: safe external link rel missing`)
  if (await root.locator('script').count()) throw new Error(`${scope}: script was not sanitized`)
  if (await root.locator('[onmouseover]').count()) throw new Error(`${scope}: event attribute was not sanitized`)
  if (await page.evaluate(() => window.__markdownXss === true)) throw new Error(`${scope}: unsafe HTML executed`)
}

async function verifyComposer(browser, name, viewport) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  await mockAPI(page)
  await page.goto(`${baseURL}/posts/new`, { waitUntil: 'networkidle' })
  await page.getByLabel('选择板块').selectOption('2')
  const editor = page.getByLabel(/帖子正文/)
  await editor.fill(markdown)
  await editor.evaluate((element) => { element.scrollTop = 0 })
  await page.screenshot({ path: `${evidenceDir}/${name}-composer-edit.png` })
  const editTab = page.getByRole('tab', { name: '编辑' })
  const previewTab = page.getByRole('tab', { name: '预览' })
  await editTab.focus()
  await editTab.press('ArrowRight')
  if (await previewTab.getAttribute('aria-selected') !== 'true') throw new Error(`${name}: preview tab did not activate from keyboard`)
  if (!await previewTab.evaluate((element) => element === document.activeElement)) throw new Error(`${name}: preview tab did not receive keyboard focus`)
  await assertRendered(page, '.rf-markdown-preview')
  await page.screenshot({ path: `${evidenceDir}/${name}-composer-preview.png` })
  await previewTab.press('ArrowLeft')
  if (await editTab.getAttribute('aria-selected') !== 'true') throw new Error(`${name}: edit tab did not reactivate from keyboard`)
  if (await editor.inputValue() !== markdown) throw new Error(`${name}: editor lost Markdown source`)
  await context.close()
}

async function verifyPublished(browser, name, viewport) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  await mockAPI(page)
  await page.goto(`${baseURL}/posts/1`, { waitUntil: 'networkidle' })
  await assertRendered(page, '.rf-x-status-content')
  await page.screenshot({ path: `${evidenceDir}/${name}-post.png` })
  await page.goto(baseURL, { waitUntil: 'networkidle' })
  const feed = page.locator('.rf-feed-post-copy .rf-markdown')
  await feed.waitFor()
  if ((await feed.textContent())?.includes('**')) throw new Error(`${name}: feed exposed raw Markdown syntax`)
  await page.screenshot({ path: `${evidenceDir}/${name}-feed.png` })
  await context.close()
}

const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  await verifyComposer(browser, 'desktop', { width: 1280, height: 900 })
  await verifyPublished(browser, 'desktop', { width: 1280, height: 900 })
  await verifyComposer(browser, 'tablet', { width: 768, height: 900 })
  await verifyPublished(browser, 'tablet', { width: 768, height: 900 })
  await verifyComposer(browser, 'mobile', { width: 375, height: 812 })
  await verifyPublished(browser, 'mobile', { width: 375, height: 812 })
  console.log('PASS Markdown composer, rendering, and sanitization on desktop/tablet/mobile')
} finally {
  await browser.close()
}
