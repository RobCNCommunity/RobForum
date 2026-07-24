<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { InfiniteLoading, Notify } from '@nutui/nutui'
import {
  createConversationInviteLink,
  createConversation,
  deleteConversation,
  errorMessage,
  fetchConversationMembers,
  fetchConversationInvites,
  fetchConversations,
  fetchMessages,
  inviteConversationMembers,
  leaveConversation,
  removeConversationMember,
  remindGroupInvitees,
  revokeConversationInviteLink,
  respondConversationInvite,
  searchUsersPage,
  sendMessage,
  updateConversationName,
  type ChatMessage,
  type Conversation,
  type ConversationInviteLink,
  type ConversationInvite,
  type ConversationMember,
  type UserSearchResult,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import PageContainer from '@/components/PageContainer.vue'
import AppIcon from '@/components/AppIcon.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useConversationStream, type ConversationStreamStatus } from '@/useConversationStream'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const conversations = ref<Conversation[]>([])
const invites = ref<ConversationInvite[]>([])
const selectedID = ref(0)
const messages = ref<ChatMessage[]>([])
const draft = ref('')
const loading = ref(true)
const messagesLoading = ref(false)
const sending = ref(false)
const processingInviteID = ref<number | null>(null)
const remindingConversationID = ref<number | null>(null)
const groupOpen = ref(false)
const groupMode = ref<'create' | 'invite'>('create')
const groupSubmitting = ref(false)
const groupName = ref('')
const memberQuery = ref('')
const memberCandidates = ref<UserSearchResult[]>([])
const selectedMemberIDs = ref<number[]>([])
const memberCandidatesLoading = ref(false)
const memberCandidatesOffset = ref(0)
const memberCandidatesHasMore = ref(true)
const groupManageOpen = ref(false)
const groupMembers = ref<ConversationMember[]>([])
const groupMembersLoading = ref(false)
const groupActionLoading = ref('')
const groupEditName = ref('')
const inviteLink = ref<ConversationInviteLink | null>(null)
const messageList = ref<HTMLElement | null>(null)
const messageInput = ref<HTMLTextAreaElement | null>(null)

const selected = computed(() => conversations.value.find((item) => item.id === selectedID.value) || null)
const selectedPeer = computed(() => selected.value?.members.find((member) => member.id !== auth.user?.id))
const selectedTitle = computed(() => {
  if (!selected.value) return '消息'
  return selected.value.kind === 'group' ? selected.value.name || '未命名群聊' : selectedPeer.value?.display_name || '私信'
})
const canSend = computed(() => Boolean(selected.value && selected.value.membership_status === 'accepted' && (selected.value.kind !== 'group' || selected.value.active)))
const canRemindInvitees = computed(() => Boolean(
  selected.value?.kind === 'group'
  && selected.value.created_by === auth.user?.id
  && selected.value.pending_invite_count > 0,
))
const canManageGroup = computed(() => Boolean(selected.value?.kind === 'group' && selected.value.created_by === auth.user?.id))
const groupDialogMinimum = computed(() => groupMode.value === 'create' ? 2 : 1)
const groupDialogTitle = computed(() => groupMode.value === 'create' ? '创建群聊' : '邀请更多成员')
const existingGroupMemberIDs = computed(() => new Set(groupMembers.value.map((member) => member.id)))
const selectedStatus = computed(() => {
  if (!selected.value) return ''
  if (selected.value.kind !== 'group') return '私信'
  if (selected.value.active) return `${selected.value.accepted_member_count} 位成员`
  return `等待 ${selected.value.pending_invite_count} 位成员确认邀请`
})
const streamStatusLabels = {
  idle: '实时连接未启动',
  connecting: '正在连接实时消息',
  open: '实时消息已连接',
  reconnecting: '实时消息正在重连',
} satisfies Record<ConversationStreamStatus, string>
const { status: streamStatus } = useConversationStream(selectedID, receiveMessage)
const streamStatusLabel = computed(() => streamStatusLabels[streamStatus.value])

function conversationName(item: Conversation) {
  if (item.kind === 'group') return item.name || '未命名群聊'
  return item.members.find((member) => member.id !== auth.user?.id)?.display_name || '私信'
}

function conversationPreview(item: Conversation) {
  if (item.kind === 'group' && !item.active) return `等待 ${item.pending_invite_count} 位成员确认`
  return item.last_message?.content || (item.kind === 'group' ? `${item.accepted_member_count} 位成员` : '开始一段私信')
}

function conversationPeer(item: Conversation) {
  return item.members.find((member) => member.id !== auth.user?.id)
}

async function scrollToLatest() {
  await nextTick()
  if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
}

function mergeMessages(primary: readonly ChatMessage[], secondary: readonly ChatMessage[]) {
  const byID = new Map<number, ChatMessage>()
  for (const item of primary) byID.set(item.id, item)
  for (const item of secondary) byID.set(item.id, item)
  return [...byID.values()].sort((left, right) => left.id - right.id)
}

function receiveMessage(message: ChatMessage) {
  if (message.conversation_id !== selectedID.value) return
  messages.value = mergeMessages(messages.value, [message])
  const conversation = conversations.value.find((item) => item.id === message.conversation_id)
  if (conversation) {
    conversation.last_message = message
    conversation.updated_at = message.created_at
    conversation.unread_count = 0
    conversations.value = [conversation, ...conversations.value.filter((item) => item.id !== conversation.id)]
  }
  void scrollToLatest()
}

function resizeMessageInput() {
  const input = messageInput.value
  if (!input) return
  input.style.height = 'auto'
  input.style.height = `${Math.min(Math.max(input.scrollHeight, 44), 92)}px`
}

async function refreshConversations() {
  const [nextConversations, nextInvites] = await Promise.all([fetchConversations(), fetchConversationInvites()])
  conversations.value = nextConversations
  invites.value = nextInvites
}

async function loadConversations() {
  loading.value = true
  try {
    await refreshConversations()
    const requestedID = Number(route.query.conversation || 0)
    selectedID.value = conversations.value.some((item) => item.id === requestedID) ? requestedID : 0
    if (selectedID.value) await loadMessages(selectedID.value)
    else messages.value = []
  } catch (error) {
    Notify.danger(errorMessage(error, '会话加载失败'))
  } finally {
    loading.value = false
  }
}

async function loadMessages(id: number) {
  selectedID.value = id
  messages.value = []
  messagesLoading.value = true
  try {
    const loaded = await fetchMessages(id)
    if (selectedID.value !== id) return
    messages.value = mergeMessages(loaded, messages.value)
  } catch (error) {
    if (selectedID.value !== id) return
    Notify.danger(errorMessage(error, '消息加载失败'))
  } finally {
    if (selectedID.value === id) messagesLoading.value = false
  }
  if (selectedID.value === id) await scrollToLatest()
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
  if (!selectedID.value || !draft.value.trim() || !canSend.value) return
  sending.value = true
  try {
    const conversationID = selectedID.value
    receiveMessage(await sendMessage(conversationID, draft.value.trim()))
    draft.value = ''
    await nextTick()
    resizeMessageInput()
    await scrollToLatest()
  } catch (error) {
    Notify.danger(errorMessage(error, '消息发送失败'))
  } finally {
    sending.value = false
  }
}

async function loadMemberCandidates(reset = false, done?: () => void) {
  if (memberCandidatesLoading.value || (!reset && !memberCandidatesHasMore.value)) {
    done?.()
    return
  }
  if (reset) {
    memberCandidatesOffset.value = 0
    memberCandidatesHasMore.value = true
    memberCandidates.value = []
  }
  memberCandidatesLoading.value = true
  try {
    const page = await searchUsersPage(memberQuery.value.trim(), memberCandidatesOffset.value, 30)
    const excluded = groupMode.value === 'invite' ? existingGroupMemberIDs.value : new Set<number>()
    const candidates = page.items.filter((user) => user.id !== auth.user?.id && !excluded.has(user.id))
    const known = new Set(memberCandidates.value.map((user) => user.id))
    memberCandidates.value = [...memberCandidates.value, ...candidates.filter((user) => !known.has(user.id))]
    memberCandidatesOffset.value = page.next_offset
    memberCandidatesHasMore.value = page.has_more
  } catch (error) {
    Notify.danger(errorMessage(error, '用户搜索失败'))
  } finally {
    memberCandidatesLoading.value = false
    done?.()
  }
}

async function findMembers() {
  selectedMemberIDs.value = []
  await loadMemberCandidates(true)
}

function toggleMember(id: number) {
  selectedMemberIDs.value = selectedMemberIDs.value.includes(id)
    ? selectedMemberIDs.value.filter((value) => value !== id)
    : [...selectedMemberIDs.value, id]
}

async function openGroupDialog(mode: 'create' | 'invite' = 'create') {
  groupMode.value = mode
  groupName.value = mode === 'create' ? '' : selected.value?.name || ''
  memberQuery.value = ''
  memberCandidates.value = []
  selectedMemberIDs.value = []
  memberCandidatesOffset.value = 0
  memberCandidatesHasMore.value = true
  if (mode === 'invite' && selected.value?.kind === 'group' && !groupMembers.value.length) await loadGroupMembers()
  groupOpen.value = true
  await loadMemberCandidates(true)
}

function closeGroupDialog() {
  if (!groupSubmitting.value) groupOpen.value = false
}

async function submitGroupDialog() {
  if (groupSubmitting.value) return
  if (selectedMemberIDs.value.length < groupDialogMinimum.value) {
    Notify.warn(groupMode.value === 'create' ? '群聊至少邀请两位其他用户' : '请至少选择一位用户')
    return
  }
  groupSubmitting.value = true
  try {
    if (groupMode.value === 'create') {
      const conversation = await createConversation({
        kind: 'group',
        name: groupName.value.trim() || '未命名群聊',
        member_ids: selectedMemberIDs.value,
      })
      await refreshConversations()
      await openConversation(conversation.id)
      Notify.success('群聊已创建，正在等待受邀成员确认')
    } else if (selected.value) {
      const result = await inviteConversationMembers(selected.value.id, selectedMemberIDs.value)
      await refreshConversations()
      await loadGroupMembers()
      Notify.success(result.invited > 0 ? `已邀请 ${result.invited} 位用户` : '所选用户已经在群聊或等待确认')
    }
    groupOpen.value = false
    groupName.value = ''
    memberQuery.value = ''
    memberCandidates.value = []
    selectedMemberIDs.value = []
  } catch (error) {
    Notify.danger(errorMessage(error, groupMode.value === 'create' ? '群聊创建失败' : '邀请成员失败'))
  } finally {
    groupSubmitting.value = false
  }
}

async function respondInvite(invite: ConversationInvite, accept: boolean) {
  if (processingInviteID.value !== null) return
  processingInviteID.value = invite.conversation.id
  try {
    const conversation = await respondConversationInvite(invite.conversation.id, accept)
    await refreshConversations()
    if (accept) {
      await openConversation(conversation.id)
      Notify.success('已加入群聊')
    } else {
      Notify.success('已忽略群聊邀请')
    }
  } catch (error) {
    Notify.danger(errorMessage(error, '邀请处理失败'))
  } finally {
    processingInviteID.value = null
  }
}

async function remindInvitees() {
  const conversation = selected.value
  if (!conversation || !canRemindInvitees.value || remindingConversationID.value !== null) return
  remindingConversationID.value = conversation.id
  try {
    const result = await remindGroupInvitees(conversation.id)
    await refreshConversations()
    Notify.success(`已提醒 ${result.reminded} 位成员确认邀请`)
  } catch (error) {
    Notify.danger(errorMessage(error, '提醒发送失败'))
  } finally {
    remindingConversationID.value = null
  }
}

async function loadGroupMembers() {
  const conversation = selected.value
  if (!conversation || conversation.kind !== 'group') {
    groupMembers.value = []
    return
  }
  groupMembersLoading.value = true
  try {
    groupMembers.value = await fetchConversationMembers(conversation.id)
  } catch (error) {
    Notify.danger(errorMessage(error, '群成员加载失败'))
  } finally {
    groupMembersLoading.value = false
  }
}

async function openGroupManager() {
  if (!selected.value || selected.value.kind !== 'group') return
  groupEditName.value = selected.value.name || '未命名群聊'
  inviteLink.value = null
  groupManageOpen.value = true
  await loadGroupMembers()
}

async function saveGroupName() {
  const conversation = selected.value
  const name = groupEditName.value.trim()
  if (!conversation || !canManageGroup.value || !name || groupActionLoading.value) return
  groupActionLoading.value = 'rename'
  try {
    await updateConversationName(conversation.id, name)
    await refreshConversations()
    Notify.success('群聊名称已更新')
  } catch (error) {
    Notify.danger(errorMessage(error, '群聊名称更新失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

async function generateInviteLink() {
  const conversation = selected.value
  if (!conversation || !canManageGroup.value || groupActionLoading.value) return
  groupActionLoading.value = 'link'
  try {
    inviteLink.value = await createConversationInviteLink(conversation.id)
    Notify.success('新的邀请链接已生成，旧链接已失效')
  } catch (error) {
    Notify.danger(errorMessage(error, '邀请链接生成失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

async function copyInviteLink() {
  if (!inviteLink.value?.join_url) return
  try {
    await navigator.clipboard.writeText(inviteLink.value.join_url)
    Notify.success('邀请链接已复制')
  } catch {
    Notify.danger('复制失败，请手动复制邀请链接')
  }
}

function selectInviteLinkInput(event: FocusEvent) {
  if (event.currentTarget instanceof HTMLInputElement) event.currentTarget.select()
}

async function revokeInviteLink() {
  const conversation = selected.value
  if (!conversation || !canManageGroup.value || groupActionLoading.value) return
  groupActionLoading.value = 'revoke-link'
  try {
    await revokeConversationInviteLink(conversation.id)
    inviteLink.value = null
    Notify.success('邀请链接已失效')
  } catch (error) {
    Notify.danger(errorMessage(error, '邀请链接撤销失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

async function removeGroupMember(member: ConversationMember) {
  const conversation = selected.value
  if (!conversation || !canManageGroup.value || member.id === auth.user?.id || groupActionLoading.value) return
  if (!window.confirm(`确定将“${member.display_name}”移出群聊吗？`)) return
  groupActionLoading.value = `remove-${member.id}`
  try {
    await removeConversationMember(conversation.id, member.id)
    await Promise.all([refreshConversations(), loadGroupMembers()])
    Notify.success('成员已移出群聊')
  } catch (error) {
    Notify.danger(errorMessage(error, '移除成员失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

async function leaveSelectedGroup() {
  const conversation = selected.value
  if (!conversation || conversation.kind !== 'group' || canManageGroup.value || groupActionLoading.value) return
  if (!window.confirm('确定退出这个群聊吗？')) return
  groupActionLoading.value = 'leave'
  try {
    await leaveConversation(conversation.id)
    groupManageOpen.value = false
    await closeConversation()
    await refreshConversations()
    Notify.success('已退出群聊')
  } catch (error) {
    Notify.danger(errorMessage(error, '退出群聊失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

async function dissolveSelectedGroup() {
  const conversation = selected.value
  if (!conversation || !canManageGroup.value || groupActionLoading.value) return
  if (!window.confirm('解散后聊天记录和邀请链接都会失效，确定继续吗？')) return
  groupActionLoading.value = 'dissolve'
  try {
    await deleteConversation(conversation.id)
    groupManageOpen.value = false
    await closeConversation()
    await refreshConversations()
    Notify.success('群聊已解散')
  } catch (error) {
    Notify.danger(errorMessage(error, '解散群聊失败'))
  } finally {
    groupActionLoading.value = ''
  }
}

watch(() => route.query.conversation, (value) => {
  const id = Number(value || 0)
  if (id && id !== selectedID.value && conversations.value.some((item) => item.id === id)) void loadMessages(id)
})

onMounted(loadConversations)
</script>

<template>
  <PageContainer class="rf-messages-page">
    <div class="rf-messages-shell" :class="{ 'has-selection': !!selected }">
      <aside class="rf-conversation-pane">
        <header>
          <strong>消息</strong>
          <button type="button" class="rf-text-button" @click="openGroupDialog">
            <AppIcon name="add" size="17" />
            <span>新建群聊</span>
          </button>
        </header>

        <div v-if="loading" class="rf-loading-block">加载中…</div>
        <template v-else>
          <section v-if="invites.length" class="rf-conversation-invites" aria-label="群聊邀请">
            <div class="rf-conversation-section-label"><AppIcon name="invite" size="16" /><strong>群聊邀请</strong><span>{{ invites.length }}</span></div>
            <article v-for="invite in invites" :key="invite.conversation.id" class="rf-invite-row">
              <span class="rf-conversation-avatar"><AppIcon name="people" size="18" /></span>
              <div>
                <strong>{{ invite.conversation.name || '未命名群聊' }}</strong>
                <small>{{ invite.conversation.members.length }} 人邀请你加入</small>
              </div>
              <div class="rf-invite-actions">
                <button type="button" :disabled="processingInviteID !== null" @click="respondInvite(invite, false)">忽略</button>
                <button type="button" :disabled="processingInviteID !== null" @click="respondInvite(invite, true)">接受</button>
              </div>
            </article>
          </section>

          <button
            v-for="item in conversations"
            :key="item.id"
            type="button"
            class="rf-conversation-row"
            :class="{ active: item.id === selectedID }"
            @click="openConversation(item.id)"
          >
            <UserAvatar v-if="item.kind === 'direct'" :src="conversationPeer(item)?.avatar_url" :name="conversationName(item)" :size="38" />
            <span v-else class="rf-conversation-avatar"><AppIcon name="people" size="19" /></span>
            <span>
              <strong>{{ conversationName(item) }}</strong>
              <small>{{ conversationPreview(item) }}</small>
            </span>
            <b v-if="item.unread_count">{{ item.unread_count > 99 ? '99+' : item.unread_count }}</b>
          </button>

          <div v-if="!conversations.length && !invites.length" class="rf-conversation-empty">
            <AppIcon name="message" size="30" />
            <strong>还没有会话</strong>
            <span>先前往用户主页，再从资料页发起私信。</span>
            <div>
              <button type="button" class="rf-primary-button" @click="router.push('/users')">发现用户</button>
              <button type="button" class="rf-text-button" @click="openGroupDialog">创建群聊</button>
            </div>
          </div>
        </template>
      </aside>

      <section class="rf-chat-pane">
        <header>
          <button v-if="selected" type="button" class="rf-icon-button rf-chat-back" aria-label="返回会话列表" @click="closeConversation">
            <AppIcon name="back" size="19" />
          </button>
          <UserAvatar v-if="selected?.kind === 'direct'" :src="selectedPeer?.avatar_url" :name="selectedTitle" :size="32" />
          <span v-else class="rf-chat-group-mark"><AppIcon name="people" size="18" /></span>
          <div class="rf-chat-heading">
            <span><strong>{{ selectedTitle }}</strong><i v-if="selected" class="rf-stream-state" :class="streamStatus" role="status" :aria-label="streamStatusLabel" :title="streamStatusLabel" /></span>
            <small v-if="selected">{{ selectedStatus }}</small>
          </div>
          <button v-if="selected?.kind === 'group'" type="button" class="rf-text-button rf-group-manage-trigger" @click="openGroupManager">
            <AppIcon name="settings" size="17" />
            <span>{{ canManageGroup ? '管理群聊' : '群成员' }}</span>
          </button>
        </header>

        <div ref="messageList" class="rf-message-list">
          <div v-if="messagesLoading" class="rf-loading-block">加载消息…</div>
          <div v-else-if="!selected" class="rf-empty">
            <AppIcon name="message" size="28" />
            <strong>选择一段对话</strong>
            <span>从会话列表中继续聊天，或在用户主页发起私信。</span>
          </div>
          <template v-else>
            <div v-if="selected.kind === 'group' && selected.pending_invite_count > 0" class="rf-group-pending-message">
              <AppIcon name="invite" size="20" />
              <div class="rf-group-pending-copy">
                <strong>{{ selected.active ? `还有 ${selected.pending_invite_count} 位成员待确认` : '正在等待成员确认' }}</strong>
                <span>{{ selected.active ? '受邀成员接受后会自动加入群聊。' : '群聊会在至少三位成员接受邀请后开放发言。' }}</span>
              </div>
              <button v-if="canRemindInvitees" type="button" class="rf-text-button rf-group-remind-button" :disabled="remindingConversationID !== null" @click="remindInvitees">
                <span v-if="remindingConversationID !== null" class="rf-inline-spinner" aria-hidden="true" />
                <AppIcon v-else name="invite" size="15" />
                <span>{{ remindingConversationID !== null ? '提醒中' : '再次提醒' }}</span>
              </button>
            </div>
            <div v-for="item in messages" :key="item.id" class="rf-message-row" :class="{ mine: item.sender_id === auth.user?.id }">
              <UserAvatar :src="item.sender_avatar" :name="item.sender_name" :size="34" />
              <div>
                <small v-if="item.sender_id !== auth.user?.id">{{ item.sender_name }}</small>
                <p>{{ item.content }}</p>
                <time>{{ new Date(item.created_at).toLocaleString('zh-CN') }}</time>
              </div>
            </div>
            <div v-if="!messages.length && selected.active" class="rf-empty rf-message-empty"><strong>还没有消息</strong><span>发送第一条消息开始聊天。</span></div>
          </template>
        </div>

        <form v-if="selected && canSend" class="rf-message-editor" @submit.prevent="submitMessage">
          <textarea ref="messageInput" v-model="draft" rows="1" maxlength="5000" placeholder="输入消息" @input="resizeMessageInput" @keydown.enter.exact.prevent="submitMessage" />
          <button type="submit" class="rf-primary-button" aria-label="发送消息" :disabled="sending || !draft.trim()"><AppIcon name="send" size="18" /><span>{{ sending ? '发送中' : '发送' }}</span></button>
        </form>
      </section>
    </div>

    <div v-if="groupOpen" class="rf-modal-backdrop rf-modal-backdrop--center" @click.self="closeGroupDialog">
      <section class="rf-dialog rf-group-dialog" role="dialog" aria-modal="true" aria-labelledby="group-dialog-title">
        <header>
          <div><h2 id="group-dialog-title">{{ groupDialogTitle }}</h2><p>{{ groupMode === 'create' ? '受邀成员确认后才会加入群聊。' : '可以多次追加邀请，已在群内的用户会自动排除。' }}</p></div>
          <button type="button" class="rf-icon-button" aria-label="关闭" :disabled="groupSubmitting" @click="closeGroupDialog"><AppIcon name="close" size="19" /></button>
        </header>
        <form class="rf-editor-form" @submit.prevent="submitGroupDialog">
          <label v-if="groupMode === 'create'">群聊名称<input v-model="groupName" maxlength="80" placeholder="例如：周末组队" /></label>
          <label>邀请成员
            <span class="rf-feed-search">
              <AppIcon name="search" size="18" />
              <input v-model="memberQuery" placeholder="搜索昵称或 Roblox 名称" @keyup.enter.prevent="findMembers" />
              <button type="button" :disabled="memberCandidatesLoading" @click="findMembers">搜索</button>
            </span>
          </label>
          <div class="rf-member-picker-scroll">
            <InfiniteLoading
              :has-more="memberCandidatesHasMore"
              :is-open-refresh="true"
              :use-window="false"
              :threshold="80"
              pull-icon="refresh"
              load-icon="loading"
              pull-txt="下拉刷新用户"
              load-txt="加载更多用户"
              load-more-txt="已经加载全部用户"
              @load-more="(done) => loadMemberCandidates(false, done)"
              @refresh="(done) => loadMemberCandidates(true, done)"
            >
              <div class="rf-member-picker">
                <button v-for="user in memberCandidates" :key="user.id" type="button" :class="{ selected: selectedMemberIDs.includes(user.id) }" @click="toggleMember(user.id)">
                  <UserAvatar :src="user.avatar_url" :name="user.display_name" :size="28" />
                  <span><strong>{{ user.display_name }}</strong><small v-if="user.roblox_name">@{{ user.roblox_name }}</small></span>
                  <AppIcon v-if="selectedMemberIDs.includes(user.id)" name="plainCheck" size="16" />
                </button>
                <div v-if="!memberCandidates.length && !memberCandidatesLoading" class="rf-member-picker-empty">没有可邀请的用户</div>
              </div>
              <template #loading><div class="rf-member-picker-loading"><span class="rf-inline-spinner" />正在加载更多用户</div></template>
            </InfiniteLoading>
          </div>
          <p class="rf-group-selection">已选择 {{ selectedMemberIDs.length }} 位用户。{{ groupMode === 'create' ? '创建群聊至少需要两位。' : '可稍后再次继续邀请。' }}</p>
          <button type="submit" class="rf-primary-button rf-group-submit" :disabled="selectedMemberIDs.length < groupDialogMinimum || groupSubmitting">
            <span v-if="groupSubmitting" class="rf-button-spinner" aria-hidden="true" />
            <span>{{ groupSubmitting ? '提交中' : groupMode === 'create' ? '发送群聊邀请' : '邀请所选用户' }}</span>
          </button>
        </form>
      </section>
    </div>

    <nut-popup v-model:visible="groupManageOpen" position="right" :style="{ width: 'min(430px, 100vw)', height: '100%' }" closeable round pop-class="rf-group-manage-popup">
      <section class="rf-group-manager">
        <header><div><h2>{{ selected?.name || '群聊管理' }}</h2><p>{{ selected?.accepted_member_count || 0 }} 位成员 · {{ selected?.pending_invite_count || 0 }} 位待确认</p></div></header>

        <form v-if="canManageGroup" class="rf-group-name-form" @submit.prevent="saveGroupName">
          <label>群聊名称<input v-model="groupEditName" maxlength="120" /></label>
          <nut-button size="small" type="primary" :loading="groupActionLoading === 'rename'" @click="saveGroupName">保存</nut-button>
        </form>

        <div v-if="canManageGroup" class="rf-group-manager-actions">
          <nut-button block plain type="primary" @click="groupManageOpen = false; openGroupDialog('invite')"><AppIcon name="add" size="17" />继续邀请成员</nut-button>
          <nut-button block plain @click="generateInviteLink"><AppIcon name="share" size="17" />{{ inviteLink ? '重新生成邀请链接' : '生成邀请链接' }}</nut-button>
        </div>

        <section v-if="canManageGroup && inviteLink" class="rf-group-link-box">
          <div><strong>邀请链接</strong><small>7 天内有效，任何已登录用户可通过链接加入。</small></div>
          <input :value="inviteLink.join_url" readonly @focus="selectInviteLinkInput" />
          <div><nut-button size="small" type="primary" @click="copyInviteLink">复制链接</nut-button><nut-button size="small" plain :loading="groupActionLoading === 'revoke-link'" @click="revokeInviteLink">使链接失效</nut-button></div>
        </section>

        <section class="rf-group-members" aria-label="群成员列表">
          <header><strong>群成员</strong><span>{{ groupMembers.length }}</span></header>
          <div v-if="groupMembersLoading" class="rf-loading-block">加载群成员…</div>
          <article v-for="member in groupMembers" v-else :key="member.id">
            <UserAvatar :src="member.avatar_url" :name="member.display_name" :size="40" />
            <div><strong>{{ member.display_name }}</strong><small>{{ member.id === selected?.created_by ? '群主' : member.membership_status === 'pending' ? '等待确认邀请' : '群成员' }}</small></div>
            <button v-if="canManageGroup && member.id !== auth.user?.id" type="button" class="rf-group-remove" :disabled="!!groupActionLoading" @click="removeGroupMember(member)">
              <span v-if="groupActionLoading === `remove-${member.id}`" class="rf-inline-spinner" />
              <span v-else>{{ member.membership_status === 'pending' ? '取消邀请' : '移出' }}</span>
            </button>
          </article>
        </section>

        <div class="rf-group-danger-actions">
          <nut-button v-if="!canManageGroup" block plain type="danger" :loading="groupActionLoading === 'leave'" @click="leaveSelectedGroup">退出群聊</nut-button>
          <nut-button v-else block plain type="danger" :loading="groupActionLoading === 'dissolve'" @click="dissolveSelectedGroup">解散群聊</nut-button>
        </div>
      </section>
    </nut-popup>
  </PageContainer>
</template>

<style scoped>
.rf-conversation-empty { display: flex; min-height: 260px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 24px 16px; color: var(--rf-muted); text-align: center; }.rf-conversation-empty strong { color: var(--rf-text); }.rf-conversation-empty > span { max-width: 250px; font-size: 13px; line-height: 1.5; }.rf-conversation-empty > div { display: flex; flex-wrap: wrap; justify-content: center; gap: 6px; margin-top: 8px; }.rf-conversation-empty .rf-primary-button, .rf-conversation-empty .rf-text-button { min-height: 38px; padding-inline: 14px; font-size: 13px; }
.rf-conversation-invites { border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--primary) 3%, var(--rf-bg)); }.rf-conversation-section-label { display: flex; align-items: center; gap: 6px; padding: 11px 12px 7px; color: var(--rf-muted); font-size: 12px; }.rf-conversation-section-label strong { color: var(--rf-text); }.rf-conversation-section-label span { display: inline-grid; min-width: 18px; height: 18px; margin-left: auto; place-items: center; border-radius: 50%; color: #fff; background: var(--primary); font-size: 10px; font-variant-numeric: tabular-nums; }
.rf-invite-row { display: grid; grid-template-columns: 34px minmax(0, 1fr); gap: 8px; padding: 8px 12px 11px; }.rf-invite-row > div { min-width: 0; }.rf-invite-row strong, .rf-invite-row small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-invite-row small { margin-top: 2px; color: var(--rf-muted); font-size: 11px; }.rf-invite-actions { grid-column: 2; display: flex; gap: 8px; }.rf-invite-actions button { min-height: 29px; padding: 0 10px; border-radius: var(--rf-pill); color: var(--primary); background: transparent; font-size: 12px; font-weight: 700; }.rf-invite-actions button:last-child { color: #fff; background: var(--primary); }.rf-invite-actions button:disabled { cursor: wait; opacity: .55; }
.rf-chat-group-mark { display: inline-grid; width: 32px; height: 32px; flex: 0 0 32px; place-items: center; border-radius: 50%; color: var(--primary); background: color-mix(in srgb, var(--primary) 10%, var(--rf-bg)); }.rf-chat-heading { display: flex; min-width: 0; flex: 1; flex-direction: column; }.rf-chat-heading > span { display: flex; min-width: 0; align-items: center; gap: 7px; }.rf-chat-heading strong, .rf-chat-heading small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-chat-heading small { color: var(--rf-muted); font-size: 11px; }.rf-stream-state { width: 8px; height: 8px; flex: 0 0 8px; border-radius: 50%; background: var(--rf-faint); }.rf-stream-state.open { background: var(--rf-success); }.rf-stream-state.connecting, .rf-stream-state.reconnecting { background: var(--primary); }.rf-group-manage-trigger { min-height: 34px; flex: 0 0 auto; gap: 5px; padding-inline: 10px; font-size: 12px; }
.rf-group-pending-message { display: grid; width: min(100%, 520px); grid-template-columns: auto minmax(0, 1fr) auto; align-items: start; gap: 10px; margin: 0 auto 5px; padding: 11px 13px; border-radius: 9px; color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); }.rf-group-pending-message > svg { flex: 0 0 auto; margin-top: 2px; }.rf-group-pending-copy { display: flex; min-width: 0; flex-direction: column; gap: 2px; }.rf-group-pending-message strong { font-size: 13px; }.rf-group-pending-copy > span { color: var(--rf-muted); font-size: 12px; line-height: 1.45; }.rf-group-remind-button { min-height: 30px; gap: 5px; padding: 0 9px; color: var(--primary); font-size: 12px; white-space: nowrap; }.rf-group-remind-button:disabled { cursor: wait; opacity: .6; }.rf-inline-spinner { width: 14px; height: 14px; border: 2px solid color-mix(in srgb, var(--primary) 25%, transparent); border-top-color: var(--primary); border-radius: 50%; animation: rf-group-spin .7s linear infinite; }.rf-message-empty { min-height: 200px; padding-top: 32px; }
.rf-message-editor .rf-primary-button { gap: 6px; padding-inline: 14px; }.rf-group-dialog > form { padding: 18px 20px 22px; }.rf-group-dialog > header p { margin: 3px 0 0; color: var(--rf-muted); font-size: 12px; }.rf-group-dialog .rf-feed-search { display: flex; margin-top: 7px; }.rf-group-selection { margin: 0; color: var(--rf-muted); font-size: 12px; }.rf-member-picker-scroll { max-height: 280px; overflow-y: auto; overscroll-behavior: contain; border: 1px solid var(--rf-line); border-radius: 10px; background: var(--rf-bg); }.rf-member-picker { max-height: none; padding: 5px; overflow: visible; }.rf-member-picker button { min-height: 48px; }.rf-member-picker button > span { display: flex; min-width: 0; overflow: hidden; flex: 1; flex-direction: column; text-overflow: ellipsis; white-space: nowrap; }.rf-member-picker button strong, .rf-member-picker button small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-member-picker button small { color: var(--rf-muted); font-size: 11px; font-weight: 400; }.rf-member-picker button > :deep(svg) { flex: 0 0 auto; color: var(--primary); }.rf-member-picker-empty, .rf-member-picker-loading { display: flex; min-height: 74px; align-items: center; justify-content: center; gap: 7px; color: var(--rf-muted); font-size: 12px; }.rf-member-picker-scroll :deep(.nut-infinite-top) { color: var(--rf-muted); background: var(--rf-bg); }.rf-group-submit { display: inline-flex; align-items: center; justify-content: center; gap: 8px; }.rf-button-spinner { width: 15px; height: 15px; border: 2px solid rgba(255,255,255,.44); border-top-color: #fff; border-radius: 50%; animation: rf-group-spin .7s linear infinite; }
.rf-group-manager { min-height: 100%; padding: 22px 18px calc(24px + env(safe-area-inset-bottom)); color: var(--rf-text); background: var(--rf-bg); }.rf-group-manager > header { padding: 0 32px 16px 0; border-bottom: 1px solid var(--rf-line); }.rf-group-manager h2 { margin: 0; font-size: 22px; letter-spacing: -.02em; }.rf-group-manager > header p { margin: 4px 0 0; color: var(--rf-muted); font-size: 12px; }.rf-group-name-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: 10px; padding: 17px 0; border-bottom: 1px solid var(--rf-line); }.rf-group-name-form label { display: flex; min-width: 0; flex-direction: column; gap: 7px; color: var(--rf-muted); font-size: 12px; font-weight: 600; }.rf-group-name-form input, .rf-group-link-box input { width: 100%; min-height: 40px; padding: 0 11px; border: 1px solid var(--rf-line); border-radius: 7px; outline: 0; color: var(--rf-text); background: var(--rf-bg-subtle); }.rf-group-name-form input:focus, .rf-group-link-box input:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 13%, transparent); }.rf-group-manager-actions { display: grid; gap: 8px; padding: 16px 0; border-bottom: 1px solid var(--rf-line); }.rf-group-manager-actions :deep(.nut-button) { justify-content: flex-start; gap: 7px; }.rf-group-link-box { display: flex; flex-direction: column; gap: 9px; padding: 16px 0; border-bottom: 1px solid var(--rf-line); }.rf-group-link-box > div:first-child { display: flex; flex-direction: column; gap: 3px; }.rf-group-link-box small { color: var(--rf-muted); font-size: 11px; line-height: 1.45; }.rf-group-link-box > div:last-child { display: flex; gap: 8px; }.rf-group-members > header { display: flex; min-height: 48px; align-items: center; justify-content: space-between; }.rf-group-members > header span { color: var(--rf-muted); font-size: 12px; }.rf-group-members article { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: 10px; min-height: 62px; border-top: 1px solid var(--rf-line); }.rf-group-members article > div { display: flex; min-width: 0; flex-direction: column; }.rf-group-members article strong, .rf-group-members article small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.rf-group-members article small { color: var(--rf-muted); font-size: 11px; }.rf-group-remove { min-height: 32px; padding: 0 8px; border-radius: var(--rf-pill); color: var(--rf-danger); background: transparent; font-size: 12px; font-weight: 700; }.rf-group-remove:hover { background: color-mix(in srgb, var(--rf-danger) 8%, transparent); }.rf-group-remove:disabled { cursor: wait; opacity: .55; }.rf-group-danger-actions { padding-top: 22px; }.rf-group-danger-actions :deep(.nut-button--plain.nut-button--danger) { color: var(--rf-danger); border-color: color-mix(in srgb, var(--rf-danger) 45%, var(--rf-line)); background: transparent; }
@keyframes rf-group-spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) { .rf-messages-shell:not(.has-selection) .rf-conversation-pane { display: block; border-right: 0; }.rf-messages-shell:not(.has-selection) .rf-chat-pane { display: none; }.rf-messages-shell.has-selection .rf-conversation-pane { display: none; }.rf-messages-shell.has-selection .rf-chat-pane { display: flex; }.rf-conversation-pane > header { position: sticky; top: 0; z-index: 2; background: color-mix(in srgb, var(--rf-bg) 92%, transparent); backdrop-filter: blur(10px); }.rf-conversation-row { min-height: 64px; padding-inline: 14px; }.rf-group-pending-message { grid-template-columns: auto minmax(0, 1fr); }.rf-group-remind-button { grid-column: 2; justify-self: start; }.rf-group-dialog > form { padding: 16px 14px calc(20px + env(safe-area-inset-bottom)); }.rf-group-manage-trigger { width: 36px; height: 36px; padding: 0; justify-content: center; }.rf-group-manage-trigger span { display: none; }.rf-member-picker-scroll { max-height: min(44vh, 320px); }.rf-group-manager { padding-inline: 15px; } }
</style>
