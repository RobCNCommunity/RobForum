<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import {
  createConversation,
  errorMessage,
  fetchConversations,
  fetchMessages,
  searchUsers,
  sendMessage,
  type ChatMessage,
  type Conversation,
  type UserSearchResult,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const conversations = ref<Conversation[]>([])
const selectedID = ref(0)
const messages = ref<ChatMessage[]>([])
const draft = ref('')
const loading = ref(true)
const messagesLoading = ref(false)
const sending = ref(false)
const groupOpen = ref(false)
const groupName = ref('')
const memberQuery = ref('')
const memberCandidates = ref<UserSearchResult[]>([])
const selectedMemberIDs = ref<number[]>([])
const messageList = ref<HTMLElement | null>(null)

const selected = computed(() => conversations.value.find((item) => item.id === selectedID.value) || null)
const selectedTitle = computed(() => {
  if (!selected.value) return '消息'
  if (selected.value.kind === 'group') return selected.value.name || '群聊'
  return selected.value.members.find((member) => member.id !== auth.user?.id)?.display_name || '私信'
})

async function scrollToLatest() {
  await nextTick()
  if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
}

async function loadConversations() {
  loading.value = true
  try {
    conversations.value = await fetchConversations()
    const requestedID = Number(route.query.conversation || 0)
    selectedID.value = conversations.value.some((item) => item.id === requestedID) ? requestedID : 0
    if (selectedID.value) await loadMessages(selectedID.value)
    else messages.value = []
  } catch (e) {
    Notify.danger(errorMessage(e, '会话加载失败'))
  } finally {
    loading.value = false
  }
}

async function loadMessages(id: number) {
  selectedID.value = id
  messagesLoading.value = true
  try {
    messages.value = await fetchMessages(id)
    await scrollToLatest()
  } catch (e) {
    Notify.danger(errorMessage(e, '消息加载失败'))
  } finally {
    messagesLoading.value = false
  }
}

async function openConversation(id: number) {
  if (selectedID.value !== id) await loadMessages(id)
  await router.replace({ path: '/messages', query: { conversation: String(id) } })
}

async function closeConversation() {
  selectedID.value = 0
  messages.value = []
  await router.replace('/messages')
}

async function submitMessage() {
  if (!selectedID.value || !draft.value.trim()) return
  sending.value = true
  try {
    messages.value.push(await sendMessage(selectedID.value, draft.value.trim()))
    draft.value = ''
    await scrollToLatest()
    conversations.value = await fetchConversations()
  } catch (e) {
    Notify.danger(errorMessage(e, '消息发送失败'))
  } finally {
    sending.value = false
  }
}

async function findMembers() {
  try {
    const result = await searchUsers(memberQuery.value.trim())
    memberCandidates.value = result.filter((user) => user.id !== auth.user?.id)
  } catch (e) {
    Notify.danger(errorMessage(e, '用户搜索失败'))
  }
}

function toggleMember(id: number) {
  selectedMemberIDs.value = selectedMemberIDs.value.includes(id)
    ? selectedMemberIDs.value.filter((value) => value !== id)
    : [...selectedMemberIDs.value, id]
}

function openGroupDialog() {
  groupOpen.value = true
  if (!memberCandidates.value.length) findMembers()
}

async function createGroup() {
  if (selectedMemberIDs.value.length < 2) {
    Notify.warn('群聊至少选择两位其他成员')
    return
  }
  try {
    const conversation = await createConversation({
      kind: 'group',
      name: groupName.value.trim() || '未命名群聊',
      member_ids: selectedMemberIDs.value,
    })
    groupOpen.value = false
    groupName.value = ''
    memberQuery.value = ''
    memberCandidates.value = []
    selectedMemberIDs.value = []
    conversations.value = await fetchConversations()
    await openConversation(conversation.id)
  } catch (e) {
    Notify.danger(errorMessage(e, '群聊创建失败'))
  }
}

function conversationName(item: Conversation) {
  if (item.kind === 'group') return item.name || '群聊'
  return item.members.find((member) => member.id !== auth.user?.id)?.display_name || '私信'
}

watch(() => route.query.conversation, (value) => {
  const id = Number(value || 0)
  if (id && id !== selectedID.value && conversations.value.some((item) => item.id === id)) loadMessages(id)
})

onMounted(loadConversations)
</script>

<template>
  <PageContainer title="私信与群聊">
    <div class="rf-messages-shell" :class="{ 'has-selection': !!selected }">
      <aside class="rf-conversation-pane">
        <header>
          <strong>会话</strong>
          <button type="button" class="rf-text-button" @click="openGroupDialog">
            <AppIcon name="add" size="17" />
            <span>群聊</span>
          </button>
        </header>
        <div v-if="loading" class="rf-loading-block">加载中…</div>
        <template v-else>
          <button
            v-for="item in conversations"
            :key="item.id"
            type="button"
            class="rf-conversation-row"
            :class="{ active: item.id === selectedID }"
            @click="openConversation(item.id)"
          >
            <span class="rf-conversation-avatar"><AppIcon :name="item.kind === 'group' ? 'people' : 'user'" size="19" /></span>
            <span>
              <strong>{{ conversationName(item) }}</strong>
              <small>{{ item.last_message?.content || '暂无消息' }}</small>
            </span>
            <b v-if="item.unread_count">{{ item.unread_count > 99 ? '99+' : item.unread_count }}</b>
          </button>
          <div v-if="!conversations.length" class="rf-conversation-empty">
            <AppIcon name="message" size="30" />
            <strong>还没有会话</strong>
            <span>查找社区用户发起私信，或创建一个群聊。</span>
            <div>
              <button type="button" class="rf-primary-button" @click="router.push('/users')">查找用户</button>
              <button type="button" class="rf-text-button" @click="openGroupDialog">创建群聊</button>
            </div>
          </div>
        </template>
      </aside>

      <section class="rf-chat-pane">
        <header>
          <button v-if="selected" type="button" class="rf-icon-button rf-chat-back" aria-label="返回会话列表" @click="closeConversation">
            <AppIcon name="arrow" size="19" />
          </button>
          <AppIcon name="message" size="20" />
          <strong>{{ selectedTitle }}</strong>
          <small v-if="selected">{{ selected.members.length }} 人</small>
        </header>
        <div ref="messageList" class="rf-message-list">
          <div v-if="messagesLoading" class="rf-loading-block">加载消息…</div>
          <div v-else-if="!selected" class="rf-empty">
            <AppIcon name="message" size="28" />
            <strong>选择一个会话</strong>
            <span>从左侧会话列表继续聊天。</span>
          </div>
          <div v-for="item in messages" v-else :key="item.id" class="rf-message-row" :class="{ mine: item.sender_id === auth.user?.id }">
            <UserAvatar :src="item.sender_avatar" :name="item.sender_name" :size="34" />
            <div>
              <small v-if="item.sender_id !== auth.user?.id">{{ item.sender_name }}</small>
              <p>{{ item.content }}</p>
              <time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time>
            </div>
          </div>
        </div>
        <form v-if="selected" class="rf-message-editor" @submit.prevent="submitMessage">
          <textarea v-model="draft" rows="2" maxlength="5000" placeholder="输入消息，Enter 发送" @keydown.enter.exact.prevent="submitMessage" />
          <button type="submit" class="rf-primary-button" :disabled="sending || !draft.trim()">{{ sending ? '发送中…' : '发送' }}</button>
        </form>
      </section>
    </div>

    <div v-if="groupOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="groupOpen = false">
      <section class="rf-dialog rf-group-dialog" role="dialog" aria-modal="true" aria-labelledby="group-dialog-title">
        <header>
          <h2 id="group-dialog-title">创建群聊</h2>
          <button type="button" class="rf-icon-button" aria-label="关闭" @click="groupOpen = false"><AppIcon name="close" size="19" /></button>
        </header>
        <form class="rf-editor-form" @submit.prevent="createGroup">
          <label>群聊名称<input v-model="groupName" maxlength="80" placeholder="例如：周末组队" /></label>
          <label>添加成员
            <span class="rf-feed-search">
              <AppIcon name="search" size="18" />
              <input v-model="memberQuery" placeholder="搜索昵称" />
              <button type="button" @click="findMembers">搜索</button>
            </span>
          </label>
          <div class="rf-member-picker">
            <button v-for="user in memberCandidates" :key="user.id" type="button" :class="{ selected: selectedMemberIDs.includes(user.id) }" @click="toggleMember(user.id)">
              <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="26" />
              <span>{{ user.display_name }}</span>
              <AppIcon v-if="selectedMemberIDs.includes(user.id)" name="check" size="16" />
            </button>
          </div>
          <p class="rf-group-selection">已选择 {{ selectedMemberIDs.length }} 位用户</p>
          <button type="submit" class="rf-primary-button" :disabled="selectedMemberIDs.length < 2">创建群聊</button>
        </form>
      </section>
    </div>
  </PageContainer>
</template>

<style scoped>
.rf-conversation-empty { display: flex; min-height: 260px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 24px 16px; color: var(--rf-muted); text-align: center; }
.rf-conversation-empty strong { color: var(--rf-text); }
.rf-conversation-empty > span { max-width: 250px; font-size: 13px; line-height: 1.5; }
.rf-conversation-empty > div { display: flex; flex-wrap: wrap; justify-content: center; gap: 6px; margin-top: 8px; }
.rf-conversation-empty .rf-primary-button, .rf-conversation-empty .rf-text-button { min-height: 38px; padding-inline: 14px; font-size: 13px; }
.rf-group-dialog > form { padding: 18px 20px 22px; }
.rf-group-dialog .rf-feed-search { display: flex; margin-top: 7px; }
.rf-group-selection { margin: 0; color: var(--rf-muted); font-size: 12px; }
.rf-member-picker button > span { min-width: 0; overflow: hidden; flex: 1; text-overflow: ellipsis; white-space: nowrap; }
.rf-member-picker button > :deep(svg) { flex: 0 0 auto; color: var(--primary); }

@media (max-width: 760px) {
  .rf-messages-shell:not(.has-selection) .rf-conversation-pane { display: block; border-right: 0; }
  .rf-messages-shell:not(.has-selection) .rf-chat-pane { display: none; }
  .rf-messages-shell.has-selection .rf-conversation-pane { display: none; }
  .rf-messages-shell.has-selection .rf-chat-pane { display: flex; }
  .rf-conversation-pane > header { position: sticky; top: 0; z-index: 2; background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(10px); }
  .rf-conversation-row { min-height: 64px; padding-inline: 14px; }
  .rf-group-dialog > form { padding: 16px 14px calc(20px + env(safe-area-inset-bottom)); }
}
</style>
