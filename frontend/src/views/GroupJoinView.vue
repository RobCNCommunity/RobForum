<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { errorMessage, joinConversationByInvite } from '@/api'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const joining = ref(true)
const error = ref('')
const token = computed(() => typeof route.query.token === 'string' ? route.query.token.trim() : '')

async function join() {
  if (!token.value) {
    error.value = '邀请链接无效'
    joining.value = false
    return
  }
  joining.value = true
  error.value = ''
  try {
    const conversation = await joinConversationByInvite(token.value)
    Notify.success('已加入群聊')
    await router.replace({ path: '/messages', query: { conversation: String(conversation.id) } })
  } catch (cause) {
    error.value = errorMessage(cause, '无法加入群聊')
  } finally {
    joining.value = false
  }
}

onMounted(join)
</script>

<template>
  <PageContainer>
    <section class="rf-group-join">
      <span class="rf-group-join-mark" :class="{ spinning: joining }"><AppIcon :name="error ? 'close' : 'people'" size="27" /></span>
      <h1>{{ joining ? '正在加入群聊' : error ? '无法加入群聊' : '已加入群聊' }}</h1>
      <p>{{ joining ? '正在验证邀请链接和群成员状态。' : error }}</p>
      <div v-if="!joining" class="rf-group-join-actions">
        <nut-button v-if="error" type="primary" @click="join">重新尝试</nut-button>
        <nut-button plain @click="router.push('/messages')">返回消息</nut-button>
      </div>
    </section>
  </PageContainer>
</template>

<style scoped>
.rf-group-join { display: flex; min-height: min(620px, calc(100vh - 120px)); flex-direction: column; align-items: center; justify-content: center; gap: 9px; padding: 36px 20px; text-align: center; }.rf-group-join-mark { display: grid; width: 58px; height: 58px; margin-bottom: 6px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, transparent); }.rf-group-join-mark.spinning :deep(svg) { animation: rf-join-spin 1s linear infinite; }.rf-group-join h1 { margin: 0; font-size: 23px; letter-spacing: -.02em; }.rf-group-join p { max-width: 34ch; margin: 0; color: var(--rf-muted); line-height: 1.55; }.rf-group-join-actions { display: flex; gap: 8px; margin-top: 10px; }@keyframes rf-join-spin { to { transform: rotate(360deg); } }
</style>
