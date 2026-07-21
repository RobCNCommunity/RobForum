<script setup lang="ts">
import { computed } from 'vue'
import {
  Add, ArrowRight, Category, Check, Close, Date, Edit, Eye, Heart, Home, Horizontal, Left, Link, Message, MoreX, My,
  Notice, People, Photograph, Retweet, Search, Setting, Share, Shop, Star, Success, Github,
} from '@nutui/icons-vue'

const props = withDefaults(defineProps<{ name: string; size?: string | number; color?: string }>(), { size: 20, color: 'currentColor' })
const icons: Record<string, unknown> = { add: Add, arrow: ArrowRight, back: Left, calendar: Date, category: Category, check: Check, close: Close, edit: Edit, eye: Eye, heart: Heart, home: Home, link: Link, menu: Horizontal, message: Message, more: MoreX, user: My, notice: Notice, people: People, image: Photograph, repost: Retweet, search: Search, settings: Setting, share: Share, shop: Shop, star: Star, success: Success, github: Github }
const component = computed(() => icons[props.name] || More)
</script>

<template><component :is="component" :width="props.size" :height="props.size" :color="props.color" aria-hidden="true" /></template>
