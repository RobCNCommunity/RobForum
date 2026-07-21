import { ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchUnreadNotificationCount } from '@/api'

export const useNotificationsStore = defineStore('notifications', () => {
  const unread = ref(0)
  const loading = ref(false)

  async function refresh() {
    loading.value = true
    try { unread.value = (await fetchUnreadNotificationCount()).count } finally { loading.value = false }
  }
  function markRead() { unread.value = 0 }
  return { unread, loading, refresh, markRead }
})
