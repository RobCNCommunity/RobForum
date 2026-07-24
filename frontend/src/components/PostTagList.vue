<script setup lang="ts">
withDefaults(defineProps<{
  tags?: readonly string[]
  compact?: boolean
}>(), {
  tags: () => [],
  compact: false,
})
</script>

<template>
  <span v-if="tags.length" class="rf-post-tags" :class="{ compact }" aria-label="帖子标签">
    <span v-for="tag in tags" :key="tag">#{{ tag }}</span>
  </span>
</template>

<style scoped>
.rf-post-tags { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 6px; }
.rf-post-tags > span { display: inline-flex; min-height: 26px; align-items: center; padding: 0 8px; border-radius: var(--rf-pill); color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, var(--rf-bg)); font-size: 12px; font-weight: 600; }
.rf-post-tags.compact { gap: 5px; }
.rf-post-tags.compact > span { min-height: 22px; padding-inline: 7px; font-size: 11px; }
</style>
