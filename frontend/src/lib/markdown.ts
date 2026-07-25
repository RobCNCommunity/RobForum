import DOMPurify from 'dompurify'
import { Marked, type MarkedExtension, type Tokens } from 'marked'
import { markdownMathExtension } from '@/lib/markdownMath'
import { highlightCodeBlocks, normalizeCodeLanguage } from '@/lib/syntaxHighlight'

const userHtmlTags = [
  'a', 'blockquote', 'br', 'code', 'del', 'em', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'hr', 'img', 'li', 'ol', 'p', 'pre', 'strong', 'table', 'tbody', 'td', 'th', 'thead',
  'tr', 'ul',
] as const

const userHtmlAttributes = ['alt', 'href', 'src', 'title'] as const
const renderedTags = [...userHtmlTags, 'span'] as const
const renderedAttributes = ['data-language', 'data-math-display', ...userHtmlAttributes] as const

const markdownExtension = {
  renderer: {
    code({ text, lang }: Tokens.Code): string {
      const code = document.createElement('code')
      code.textContent = text
      const language = normalizeCodeLanguage(lang)
      if (language) code.dataset.language = language
      return `<pre>${code.outerHTML}</pre>`
    },
    html({ text }: Tokens.HTML | Tokens.Tag): string {
      return DOMPurify.sanitize(text, {
        ALLOWED_ATTR: [...userHtmlAttributes],
        ALLOWED_TAGS: [...userHtmlTags],
      })
    },
  },
} satisfies MarkedExtension

const markdownParser = new Marked(markdownExtension, markdownMathExtension)

export function renderMarkdown(source: string, compact = false): string {
  if (!source.trim()) return ''

  const parsed = markdownParser.parse(source, {
    async: false,
    breaks: true,
    gfm: true,
  })
  const sanitized = DOMPurify.sanitize(parsed, {
    ALLOWED_ATTR: [...renderedAttributes],
    ALLOWED_TAGS: [...renderedTags],
  })
  const template = document.createElement('template')
  template.innerHTML = sanitized

  if (compact) {
    for (const formula of template.content.querySelectorAll('[data-math-display]')) {
      formula.replaceWith(document.createTextNode(formula.textContent || ''))
    }
    for (const diagram of template.content.querySelectorAll('pre > code[data-language="mermaid"]')) {
      diagram.parentElement?.replaceWith(document.createTextNode('[Mermaid 图表]'))
    }
  } else {
    highlightCodeBlocks(template.content)
  }

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
