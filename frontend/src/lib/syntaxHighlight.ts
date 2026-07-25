import highlight from 'highlight.js/lib/common'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import nginx from 'highlight.js/lib/languages/nginx'

highlight.registerLanguage('dockerfile', dockerfile)
highlight.registerAliases(['docker'], { languageName: 'dockerfile' })
highlight.registerLanguage('nginx', nginx)
highlight.registerAliases(['vue'], { languageName: 'xml' })

const languagePattern = /^[a-z0-9][a-z0-9_+#.-]{0,31}$/i

export function normalizeCodeLanguage(value?: string): string {
  const candidate = value?.trim().split(/\s+/, 1)[0]?.toLowerCase() || ''
  return languagePattern.test(candidate) ? candidate : ''
}

export function highlightCodeBlocks(root: DocumentFragment): void {
  for (const code of root.querySelectorAll<HTMLElement>('pre > code')) {
    const specifiedLanguage = normalizeCodeLanguage(code.dataset.language)
    const container = code.parentElement
    if (!container) continue
    if (specifiedLanguage === 'mermaid') {
      container.dataset.language = 'mermaid'
      continue
    }

    if (specifiedLanguage && !highlight.getLanguage(specifiedLanguage)) {
      container.dataset.language = specifiedLanguage
      code.classList.add('hljs', `language-${specifiedLanguage}`)
      continue
    }

    const result = specifiedLanguage
      ? highlight.highlight(code.textContent || '', { language: specifiedLanguage, ignoreIllegals: true })
      : highlight.highlightAuto(code.textContent || '')
    const canonicalLanguage = normalizeCodeLanguage(result.language) || specifiedLanguage || 'text'
    container.dataset.language = canonicalLanguage
    code.innerHTML = result.value
    code.classList.add('hljs', `language-${canonicalLanguage}`)
  }
}
