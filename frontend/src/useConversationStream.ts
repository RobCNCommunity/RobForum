import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { ChatMessage } from '@/api'

export type ConversationStreamStatus = 'idle' | 'connecting' | 'open' | 'reconnecting'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isChatMessage(value: unknown): value is ChatMessage {
  if (!isRecord(value)) return false
  return typeof value.id === 'number'
    && typeof value.conversation_id === 'number'
    && typeof value.sender_id === 'number'
    && typeof value.sender_name === 'string'
    && typeof value.sender_avatar === 'string'
    && typeof value.content === 'string'
    && typeof value.created_at === 'string'
}

function parseMessage(raw: string) {
  try {
    const value: unknown = JSON.parse(raw)
    return isChatMessage(value) ? value : null
  } catch (error) {
    if (error instanceof SyntaxError) return null
    throw error
  }
}

export function useConversationStream(conversationID: Ref<number>, receive: (message: ChatMessage) => void) {
  const status = ref<ConversationStreamStatus>('idle')
  let socket: WebSocket | undefined
  let reconnectTimer: number | undefined
  let reconnectAttempt = 0
  let generation = 0

  function clearReconnectTimer() {
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    reconnectTimer = undefined
  }

  function closeSocket() {
    const active = socket
    socket = undefined
    if (active && active.readyState < WebSocket.CLOSING) active.close(1000, 'conversation changed')
  }

  function scheduleReconnect(id: number, currentGeneration: number) {
    if (currentGeneration !== generation || conversationID.value !== id) return
    status.value = 'reconnecting'
    const delay = Math.min(15_000, 750 * 2 ** reconnectAttempt)
    reconnectAttempt += 1
    reconnectTimer = window.setTimeout(() => connect(id, currentGeneration), delay)
  }

  function connect(id: number, currentGeneration: number) {
    if (currentGeneration !== generation || id < 1) return
    clearReconnectTimer()
    status.value = reconnectAttempt ? 'reconnecting' : 'connecting'
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const nextSocket = new WebSocket(`${protocol}//${window.location.host}/api/v1/conversations/${id}/stream`)
    socket = nextSocket

    nextSocket.addEventListener('open', () => {
      if (socket !== nextSocket || currentGeneration !== generation) return
      reconnectAttempt = 0
      status.value = 'open'
    })
    nextSocket.addEventListener('message', (event: MessageEvent<string>) => {
      if (socket !== nextSocket || currentGeneration !== generation) return
      const message = parseMessage(event.data)
      if (message?.conversation_id === id) receive(message)
    })
    nextSocket.addEventListener('close', () => {
      if (socket !== nextSocket || currentGeneration !== generation) return
      socket = undefined
      scheduleReconnect(id, currentGeneration)
    })
    nextSocket.addEventListener('error', () => {
      if (socket === nextSocket) nextSocket.close()
    })
  }

  const stopWatching = watch(conversationID, (id) => {
    generation += 1
    clearReconnectTimer()
    closeSocket()
    reconnectAttempt = 0
    status.value = id > 0 ? 'connecting' : 'idle'
    if (id > 0) connect(id, generation)
  }, { immediate: true })

  onBeforeUnmount(() => {
    generation += 1
    stopWatching()
    clearReconnectTimer()
    closeSocket()
    status.value = 'idle'
  })

  return { status }
}
