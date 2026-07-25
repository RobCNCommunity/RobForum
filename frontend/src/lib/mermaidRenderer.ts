import DOMPurify from 'dompurify'
import mermaid, { type MermaidConfig } from 'mermaid'
import type { ColorTheme } from '@/theme'

let diagramSequence = 0

function token(styles: CSSStyleDeclaration, name: string): string {
  return styles.getPropertyValue(name).trim()
}

function markDiagramError(block: HTMLElement): void {
  block.dataset.mermaidError = 'true'
  block.setAttribute('aria-label', 'Mermaid 图表语法错误，显示源代码')
}

export async function renderMermaidDiagrams(root: HTMLElement, theme: ColorTheme): Promise<void> {
  const styles = getComputedStyle(root)
  const background = token(styles, '--rf-bg')
  const surface = token(styles, '--rf-bg-subtle')
  const hover = token(styles, '--rf-bg-hover')
  const text = token(styles, '--rf-text')
  const muted = token(styles, '--rf-muted')
  const border = token(styles, '--rf-line')
  const primary = token(styles, '--primary')
  const config = {
    darkMode: theme === 'dark',
    fontFamily: styles.fontFamily,
    htmlLabels: false,
    logLevel: 'fatal',
    look: 'classic',
    maxEdges: 500,
    maxTextSize: 20_000,
    secure: [
      'secure', 'securityLevel', 'startOnLoad', 'maxTextSize', 'suppressErrorRendering',
      'maxEdges', 'theme', 'themeVariables', 'themeCSS', 'fontFamily', 'altFontFamily',
      'htmlLabels',
    ],
    securityLevel: 'strict',
    startOnLoad: false,
    suppressErrorRendering: true,
    theme: 'base',
    themeVariables: {
      actorBkg: surface,
      actorBorder: border,
      actorTextColor: text,
      background,
      clusterBkg: surface,
      clusterBorder: border,
      edgeLabelBackground: background,
      lineColor: muted,
      mainBkg: hover,
      nodeBorder: primary,
      noteBkgColor: surface,
      noteBorderColor: border,
      noteTextColor: text,
      primaryBorderColor: primary,
      primaryColor: hover,
      primaryTextColor: text,
      secondaryColor: surface,
      signalColor: text,
      signalTextColor: text,
      tertiaryColor: background,
      titleColor: text,
    },
  } satisfies MermaidConfig

  mermaid.initialize(config)

  for (const block of root.querySelectorAll<HTMLElement>('pre[data-language="mermaid"]')) {
    const source = block.querySelector('code')?.textContent || ''
    try {
      if (!await mermaid.parse(source, { suppressErrors: true })) {
        markDiagramError(block)
        continue
      }

      diagramSequence += 1
      const { svg } = await mermaid.render(`rf-mermaid-${diagramSequence}`, source)
      if (!block.isConnected || !root.contains(block)) continue

      const diagram = document.createElement('div')
      diagram.className = 'rf-mermaid'
      diagram.tabIndex = 0
      diagram.setAttribute('role', 'img')
      diagram.setAttribute('aria-label', 'Mermaid 图表')
      diagram.innerHTML = DOMPurify.sanitize(svg, {
        FORBID_ATTR: ['href', 'target', 'xlink:href'],
        FORBID_TAGS: ['foreignObject', 'script'],
        USE_PROFILES: { svg: true, svgFilters: true },
      })
      if (!diagram.querySelector('svg')) {
        markDiagramError(block)
        continue
      }
      block.replaceWith(diagram)
    } catch (error) {
      if (error instanceof Error) {
        markDiagramError(block)
        continue
      }
      throw error
    }
  }
}
