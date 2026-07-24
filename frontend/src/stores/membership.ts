import { ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchMembershipConfig, type MembershipSettings } from '@/api'

const fallbackConfig: MembershipSettings = {
  enabled: false,
  default_withdrawal_fee_bps: 300,
  default_service_fee_bps: 500,
  tiers: [],
  updated_at: '',
}

export const useMembershipStore = defineStore('membership', () => {
  const config = ref<MembershipSettings>({ ...fallbackConfig })
  const loading = ref(false)
  let loaded = false
  let pending: Promise<MembershipSettings> | null = null

  function setConfig(value: MembershipSettings) {
    config.value = value
    loaded = true
  }

  async function load(force = false) {
    if (loaded && !force) return config.value
    if (pending && !force) return pending
    loading.value = true
    pending = fetchMembershipConfig()
      .then((value) => {
        setConfig(value)
        return value
      })
      .catch(() => {
        loaded = true
        return config.value
      })
      .finally(() => {
        loading.value = false
        pending = null
      })
    return pending
  }

  return { config, loading, load, setConfig }
})
