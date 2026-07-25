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

行内公式：$E = mc^2$。

$$
\\int_0^1 x^2 \\, dx = \\frac{1}{3}
$$

\`\`\`mermaid
flowchart LR
  markdown[Markdown] --> render[安全渲染]
\`\`\`

\`\`\`javascript
const greeting = "hello"
console.log(greeting)
\`\`\`

\`\`\`python
def greet(name: str) -> str:
    return f"Hello, {name}"
\`\`\`

\`\`\`go
func main() {
    fmt.Println("hello")
}
\`\`\`

\`\`\`rust
fn main() {
    println!("hello");
}
\`\`\`

\`\`\`made-up-language
plain fallback <stays safe>
\`\`\`

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
  await root.locator('p .katex').waitFor({ timeout: 10_000 })
  if (await root.locator('p .katex').count() !== 1) throw new Error(`${scope}: inline LaTeX was not rendered`)
  if (await root.locator('.katex-display > .katex').count() !== 1) throw new Error(`${scope}: block LaTeX was not rendered`)
  if (await root.locator('.katex-mathml math').count() !== 2) throw new Error(`${scope}: accessible LaTeX MathML missing`)
  const diagram = root.locator('.rf-mermaid[role="img"]')
  await diagram.locator('svg').waitFor({ timeout: 10_000 })
  if (await diagram.getAttribute('aria-label') !== 'Mermaid 图表') throw new Error(`${scope}: Mermaid accessible label missing`)
  if (await root.locator('pre[data-language="mermaid"]').count()) throw new Error(`${scope}: Mermaid source block was not replaced`)
  if (await diagram.locator('script, foreignObject, [onload], a[href^="javascript:"]').count()) throw new Error(`${scope}: Mermaid output was not sanitized`)
  for (const language of ['javascript', 'python', 'go', 'rust']) {
    const code = root.locator(`pre[data-language="${language}"] > code.hljs.language-${language}`)
    if (await code.count() !== 1) throw new Error(`${scope}: ${language} code block was not highlighted`)
    if (await code.locator('span').count() === 0) throw new Error(`${scope}: ${language} highlighting emitted no syntax tokens`)
  }
  const fallback = root.locator('pre[data-language="made-up-language"] > code')
  if (await fallback.count() !== 1) throw new Error(`${scope}: unknown language fallback missing`)
  if ((await fallback.textContent())?.trim() !== 'plain fallback <stays safe>') throw new Error(`${scope}: unknown language fallback changed source text`)
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
  await page.screenshot({ path: `${evidenceDir}/${name}-composer-preview.png`, fullPage: true })
  await previewTab.press('ArrowLeft')
  if (await editTab.getAttribute('aria-selected') !== 'true') throw new Error(`${name}: edit tab did not reactivate from keyboard`)
  if (await editor.inputValue() !== markdown) throw new Error(`${name}: editor lost Markdown source`)
  await context.close()
}

async function verifyPublished(browser, name, viewport, colorScheme = 'light') {
  const context = await browser.newContext({ viewport, colorScheme })
  const page = await context.newPage()
  await mockAPI(page)
  await page.goto(`${baseURL}/posts/1`, { waitUntil: 'networkidle' })
  await assertRendered(page, '.rf-x-status-content')
  await page.screenshot({ path: `${evidenceDir}/${name}-post.png`, fullPage: true })
  await page.goto(baseURL, { waitUntil: 'networkidle' })
  const feed = page.locator('.rf-feed-post-copy .rf-markdown')
  await feed.waitFor()
  const feedText = await feed.textContent() || ''
  if (feedText.includes('**')) throw new Error(`${name}: feed exposed raw Markdown syntax`)
  if (feedText.includes('flowchart')) throw new Error(`${name}: feed exposed raw Mermaid source`)
  if ((feedText.match(/E = mc\^2/g) || []).length !== 1) throw new Error(`${name}: feed did not preserve one plain-text formula`)
  await page.screenshot({ path: `${evidenceDir}/${name}-feed.png`, fullPage: true })
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
  await verifyPublished(browser, 'desktop-dark', { width: 1280, height: 900 }, 'dark')
  await verifyPublished(browser, 'mobile-dark', { width: 375, height: 812 }, 'dark')
  console.log('PASS Markdown composer, rendering, and sanitization on desktop/tablet/mobile')
} finally {
  await browser.close()
}
