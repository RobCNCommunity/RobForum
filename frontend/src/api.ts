import axios from 'axios'

export const api = axios.create({ baseURL: '/api/v1', withCredentials: true, timeout: 15000 })

function readCookie(name: string) {
  if (typeof document === 'undefined') return ''
  const prefix = `${name}=`
  const value = document.cookie.split('; ').find((item) => item.startsWith(prefix))
  return value ? decodeURIComponent(value.slice(prefix.length)) : ''
}

api.interceptors.request.use((config) => {
  const method = String(config.method || 'get').toLowerCase()
  if (typeof document !== 'undefined' && !['get', 'head', 'options'].includes(method)) {
    const token = readCookie('roblox_csrf')
    if (token) config.headers.set('X-CSRF-Token', token)
  }
  return config
})

api.interceptors.response.use(undefined, async (error) => {
  const status = Number(error?.response?.status || 0)
  const code = String(error?.response?.data?.error?.code || '')
  const config = error?.config as (Record<string, any> & { __csrfRetried?: boolean }) | undefined

  if (status === 403 && ['csrf_token_required', 'csrf_token_invalid'].includes(code) && config && !config.__csrfRetried) {
    config.__csrfRetried = true
    try {
      await api.get('/site/settings', { headers: { 'Cache-Control': 'no-cache' } })
      const token = readCookie('roblox_csrf')
      if (token) {
        config.headers?.set?.('X-CSRF-Token', token)
        return api.request(config)
      }
    } catch {
      // Preserve the original mutation error below.
    }
  }

  if (status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('robforum:session-expired'))
  }
  return Promise.reject(error)
})

export interface SiteSettings {
  site_name: string
  site_description: string
  logo_url: string
  avatar_url: string
  verification_badge_url: string
  primary_color: string
  public_url: string
  allow_register: boolean
  require_email_verification: boolean
  post_review_required: boolean
  allowed_email_domains: string[]
  banner_enabled: boolean
  banner_text: string
  banner_link: string
  updated_at: string
}

export interface User {
  id: number
  email: string
  display_name: string
  avatar_url: string
  cover_url: string
  bio: string
  role: string
  status: string
  blue_verified: boolean
  verification_label?: string
  roblox_name?: string
  roblox_id?: string
  roblox_verified: boolean
  created_at: string
}

