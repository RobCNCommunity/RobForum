<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderMarkdown } from '@/lib/markdown'
import { useTheme } from '@/theme'

const props = withDefaults(defineProps<{
  source: string
  compact?: boolean
  imageReplacements?: Record<string, string>
}>(), {
  compact: false,
  imageReplacements: () => ({}),
})

const html = computed(() => {
  const rendered = renderMarkdown(props.source, props.compact)
  if (!rendered || !Object.keys(props.imageReplacements).length) return rendered
  const template = document.createElement('template')
  template.innerHTML = rendered
  for (const image of template.content.querySelectorAll('img')) {
    const replacement = props.imageReplacements[image.getAttribute('src') || '']
    if (replacement) image.src = replacement
  }
  return template.innerHTML
})
const root = ref<HTMLElement>()
const { theme } = useTheme()
let enhancementVersion = 0

async function enhanceContent(): Promise<void> {
  const version = ++enhancementVersion
  await nextTick()
  const container = root.value
  if (!container || props.compact) return

  const tasks: Promise<void>[] = []
  if (container.querySelector('[data-math-display]')) {
    tasks.push(import('@/lib/mathRenderer').then(({ renderMath }) => {
      if (version === enhancementVersion && container === root.value) renderMath(container)
    }))
  }
  if (container.querySelector('pre[data-language="mermaid"]')) {
    tasks.push(import('@/lib/mermaidRenderer').then(async ({ renderMermaidDiagrams }) => {
      if (version === enhancementVersion && container === root.value) {
        await renderMermaidDiagrams(container, theme.value)
      }
    }))
  }
  await Promise.all(tasks)
}

watch([html, theme], () => { void enhanceContent() }, { flush: 'post' })
onMounted(() => { void enhanceContent() })
onBeforeUnmount(() => { enhancementVersion += 1 })
</script>

<template>
  <div :key="theme" ref="root" class="rf-markdown" :class="{ compact }" v-html="html" />
</template>

