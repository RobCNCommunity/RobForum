<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from '@nutui/nutui'
import { deleteMyCover, errorMessage, updateMyProfile, uploadMyAvatar, uploadMyCover } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppIcon from '@/components/AppIcon.vue'
import PageContainer from '@/components/PageContainer.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import VerifiedBadge from '@/components/VerifiedBadge.vue'
import MembershipBadge from '@/components/MembershipBadge.vue'

const auth = useAuthStore()
const router = useRouter()
const avatarInput = ref<HTMLInputElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)
const form = reactive({
  display_name: auth.user?.display_name || '',
  bio: auth.user?.bio || '',
})
const avatarFile = ref<File | null>(null)
const avatarPreview = ref('')
const coverFile = ref<File | null>(null)
const coverPreview = ref('')
const coverRemoved = ref(false)
const saving = ref(false)

const currentAvatar = computed(() => avatarPreview.value || auth.user?.avatar_url || '')
const currentCover = computed(() => coverPreview.value || (coverRemoved.value ? '' : auth.user?.cover_url || ''))
const canSave = computed(() => form.display_name.trim().length >= 2 && !saving.value)

function revokePreview(value: string) {
  if (value) URL.revokeObjectURL(value)
}

function chooseImage(
  event: Event,
  kind: 'avatar' | 'cover',
) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] || null
  if (!file) return
  if (!['image/jpeg', 'image/png'].includes(file.type)) {
    Notify.warn(`${kind === 'avatar' ? '头像' : '封面'}仅支持 JPG 和 PNG`)
    input.value = ''
    return
  }
	const maxSize = kind === 'avatar' ? 5 * 1024 * 1024 : 7 * 1024 * 1024
  if (file.size > maxSize) {
		Notify.warn(`${kind === 'avatar' ? '头像不能超过 5 MB' : '封面不能超过 7 MB'}`)
    input.value = ''
    return
  }
  if (kind === 'avatar') {
    revokePreview(avatarPreview.value)
    avatarFile.value = file
    avatarPreview.value = URL.createObjectURL(file)
    return
  }
  revokePreview(coverPreview.value)
  coverFile.value = file
  coverPreview.value = URL.createObjectURL(file)
  coverRemoved.value = false
}

function removeCover() {
  revokePreview(coverPreview.value)
  coverPreview.value = ''
  coverFile.value = null
  coverRemoved.value = !!auth.user?.cover_url
  if (coverInput.value) coverInput.value.value = ''
}

async function saveProfile() {
  const displayName = form.display_name.trim()
  if (displayName.length < 2) {
    Notify.warn('昵称至少需要 2 个字符')
    return
  }
  saving.value = true
  try {
    auth.setUser(await updateMyProfile({ display_name: displayName, bio: form.bio.trim() }))
    if (avatarFile.value) auth.setUser(await uploadMyAvatar(avatarFile.value))
    if (coverFile.value) auth.setUser(await uploadMyCover(coverFile.value))
    else if (coverRemoved.value) auth.setUser(await deleteMyCover())

    avatarFile.value = null
    coverFile.value = null
    coverRemoved.value = false
    if (avatarInput.value) avatarInput.value.value = ''
    if (coverInput.value) coverInput.value.value = ''
    revokePreview(avatarPreview.value)
    revokePreview(coverPreview.value)
    avatarPreview.value = ''
    coverPreview.value = ''
    Notify.success('个人资料已保存')
  } catch (error) {
    Notify.danger(errorMessage(error, '个人资料保存失败'))
  } finally {
    saving.value = false
  }
}

onBeforeUnmount(() => {
  revokePreview(avatarPreview.value)
  revokePreview(coverPreview.value)
})
</script>

