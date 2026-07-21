<script setup lang="ts">
defineProps<{ title?: string }>()
</script>

<template>
  <section class="rf-page">
    <header v-if="title || $slots.header || $slots.extra" class="rf-page-header">
      <slot name="header">
        <div class="rf-page-heading"><h1>{{ title }}</h1></div>
        <div v-if="$slots.extra" class="rf-page-extra"><slot name="extra" /></div>
      </slot>
    </header>
    <div class="rf-page-body"><slot /></div>
  </section>
</template>