<style scoped>
.rf-markdown { width: 100%; min-width: 0; max-width: 100%; color: inherit; font: inherit; line-break: strict; overflow-wrap: anywhere; word-break: break-word; }
.rf-markdown :deep(> :first-child) { margin-top: 0; }
.rf-markdown :deep(> :last-child) { margin-bottom: 0; }
.rf-markdown :deep(p) { margin: 0 0 0.8em; white-space: normal; }
.rf-markdown :deep(h1), .rf-markdown :deep(h2), .rf-markdown :deep(h3), .rf-markdown :deep(h4), .rf-markdown :deep(h5), .rf-markdown :deep(h6) { margin: 1.1em 0 0.5em; color: var(--rf-text); font-family: var(--rf-font-display); line-height: 1.35; letter-spacing: 0; }
.rf-markdown :deep(h1) { font-size: 1.45em; }
.rf-markdown :deep(h2) { font-size: 1.3em; }
.rf-markdown :deep(h3) { font-size: 1.16em; }
.rf-markdown :deep(h4), .rf-markdown :deep(h5), .rf-markdown :deep(h6) { font-size: 1em; }
.rf-markdown :deep(ul), .rf-markdown :deep(ol) { margin: 0 0 0.85em; padding-left: 1.55em; }
.rf-markdown :deep(li + li) { margin-top: 0.3em; }
.rf-markdown :deep(blockquote) { margin: 0 0 0.85em; padding: 0.15em 0 0.15em 0.9em; border-left: 3px solid var(--primary); color: var(--rf-muted); }
.rf-markdown :deep(a) { color: var(--primary); text-decoration: underline; text-decoration-thickness: 1px; text-underline-offset: 3px; }
.rf-markdown :deep(code) { padding: 0.12em 0.35em; border-radius: 4px; background: var(--rf-bg-subtle); font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 0.88em; }
.rf-markdown :deep(pre) { position: relative; max-width: 100%; margin: 0 0 0.9em; overflow-x: auto; padding: 12px 14px; border: 1px solid var(--rf-line); border-radius: var(--rf-radius); background: var(--rf-bg-subtle); }
.rf-markdown :deep(pre[data-language]) { padding-top: 32px; }
.rf-markdown :deep(pre[data-language]::before) { position: absolute; top: 10px; right: 12px; color: var(--rf-muted); content: attr(data-language); font-family: var(--rf-font); font-size: 11px; font-weight: 650; line-height: 1; pointer-events: none; }
.rf-markdown :deep(pre code) { padding: 0; background: transparent; font-size: 0.86em; white-space: pre; }
.rf-markdown :deep(.hljs-comment), .rf-markdown :deep(.hljs-quote) { color: var(--rf-code-comment); font-style: italic; }
.rf-markdown :deep(.hljs-keyword), .rf-markdown :deep(.hljs-selector-tag), .rf-markdown :deep(.hljs-literal), .rf-markdown :deep(.hljs-section), .rf-markdown :deep(.hljs-link) { color: var(--rf-code-keyword); }
.rf-markdown :deep(.hljs-string), .rf-markdown :deep(.hljs-doctag), .rf-markdown :deep(.hljs-regexp), .rf-markdown :deep(.hljs-template-tag), .rf-markdown :deep(.hljs-template-variable) { color: var(--rf-code-string); }
.rf-markdown :deep(.hljs-title), .rf-markdown :deep(.hljs-name), .rf-markdown :deep(.hljs-type), .rf-markdown :deep(.hljs-attribute) { color: var(--rf-code-title); }
.rf-markdown :deep(.hljs-number), .rf-markdown :deep(.hljs-symbol), .rf-markdown :deep(.hljs-bullet), .rf-markdown :deep(.hljs-built_in) { color: var(--rf-code-number); }
.rf-markdown :deep(.hljs-variable), .rf-markdown :deep(.hljs-params) { color: var(--rf-code-variable); }
.rf-markdown :deep(.hljs-tag), .rf-markdown :deep(.hljs-selector-id), .rf-markdown :deep(.hljs-selector-class) { color: var(--rf-code-tag); }
.rf-markdown :deep(.hljs-meta), .rf-markdown :deep(.hljs-property) { color: var(--rf-code-meta); }
.rf-markdown :deep(.hljs-addition) { color: var(--rf-success); background: color-mix(in srgb, var(--rf-success) 10%, transparent); }
.rf-markdown :deep(.hljs-deletion) { color: var(--rf-danger); background: color-mix(in srgb, var(--rf-danger) 10%, transparent); }
.rf-markdown :deep(.katex) { color: var(--rf-text); font-size: 1em; }
.rf-markdown :deep(.katex-display) { max-width: 100%; margin: 0 0 0.9em; overflow-x: auto; overflow-y: hidden; padding: 0.65em 0; }
.rf-markdown :deep(.katex-display:focus-visible), .rf-markdown :deep(.rf-mermaid:focus-visible) { outline: 2px solid var(--primary); outline-offset: 2px; }
.rf-markdown :deep(.rf-mermaid) { max-width: 100%; margin: 0 0 0.9em; overflow-x: auto; overscroll-behavior-inline: contain; padding: 12px; border: 1px solid var(--rf-line); border-radius: var(--rf-radius); background: var(--rf-bg-subtle); }
.rf-markdown :deep(.rf-mermaid svg) { display: block; min-width: min(520px, 100%); max-width: none !important; height: auto; margin: 0 auto; }
.rf-markdown :deep(hr) { margin: 1em 0; border: 0; border-top: 1px solid var(--rf-line); }
.rf-markdown :deep(table) { display: block; width: 100%; margin: 0 0 0.9em; overflow-x: auto; border-collapse: collapse; }
.rf-markdown :deep(th), .rf-markdown :deep(td) { padding: 7px 10px; border: 1px solid var(--rf-line); text-align: left; }
.rf-markdown :deep(th) { background: var(--rf-bg-subtle); font-weight: 700; }
.rf-markdown :deep(img) { display: block; max-width: 100%; max-height: 520px; margin: 0.8em 0; border-radius: var(--rf-radius); object-fit: contain; }
.rf-markdown.compact { max-height: 4.7em; overflow: hidden; line-height: 1.55; }
.rf-markdown.compact :deep(p), .rf-markdown.compact :deep(h1), .rf-markdown.compact :deep(h2), .rf-markdown.compact :deep(h3), .rf-markdown.compact :deep(h4), .rf-markdown.compact :deep(h5), .rf-markdown.compact :deep(h6), .rf-markdown.compact :deep(ul), .rf-markdown.compact :deep(ol), .rf-markdown.compact :deep(blockquote), .rf-markdown.compact :deep(pre), .rf-markdown.compact :deep(table) { margin: 0; font: inherit; line-height: inherit; }
.rf-markdown.compact :deep(ul), .rf-markdown.compact :deep(ol) { padding-left: 1.25em; }
.rf-markdown.compact :deep(pre) { padding: 0; border: 0; background: transparent; white-space: pre-wrap; }
.rf-markdown.compact :deep(pre[data-language]::before) { content: none; }
</style>