<template>
  <PageContainer>
    <form class="rf-x-edit-profile" @submit.prevent="saveProfile">
      <header class="rf-x-edit-header">
        <button type="button" class="rf-x-edit-back" aria-label="返回" @click="router.back()">
          <AppIcon name="back" size="21" />
        </button>
        <strong>编辑个人资料</strong>
        <button type="submit" class="rf-x-edit-save" :disabled="!canSave">
          {{ saving ? '保存中' : '保存' }}
        </button>
      </header>

      <section class="rf-x-edit-media" aria-label="头像和封面">
        <div class="rf-x-edit-cover" :class="{ empty: !currentCover }">
          <img v-if="currentCover" :src="currentCover" alt="封面预览" />
          <div class="rf-x-edit-cover-controls">
            <button type="button" aria-label="选择封面图片" title="选择封面图片" @click="coverInput?.click()">
              <AppIcon name="image" size="21" />
            </button>
            <button v-if="currentCover" type="button" aria-label="移除封面" title="移除封面" @click="removeCover">
              <AppIcon name="close" size="21" />
            </button>
          </div>
        </div>
        <button type="button" class="rf-x-edit-avatar" aria-label="选择头像图片" @click="avatarInput?.click()">
          <UserAvatar :src="currentAvatar" :name="form.display_name" :size="112" />
          <span><AppIcon name="image" size="20" /></span>
        </button>
        <input ref="avatarInput" class="rf-visually-hidden" type="file" accept="image/jpeg,image/png" @change="chooseImage($event, 'avatar')" />
        <input ref="coverInput" class="rf-visually-hidden" type="file" accept="image/jpeg,image/png" @change="chooseImage($event, 'cover')" />
      </section>

      <section class="rf-x-edit-fields">
        <label class="rf-x-edit-field">
          <span>名称</span>
          <small>{{ form.display_name.length }}/80</small>
          <input v-model="form.display_name" maxlength="80" autocomplete="nickname" required />
        </label>
        <label class="rf-x-edit-field rf-x-edit-bio">
          <span>个人简介</span>
          <small>{{ form.bio.length }}/500</small>
          <textarea v-model="form.bio" rows="5" maxlength="500" />
        </label>
        <div class="rf-x-edit-field readonly">
          <span>账号邮箱</span>
          <input :value="auth.user?.email" readonly tabindex="-1" />
        </div>
      </section>

      <button type="button" class="rf-x-verification-row" @click="router.push('/verification/apply')">
        <span>
          <strong>{{ auth.user?.blue_verified ? (auth.user.verification_label || '已认证') : '认证' }}</strong>
          <VerifiedBadge :verified="auth.user?.blue_verified" :label="auth.user?.verification_label" /><MembershipBadge :active="auth.user?.member_active" :tier-id="auth.user?.membership_tier_id" />
        </span>
        <AppIcon name="arrow" size="17" />
      </button>
    </form>
  </PageContainer>
</template>

