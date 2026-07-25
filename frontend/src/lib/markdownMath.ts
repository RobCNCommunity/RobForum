import type { MarkedExtension, Tokens } from 'marked'

const inlineRule = /^(\${1,2})(?!\$)((?:\\.|[^\\\n])*?(?:\\.|[^\\\n\$]))\1(?=[\s?!\.,:？！。，：]|$)/
const blockRule = /^(\${1,2})\n((?:\\[^]|[^\\])+?)\n\1(?:\n|$)/

type MathToken = Tokens.Generic & {
  readonly displayMode: boolean
  readonly text: string
  readonly type: 'blockMath' | 'inlineMath'
}

function isMathToken(token: Tokens.Generic): token is MathToken {
  return (token.type === 'blockMath' || token.type === 'inlineMath')
    && typeof token.displayMode === 'boolean'
    && typeof token.text === 'string'
}

function renderPlaceholder(token: Tokens.Generic): string | false {
  if (!isMathToken(token)) return false
  const placeholder = document.createElement('span')
  placeholder.dataset.mathDisplay = token.displayMode ? 'true' : 'false'
  placeholder.textContent = token.text
  return placeholder.outerHTML
}

export const markdownMathExtension = {
  extensions: [
    {
      name: 'inlineMath',
      level: 'inline',
      start(source) {
        const index = source.indexOf('$')
        return index === -1 ? undefined : index
      },
      tokenizer(source) {
        const match = source.match(inlineRule)
        const delimiter = match?.[1]
        const text = match?.[2]
        if (!delimiter || !text) return
        return {
          displayMode: delimiter.length === 2,
          raw: match[0],
          text: text.trim(),
          type: 'inlineMath',
        }
      },
      renderer: renderPlaceholder,
    },
    {
      name: 'blockMath',
      level: 'block',
      tokenizer(source) {
        const match = source.match(blockRule)
        const delimiter = match?.[1]
        const text = match?.[2]
        if (!delimiter || !text) return
        return {
          displayMode: delimiter.length === 2,
          raw: match[0],
          text: text.trim(),
          type: 'blockMath',
        }
      },
      renderer: renderPlaceholder,
    },
  ],
} satisfies MarkedExtension
