<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown } from '@/lib/markdown'

const props = withDefaults(defineProps<{
  source: string
  compact?: boolean
}>(), {
  compact: false,
})

const html = computed(() => renderMarkdown(props.source, props.compact))
</script>

<template>
  <div class="rf-markdown" :class="{ compact }" v-html="html" />
</template>

<style scoped>
.rf-markdown { min-width: 0; color: inherit; font: inherit; line-break: strict; overflow-wrap: break-word; word-break: auto-phrase; }
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
.rf-markdown :deep(pre) { max-width: 100%; margin: 0 0 0.9em; overflow-x: auto; padding: 12px 14px; border: 1px solid var(--rf-line); border-radius: var(--rf-radius); background: var(--rf-bg-subtle); }
.rf-markdown :deep(pre code) { padding: 0; background: transparent; font-size: 0.86em; white-space: pre; }
.rf-markdown :deep(hr) { margin: 1em 0; border: 0; border-top: 1px solid var(--rf-line); }
.rf-markdown :deep(table) { display: block; width: 100%; margin: 0 0 0.9em; overflow-x: auto; border-collapse: collapse; }
.rf-markdown :deep(th), .rf-markdown :deep(td) { padding: 7px 10px; border: 1px solid var(--rf-line); text-align: left; }
.rf-markdown :deep(th) { background: var(--rf-bg-subtle); font-weight: 700; }
.rf-markdown :deep(img) { display: block; max-width: 100%; max-height: 520px; margin: 0.8em 0; border-radius: var(--rf-radius); object-fit: contain; }
.rf-markdown.compact { max-height: 4.7em; overflow: hidden; line-height: 1.55; }
.rf-markdown.compact :deep(p), .rf-markdown.compact :deep(h1), .rf-markdown.compact :deep(h2), .rf-markdown.compact :deep(h3), .rf-markdown.compact :deep(h4), .rf-markdown.compact :deep(h5), .rf-markdown.compact :deep(h6), .rf-markdown.compact :deep(ul), .rf-markdown.compact :deep(ol), .rf-markdown.compact :deep(blockquote), .rf-markdown.compact :deep(pre), .rf-markdown.compact :deep(table) { margin: 0; font: inherit; line-height: inherit; }
.rf-markdown.compact :deep(ul), .rf-markdown.compact :deep(ol) { padding-left: 1.25em; }
.rf-markdown.compact :deep(pre) { padding: 0; border: 0; background: transparent; white-space: pre-wrap; }
</style>
