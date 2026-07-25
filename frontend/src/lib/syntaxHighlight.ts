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
  for (const code of root.querySelectorAll<HTMLElement>('pre > code[data-language]')) {
    const specifiedLanguage = normalizeCodeLanguage(code.dataset.language)
    const container = code.parentElement
    if (!specifiedLanguage || !container) continue

    container.dataset.language = specifiedLanguage
    if (!highlight.getLanguage(specifiedLanguage)) continue

    const result = highlight.highlight(code.textContent || '', {
      language: specifiedLanguage,
      ignoreIllegals: true,
    })
    const canonicalLanguage = normalizeCodeLanguage(result.language) || specifiedLanguage
    code.innerHTML = result.value
    code.classList.add('hljs', `language-${canonicalLanguage}`)
  }
}
