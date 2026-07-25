import 'katex/dist/katex.min.css'
import DOMPurify from 'dompurify'
import katex from 'katex'

const mathTags = [
  'annotation', 'maction', 'math', 'menclose', 'merror', 'mfrac', 'mi', 'mmultiscripts',
  'mn', 'mo', 'mover', 'mpadded', 'mphantom', 'mprescripts', 'mroot', 'mrow', 'ms',
  'mspace', 'msqrt', 'mstyle', 'msub', 'msubsup', 'msup', 'mtable', 'mtd', 'mtext',
  'mtr', 'munder', 'munderover', 'semantics', 'span', 'svg', 'path',
] as const
const mathAttributes = [
  'aria-hidden', 'class', 'd', 'display', 'encoding', 'focusable', 'height',
  'preserveAspectRatio', 'style', 'title', 'viewBox', 'width', 'xmlns',
] as const

export function renderMath(root: HTMLElement): void {
  for (const placeholder of root.querySelectorAll<HTMLElement>('[data-math-display]')) {
    const displayMode = placeholder.dataset.mathDisplay === 'true'
    const markup = katex.renderToString(placeholder.textContent || '', {
      displayMode,
      maxExpand: 1_000,
      maxSize: 10,
      output: 'htmlAndMathml',
      strict: 'warn',
      throwOnError: false,
      trust: false,
    })
    const template = document.createElement('template')
    template.innerHTML = DOMPurify.sanitize(markup, {
      ALLOWED_ATTR: [...mathAttributes],
      ALLOWED_TAGS: [...mathTags],
    })
    const formula = template.content.firstElementChild
    if (!(formula instanceof HTMLElement)) continue
    if (displayMode) formula.tabIndex = 0
    placeholder.replaceWith(formula)
  }
}
