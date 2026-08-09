<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNotificationsStore } from '@/stores/notifications'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{ selectedKey: string; compact?: boolean }>(), { compact: false })
const emit = defineEmits<{ select: [] }>()
const auth = useAuthStore()
const notifications = useNotificationsStore()
const router = useRouter()
const sections = computed(() => [
  { label: '', items: [
    { key: '/', label: '首页', icon: 'home' },
    { key: '/boards/guides', label: '探索', icon: 'category' },
    { key: '/resources', label: '资源', icon: 'shop' },
    { key: '/users', label: '用户', icon: 'people' },
    ...(auth.user ? [
      { key: '/notifications', label: '通知', icon: 'notice' },
      { key: '/messages', label: '消息', icon: 'message' },
    ] : []),
    { key: '/boards/team-up', label: '组队', icon: 'invite' },
  ]},
  ...(auth.user ? [{ label: '我的空间', items: [
    { key: `/users/${auth.user.id}`, label: '个人主页', icon: 'user' },
    { key: '/bookmarks', label: '收藏', icon: 'star' },
    { key: '/settings/profile', label: '资料设置', icon: 'settings' },
    { key: '/check-in', label: '签到与资历', icon: 'calendar' },
    { key: '/points', label: '积分中心', icon: 'badge' },
    { key: '/points/store', label: '积分商城', icon: 'shop' },
    { key: '/lottery', label: '抽奖活动', icon: 'announcement' },
    { key: '/membership', label: '会员中心', icon: 'crown' },
    { key: '/wallet', label: '钱包', icon: 'wallet' },
    { key: '/avatar-frames', label: '头像框市场', icon: 'shop' },
    ...(!auth.user.blue_verified ? [{ key: '/verification/apply', label: '蓝微认证', icon: 'check' }] : []),
  ]}] : []),
  ...(auth.isAdmin ? [{ label: '管理', items: [
    { key: '/admin/settings', label: '系统设置', icon: 'settings' },
    { key: '/admin/ads', label: '广告管理', icon: 'image' },
    { key: '/admin/notices', label: '公告管理', icon: 'announcement' },
    { key: '/admin/resources', label: '资源审核', icon: 'review' },
    { key: '/admin/payouts', label: '提现审核', icon: 'payout' },
    { key: '/admin/wallet', label: '钱包与兑换码', icon: 'code' },
    { key: '/admin/verifications', label: '认证审核', icon: 'badge' },
    { key: '/admin/users', label: '用户管理', icon: 'members' },
    { key: '/admin/posts', label: '内容审核', icon: 'document' },
    { key: '/admin/avatar-frames', label: '头像框管理', icon: 'badge' },
    { key: '/admin/points', label: '积分与活动', icon: 'shop' },
  ]}] : []),
])

async function navigate(key: string) { emit('select'); await router.push(key) }
</script>

<template>
  <nav class="rf-nav" :class="{ 'rf-nav-compact': props.compact }" aria-label="主导航">
    <div v-for="section in sections" :key="section.label || 'main'" class="rf-nav-section">
      <div v-if="section.label && !props.compact" class="rf-nav-label">{{ section.label }}</div>
      <button v-for="item in section.items" :key="item.key" type="button" class="rf-nav-item" :class="{ active: selectedKey === item.key }" :aria-label="item.label" :aria-current="selectedKey === item.key ? 'page' : undefined" @click="navigate(item.key)">
        <span class="rf-nav-icon"><nut-badge v-if="item.key === '/notifications' && notifications.unread" :value="notifications.unread > 99 ? '99+' : notifications.unread"><AppIcon :name="item.icon" size="22" /></nut-badge><AppIcon v-else :name="item.icon" size="22" /></span><span v-if="!props.compact" class="rf-nav-text">{{ item.label }}</span>
      </button>
    </div>
    <nut-button
      v-if="auth.user"
      class="rf-sidebar-compose"
      type="primary"
      @click="navigate('/posts/new')"
    >
      <AppIcon name="pen" size="18" />
      <span v-if="!props.compact">发布</span>
    </nut-button>
    <nut-button
      v-else
      class="rf-sidebar-compose rf-sidebar-signin"
      plain
      @click="navigate('/login')"
    >
      <span v-if="!props.compact">登录</span>
      <AppIcon v-else name="user" size="18" />
    </nut-button>
  </nav>
</template>