export interface Board { id: number; slug: string; name: string; description: string; icon: string; post_count: number }
export interface PostMedia { id: number; url: string; mime_type: string; width: number; height: number; size_bytes: number }
export interface Post { id: number; board_id: number; board_name: string; author_id: number; author_name: string; author_avatar: string; author_verified: boolean; author_verification_label?: string; title: string; content: string; post_type: string; status: string; pinned: boolean; featured: boolean; views: number; comment_count: number; like_count: number; repost_count: number; liked: boolean; bookmarked: boolean; reposted: boolean; media?: PostMedia[]; created_at: string; updated_at: string }
export interface Comment { id: number; post_id: number; author_id: number; author_name: string; author_avatar: string; author_verified: boolean; author_verification_label?: string; content: string; like_count: number; liked: boolean; created_at: string }
export interface ResourceFile { id: number; resource_id: number; original_name: string; mime_type: string; size_bytes: number; sha256: string; created_at: string }
export interface Resource { id: number; creator_id: number; creator_name: string; creator_verified: boolean; creator_verification_label?: string; title: string; description: string; game: string; version: string; resource_type: string; price_cents: number; status: string; review_reason?: string; download_count: number; sales_count: number; file?: ResourceFile; created_at: string; updated_at: string }
export interface PublicUser { id: number; display_name: string; avatar_url: string; cover_url: string; bio: string; blue_verified: boolean; verification_label?: string; roblox_name?: string; roblox_verified: boolean; created_at: string }
export interface AdminUser { id: number; email: string; display_name: string; avatar_url: string; role: string; status: string; blue_verified: boolean; verification_label?: string; post_count: number; comment_count: number; resource_count: number; created_at: string; updated_at: string }
export interface UserProfile { user: PublicUser; posts: Post[]; resources: Resource[]; follower_count: number; following_count: number; following: boolean }
export interface UserSearchResult extends PublicUser { post_count: number; resource_count: number; hot_score: number }
export interface ChatMessage { id: number; conversation_id: number; sender_id: number; sender_name: string; sender_avatar: string; content: string; created_at: string }
export interface Conversation { id: number; kind: 'direct' | 'group'; name: string; created_by: number; members: PublicUser[]; last_message?: ChatMessage; unread_count: number; created_at: string; updated_at: string }
export interface LikeResult { liked: boolean; like_count: number }
export interface FollowStatus { following: boolean; follower_count: number; following_count: number }
export interface RepostResult { reposted: boolean; repost_count: number }
export interface NotificationItem { id: number; kind: string; actor_id: number; actor_name: string; actor_avatar: string; post_id?: number; comment_id?: number; post_title?: string; read: boolean; created_at: string }
export interface VerificationApplication { id: number; user_id: number; user_name: string; user_email?: string; verification_type: string; requested_label: string; evidence_url: string; statement: string; status: string; review_note?: string; reviewed_by?: number; reviewed_at?: string; created_at: string; updated_at: string }
export interface PaymentConfig { enabled: boolean; kind: string; gateway_url: string; merchant_id: string; secret?: string; has_secret: boolean; masked_secret?: string; pay_type: string; notify_url: string; return_url: string }
export interface CommerceOrder { id: number; order_no: string; user_id: number; resource_id: number; resource_title: string; amount_cents: number; status: string; gateway_trade_no?: string; payment_url?: string; paid_at?: string; created_at: string }
export interface CreatorPayout { id: number; creator_id: number; creator_name?: string; creator_email?: string; amount_cents: number; status: string; payout_method: string; payout_account: string; account_name: string; note: string; review_note?: string; reviewed_by?: number; reviewed_at?: string; created_at: string; updated_at: string }
export interface SMTPConfig { enabled: boolean; host: string; port: number; username: string; password?: string; has_password: boolean; masked_password?: string; from_email: string; from_name: string; tls_mode: string }
export interface CaptchaConfig { enabled: boolean; provider: 'gt4' | 'gt3' | 'aliyun'; site_key: string; endpoint: string; has_secret: boolean; masked_secret?: string; secret?: string; adapter_status: string }
export interface CaptchaUpdate { enabled: boolean; provider: 'gt4' | 'gt3' | 'aliyun'; site_key: string; endpoint: string; masked_secret?: string }
export interface PublicOAuthConfig { enabled: boolean; provider_name: string }
export interface OAuthConfig {
  enabled: boolean
  provider_key: string
  provider_name: string
  client_id: string
  client_secret?: string
  has_client_secret: boolean
  masked_client_secret?: string
  clear_client_secret?: boolean
  authorization_url: string
  token_url: string
  userinfo_url: string
  scopes: string
  token_auth_method: 'client_secret_post' | 'client_secret_basic'
  require_verified_email: boolean
  updated_at: string
}

