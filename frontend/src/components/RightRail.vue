<script setup lang="ts">
import { useRouter } from 'vue-router'
import AppIcon from './AppIcon.vue'
const router = useRouter()
const trends = ['Roblox 中文玩家', '游戏攻略分享', '创作者资源', '组队招募', '社区活动']
</script>

<template>
  <aside class="rf-right-rail" aria-label="发现">
    <button type="button" class="rf-search-pill" @click="router.push('/')"><AppIcon name="search" size="18" /><span>搜索社区</span></button>
    <section class="rf-rail-panel"><h2>正在发生</h2><button v-for="(trend, index) in trends" :key="trend" type="button" class="rf-trend" @click="router.push('/users')"><span>热门 · {{ index + 1 }}</span><strong>#{{ trend }}</strong><small>查看相关内容</small></button><button type="button" class="rf-show-more" @click="router.push('/users')">查看更多</button></section>
    <section class="rf-rail-note"><strong>RobForum</strong><p>面向 Roblox 中文玩家的独立社区。</p><div><RouterLink to="/">关于</RouterLink><span>·</span><RouterLink to="/resources">资源</RouterLink><span>·</span><span>隐私</span></div></section>
  </aside>
</template>
