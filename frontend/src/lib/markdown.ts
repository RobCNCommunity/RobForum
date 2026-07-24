import DOMPurify from 'dompurify'
import { marked } from 'marked'

const allowedTags = [
  'a', 'blockquote', 'br', 'code', 'del', 'em', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'hr', 'img', 'li', 'ol', 'p', 'pre', 'strong', 'table', 'tbody', 'td', 'th', 'thead',
  'tr', 'ul',
] as const

export function renderMarkdown(source: string, compact = false): string {
  if (!source.trim()) return ''

  const parsed = marked.parse(source, {
    async: false,
    breaks: true,
    gfm: true,
  })
  const sanitized = DOMPurify.sanitize(parsed, {
    ALLOWED_ATTR: ['alt', 'href', 'src', 'title'],
    ALLOWED_TAGS: [...allowedTags],
  })
  const template = document.createElement('template')
  template.innerHTML = sanitized

  for (const link of template.content.querySelectorAll('a')) {
    if (compact) {
      link.replaceWith(document.createTextNode(link.textContent || ''))
      continue
    }
    link.target = '_blank'
    link.rel = 'nofollow noopener noreferrer'
  }

  for (const image of template.content.querySelectorAll('img')) {
    if (compact) {
      image.replaceWith(document.createTextNode(image.alt ? `[图片：${image.alt}]` : '[图片]'))
      continue
    }
    image.loading = 'lazy'
    image.decoding = 'async'
  }

  return template.innerHTML
}

export function markdownToPlainText(source: string): string {
  const template = document.createElement('template')
  template.innerHTML = renderMarkdown(source, true)
  return (template.content.textContent || '').replace(/\s+/g, ' ').trim()
}