export interface AdSlot {
  id: number
  title: string
  image_url: string
  link_url: string
  sort_order: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface Notice {
  id: number
  title: string
  content: string
  level: string
  pinned: boolean
  enabled: boolean
  created_by?: number
  created_at: string
  updated_at: string
}

export interface WalletEntry {
  id: number
  user_id: number
  entry_type: string
  amount_cents: number
  reference_type: string
  reference_id: number
  note: string
  created_at: string
}

export interface WalletSummary {
  available_cents: number
  entries: WalletEntry[]
}

function data<T>(response: { data: { data: T } }): T { return response.data.data }

export async function fetchSiteSettings() { return data<SiteSettings>(await api.get('/site/settings')) }
export async function fetchPublicCaptcha() { return data<CaptchaConfig>(await api.get('/captcha/config')) }
export async function fetchPublicOAuth() { return data<PublicOAuthConfig>(await api.get('/oauth/config')) }
export async function fetchBoards() { return data<Board[]>(await api.get('/boards')) }
export async function fetchPosts(params?: { board?: string; q?: string }) { return data<Post[]>(await api.get('/posts', { params })) }
export async function fetchFollowingPosts() { return data<Post[]>(await api.get('/me/feed/following')) }
export async function fetchBookmarkedPosts() { return data<Post[]>(await api.get('/me/bookmarks')) }
export async function fetchPost(id: number) { return data<Post>(await api.get(`/posts/${id}`)) }
export async function fetchComments(id: number) { return data<Comment[]>(await api.get(`/posts/${id}/comments`)) }
export async function createPost(input: { board_id: number; title: string; content: string; post_type: string; files?: File[] }) { const form = new FormData(); form.append('board_id', String(input.board_id)); form.append('title', input.title); form.append('content', input.content); form.append('post_type', input.post_type); for (const file of input.files || []) form.append('files', file); return data<Post>(await api.post('/posts', form)) }
export async function createComment(id: number, content: string) { return data<Comment>(await api.post(`/posts/${id}/comments`, { content })) }
export async function fetchResources() { return data<Resource[]>(await api.get('/resources')) }
export async function fetchMyResources() { return data<Resource[]>(await api.get('/me/resources')) }
export async function createResource(input: { title: string; description: string; game: string; version: string; resource_type: string; price_cents: number; file: File }) { const form = new FormData(); form.append('title', input.title); form.append('description', input.description); form.append('game', input.game); form.append('version', input.version); form.append('resource_type', input.resource_type); form.append('price_cents', String(input.price_cents)); form.append('file', input.file); return data<Resource>(await api.post('/resources', form)) }
export async function fetchAdminResources(status = 'pending') { return data<Resource[]>(await api.get('/admin/resources', { params: { status } })) }
export async function reviewResource(id: number, status: string, reason = '') { return data<Resource>(await api.patch(`/admin/resources/${id}`, { status, reason })) }
export async function createResourceOrder(id: number) { return data<CommerceOrder>(await api.post(`/resources/${id}/orders`)) }
export async function fetchMyOrders() { return data<CommerceOrder[]>(await api.get('/me/orders')) }
export async function fetchCreatorBalance() { return data<{ available_cents: number; platform_fee_percent: number }>(await api.get('/me/creator/balance')) }
export async function fetchCreatorPayouts() { return data<CreatorPayout[]>(await api.get('/me/creator/payouts')) }
export async function requestCreatorPayout(input: { amount_cents: number; payout_method: string; payout_account: string; account_name: string; note: string }) { return data<CreatorPayout>(await api.post('/me/creator/payouts', input)) }
export async function fetchPaymentConfig() { return data<PaymentConfig>(await api.get('/admin/payment')) }
export async function updatePaymentConfig(input: PaymentConfig) { return data<PaymentConfig>(await api.put('/admin/payment', input)) }
export async function fetchAdminPayouts(status = 'pending') { return data<CreatorPayout[]>(await api.get('/admin/payouts', { params: { status } })) }
export async function reviewCreatorPayout(id: number, status: 'paid' | 'rejected', note = '') { return data<CreatorPayout>(await api.patch(`/admin/payouts/${id}`, { status, note })) }
export async function register(input: { email: string; password: string; display_name: string; captcha_token?: string; email_code?: string }) { return data<{ user: User }>(await api.post('/auth/register', input)) }
export async function sendRegistrationVerification(email: string, captcha_token = '') { return data<{ sent: boolean; expires_in: number }>(await api.post('/auth/register/verification', { email, captcha_token })) }
export async function login(input: { email: string; password: string; captcha_token?: string }) { return data<{ user: User }>(await api.post('/auth/login', input)) }
export async function logout() { return data<{ logged_out: boolean }>(await api.post('/auth/logout')) }
export async function fetchMe() { return data<User>(await api.get('/me')) }
export async function updateMyProfile(input: { display_name: string; bio: string }) { return data<User>(await api.patch('/me/profile', input)) }
export async function uploadMyAvatar(file: File) { const form = new FormData(); form.append('file', file); return data<User>(await api.post('/me/avatar', form)) }
export async function uploadMyCover(file: File) { const form = new FormData(); form.append('file', file); return data<User>(await api.post('/me/cover', form)) }
export async function deleteMyCover() { return data<User>(await api.delete('/me/cover')) }
export async function fetchUserProfile(id: number) { return data<UserProfile>(await api.get(`/users/${id}`)) }
export async function searchUsers(q = '') { return data<UserSearchResult[]>(await api.get('/users/search', { params: { q } })) }
export async function fetchHotUsers() { return data<UserSearchResult[]>(await api.get('/users/hot')) }
export async function fetchBlockStatus(id: number) { return data<{ blocked: boolean }>(await api.get(`/users/${id}/block`)) }
export async function setUserBlocked(id: number, blocked: boolean) { return data<{ blocked: boolean }>(blocked ? await api.post(`/users/${id}/block`) : await api.delete(`/users/${id}/block`)) }
export async function fetchFollowStatus(id: number) { return data<FollowStatus>(await api.get(`/users/${id}/follow`)) }
export async function setUserFollowing(id: number, following: boolean) { return data<FollowStatus>(following ? await api.post(`/users/${id}/follow`) : await api.delete(`/users/${id}/follow`)) }
export async function deleteComment(id: number) { return data<{ deleted: boolean }>(await api.delete(`/comments/${id}`)) }
export async function fetchPostLike(id: number) { return data<LikeResult>(await api.get(`/posts/${id}/like`)) }
export async function togglePostLike(id: number) { return data<LikeResult>(await api.post(`/posts/${id}/like`)) }
export async function fetchCommentLike(id: number) { return data<LikeResult>(await api.get(`/comments/${id}/like`)) }
export async function toggleCommentLike(id: number) { return data<LikeResult>(await api.post(`/comments/${id}/like`)) }
export async function fetchBookmarkStatus(id: number) { return data<{ bookmarked: boolean }>(await api.get(`/posts/${id}/bookmark`)) }
export async function togglePostBookmark(id: number) { return data<{ bookmarked: boolean }>(await api.post(`/posts/${id}/bookmark`)) }
export async function fetchRepostStatus(id: number) { return data<RepostResult>(await api.get(`/posts/${id}/repost`)) }
export async function togglePostRepost(id: number) { return data<RepostResult>(await api.post(`/posts/${id}/repost`)) }
export async function fetchNotifications() { return data<NotificationItem[]>(await api.get('/me/notifications')) }
export async function fetchUnreadNotificationCount() { return data<{ count: number }>(await api.get('/me/notifications/unread-count')) }
export async function markNotificationsRead() { return data<{ read: boolean }>(await api.post('/me/notifications/read')) }
export async function fetchConversations() { return data<Conversation[]>(await api.get('/me/conversations')) }
export async function createConversation(input: { kind: 'direct' | 'group'; name?: string; member_ids: number[] }) { return data<Conversation>(await api.post('/conversations', input)) }
export async function fetchMessages(id: number) { return data<ChatMessage[]>(await api.get(`/conversations/${id}/messages`)) }
export async function sendMessage(id: number, content: string) { return data<ChatMessage>(await api.post(`/conversations/${id}/messages`, { content })) }
export async function fetchMyVerificationApplications() { return data<VerificationApplication[]>(await api.get('/me/verification-applications')) }
export async function createVerificationApplication(input: { verification_type: string; requested_label: string; evidence_url: string; statement: string }) { return data<VerificationApplication>(await api.post('/me/verification-applications', input)) }
export async function fetchAdminVerifications(status = 'pending') { return data<VerificationApplication[]>(await api.get('/admin/verifications', { params: { status } })) }
export async function reviewVerificationApplication(id: number, status: 'approved' | 'rejected', label = '', note = '') { return data<VerificationApplication>(await api.patch(`/admin/verifications/${id}`, { status, label, note })) }
export async function fetchAdminUsers(params?: { q?: string; status?: string }) { return data<AdminUser[]>(await api.get('/admin/users', { params })) }
export async function updateAdminUserStatus(id: number, status: 'active' | 'banned', reason: string) { return data<User>(await api.patch(`/admin/users/${id}/status`, { status, reason })) }
export async function deleteAdminUser(id: number, reason: string) { return data<{ deleted: boolean }>(await api.delete(`/admin/users/${id}`, { data: { reason } })) }
export async function fetchAdminPosts(params?: { status?: string; q?: string; post_id?: number }) { return data<Post[]>(await api.get('/admin/posts', { params })) }
export async function moderateAdminPost(id: number, status: 'published' | 'rejected' | 'hidden' | 'deleted', reason = '') { return data<Post>(await api.patch(`/admin/posts/${id}/moderate`, { status, reason })) }
export async function forgotPassword(email: string, captcha_token = '') { return data<{ message: string }>(await api.post('/auth/forgot-password', { email, captcha_token })) }
export async function resetPassword(token: string, password: string) { return data<{ reset: boolean }>(await api.post('/auth/reset-password', { token, password })) }
export async function fetchAdminSite() { return data<SiteSettings>(await api.get('/admin/site')) }
export async function updateAdminSite(input: SiteSettings) { return data<SiteSettings>(await api.put('/admin/site', input)) }
export async function deleteVerificationBadge() { return data<SiteSettings>(await api.delete('/admin/site/verification-badge')) }
export async function fetchSMTP() { return data<SMTPConfig>(await api.get('/admin/smtp')) }
export async function updateSMTP(input: SMTPConfig) { return data<SMTPConfig>(await api.put('/admin/smtp', input)) }
export async function testSMTP(email: string) { return data<{ sent: boolean }>(await api.post('/admin/smtp/test', { email })) }
export async function fetchCaptcha() { return data<CaptchaConfig>(await api.get('/admin/captcha')) }
export async function updateCaptcha(input: CaptchaUpdate) { return data<CaptchaConfig>(await api.put('/admin/captcha', input)) }
export async function fetchOAuth() { return data<OAuthConfig>(await api.get('/admin/oauth')) }
export async function updateOAuth(input: OAuthConfig) { return data<OAuthConfig>(await api.put('/admin/oauth', input)) }
export function resourceDownloadURL(id: number) { return `/api/v1/resources/${id}/download` }


export async function fetchAds() { return data<AdSlot[]>(await api.get('/ads')) }
export async function fetchAdminAds() { return data<AdSlot[]>(await api.get('/admin/ads')) }
export async function createAdminAd(input: { title?: string; image_url?: string; link_url?: string; sort_order?: number; enabled?: boolean }) { return data<AdSlot>(await api.post('/admin/ads', input)) }
export async function updateAdminAd(id: number, input: { title?: string; image_url?: string; link_url?: string; sort_order?: number; enabled?: boolean }) { return data<AdSlot>(await api.put('/admin/ads/' + id, input)) }
export async function deleteAdminAd(id: number) { return data<{ deleted: boolean }>(await api.delete('/admin/ads/' + id)) }
export async function fetchNotices() { return data<Notice[]>(await api.get('/notices')) }
export async function fetchAdminNotices() { return data<Notice[]>(await api.get('/admin/notices')) }
export async function createAdminNotice(input: { title: string; content: string; level?: string; pinned?: boolean; enabled?: boolean }) { return data<Notice>(await api.post('/admin/notices', input)) }
export async function updateAdminNotice(id: number, input: { title?: string; content?: string; level?: string; pinned?: boolean; enabled?: boolean }) { return data<Notice>(await api.put('/admin/notices/' + id, input)) }
export async function deleteAdminNotice(id: number) { return data<{ deleted: boolean }>(await api.delete('/admin/notices/' + id)) }
export async function pinPost(id: number, pinned: boolean) { return data<Post>(await api.patch('/admin/posts/' + id + '/pin', { pinned })) }
export async function fetchWallet() { return data<WalletSummary>(await api.get('/me/wallet')) }
export function errorMessage(error: any, fallback = '请求失败，请稍后重试') { return error?.response?.data?.error?.message || fallback }