<style scoped>
.rf-x-edit-profile { min-width: 0; min-height: 100vh; background: var(--rf-bg); }
.rf-x-edit-header { position: sticky; top: 0; z-index: 12; display: grid; min-height: 56px; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 5px 16px; border-bottom: 1px solid var(--rf-line); background: color-mix(in srgb, var(--rf-bg) 88%, transparent); backdrop-filter: blur(12px); }
.rf-x-edit-header > strong { overflow: hidden; font-size: 20px; text-overflow: ellipsis; white-space: nowrap; }
.rf-x-edit-back { display: inline-grid; width: 40px; height: 40px; place-items: center; border-radius: 50%; color: var(--rf-text); background: transparent; }
.rf-x-edit-back:hover { background: var(--rf-bg-hover); }
.rf-x-edit-save { min-width: 72px; min-height: 34px; padding: 0 17px; border-radius: var(--rf-pill); color: var(--rf-bg); background: var(--rf-text); font-size: 14px; font-weight: 700; }
.rf-x-edit-save:disabled { cursor: not-allowed; opacity: .45; }
.rf-x-edit-media { position: relative; padding-bottom: 58px; }
.rf-x-edit-cover { position: relative; width: 100%; aspect-ratio: 3 / 1; overflow: hidden; background: #cfd9de; }
.rf-x-edit-cover img { display: block; width: 100%; height: 100%; object-fit: cover; }
.rf-x-edit-cover-controls { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; gap: 14px; background: rgba(15, 20, 25, .18); }
.rf-x-edit-cover.empty .rf-x-edit-cover-controls { background: transparent; }
.rf-x-edit-cover-controls button, .rf-x-edit-avatar > span { display: inline-grid; width: 42px; height: 42px; place-items: center; border-radius: 50%; color: #fff; background: rgba(15, 20, 25, .72); backdrop-filter: blur(6px); transition: background-color 150ms ease-out, transform 100ms ease-out; }
.rf-x-edit-cover-controls button:hover, .rf-x-edit-avatar:hover > span { background: rgba(15, 20, 25, .86); }
.rf-x-edit-cover-controls button:active, .rf-x-edit-avatar:active > span { transform: scale(.94); }
.rf-x-edit-avatar { position: absolute; bottom: 0; left: 16px; width: 120px; height: 120px; padding: 4px; border-radius: 50%; background: var(--rf-bg); }
.rf-x-edit-avatar :deep(.rf-user-avatar) { width: 112px !important; height: 112px !important; flex-basis: 112px !important; }
.rf-x-edit-avatar > span { position: absolute; top: 50%; left: 50%; width: 42px; height: 42px; transform: translate(-50%, -50%); }
.rf-x-edit-avatar:active > span { transform: translate(-50%, -50%) scale(.94); }
.rf-x-edit-fields { display: flex; flex-direction: column; gap: 16px; padding: 18px 16px; }
.rf-x-edit-field { position: relative; display: block; min-width: 0; }
.rf-x-edit-field > span { position: absolute; z-index: 1; top: 7px; left: 12px; color: var(--rf-muted); font-size: 12px; pointer-events: none; }
.rf-x-edit-field > small { position: absolute; z-index: 1; top: 7px; right: 12px; color: var(--rf-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.rf-x-edit-field input, .rf-x-edit-field textarea { width: 100%; min-width: 0; border: 1px solid var(--rf-faint); border-radius: 4px; outline: 0; color: var(--rf-text); background: var(--rf-bg); font: inherit; transition: border-color 150ms ease-out, box-shadow 150ms ease-out; }
.rf-x-edit-field input { height: 58px; padding: 22px 12px 5px; }
.rf-x-edit-field textarea { min-height: 116px; padding: 23px 12px 9px; line-height: 1.45; resize: vertical; }
.rf-x-edit-field input:focus, .rf-x-edit-field textarea:focus { border-color: var(--primary); box-shadow: 0 0 0 1px var(--primary); }
.rf-x-edit-field:focus-within > span { color: var(--primary); }
.rf-x-edit-field.readonly input { color: var(--rf-muted); background: var(--rf-bg-subtle); }
.rf-x-verification-row { display: flex; width: 100%; min-height: 54px; align-items: center; justify-content: space-between; gap: 12px; padding: 0 16px; border-top: 1px solid var(--rf-line); border-bottom: 1px solid var(--rf-line); color: var(--rf-text); background: transparent; text-align: left; }
.rf-x-verification-row:hover { background: var(--rf-bg-hover); }
.rf-x-verification-row > span { display: inline-flex; align-items: center; gap: 6px; }
.rf-visually-hidden { position: absolute !important; width: 1px !important; height: 1px !important; padding: 0 !important; overflow: hidden !important; clip: rect(0, 0, 0, 0) !important; white-space: nowrap !important; border: 0 !important; }

@media (max-width: 560px) {
  .rf-x-edit-header { padding-inline: 10px 12px; }
  .rf-x-edit-media { padding-bottom: 46px; }
  .rf-x-edit-avatar { left: 12px; width: 96px; height: 96px; padding: 3px; }
  .rf-x-edit-avatar :deep(.rf-user-avatar) { width: 90px !important; height: 90px !important; flex-basis: 90px !important; }
  .rf-x-edit-fields { padding: 16px 12px; }
}
</style>
