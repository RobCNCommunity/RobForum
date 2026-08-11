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
  user_agreement_url: string
  cookies_policy_url: string
  allow_register: boolean
  require_email_verification: boolean
  post_review_required: boolean
  allowed_email_domains: string[]
  banner_enabled: boolean
  banner_text: string
  banner_link: string
  updated_at: string
}

export interface AvatarFrame {
  id: number
  name: string
  description: string
  style: 'ring' | 'double' | 'glow' | 'pixel' | 'halo' | 'image'
  image_url: string
  primary_color: string
  secondary_color: string
  price_cents: number
  allowed_regular: boolean
  allowed_member: boolean
  allowed_admin: boolean
  enabled: boolean
  sort_order: number
  sales_count: number
  owned: boolean
  equipped: boolean
  can_use: boolean
  purchased_at?: string
  created_at: string
  updated_at: string
}

export interface AvatarFrameUploadSettings {
  allow_regular_upload: boolean
  allow_member_upload: boolean
  can_upload: boolean
  updated_at: string
}

export interface AvatarFrameSubmission {
  id: number
  user_id: number
  user_name: string
  user_avatar: string
  name: string
  description: string
  image_url: string
  status: 'pending' | 'approved' | 'rejected'
  review_note: string
  reviewed_by?: number
  reviewed_at?: string
  approved_frame_id?: number
  created_at: string
  updated_at: string
}

export interface User {
  id: number
  custom_uid?: string
  email: string
  display_name: string
  avatar_url: string
  cover_url: string
  bio: string
  role: string
  status: string
  blue_verified: boolean
  verification_label?: string
  member_active: boolean
  membership_tier_id?: number
  membership_started_at?: string
  membership_expires_at?: string
  avatar_frame?: AvatarFrame
  roblox_name?: string
  roblox_id?: string
  roblox_verified: boolean
  two_factor_enabled: boolean
  created_at: string
}

export interface Badge { id: number; slug: string; name: string; description: string; icon: string; color: string; awarded_at: string }
export interface UserProgress { experience: number; total_checkins: number; current_streak: number; longest_streak: number; last_checkin_date?: string; checked_in_today: boolean; level: number; level_name: string; level_min_experience: number; next_level_experience: number; level_progress: number }
export interface CheckinRecord { date: string; experience_awarded: number; streak: number; created_at: string }
export interface CheckinSummary { progress: UserProgress; history: CheckinRecord[]; badges: Badge[]; newly_awarded: Badge[] }
export interface PointAccount { user_id: number; balance: number; total_earned: number; total_spent: number; updated_at: string; user_name?: string; user_avatar?: string }
export interface PointLedger { id: number; user_id: number; amount: number; type: string; source_id?: number; balance_after: number; note: string; created_at: string }
export interface PointSummary { account: PointAccount; entries: PointLedger[] }
export interface PointProduct { id: number; name: string; image_url: string; points_required: number; shipping_points: number; stock: number; physical: boolean; description: string; status: string; sort_order: number; created_at: string; updated_at: string }
export interface ShippingAddress { name: string; phone: string; province: string; city: string; district: string; detail: string }
export interface PointOrder { id: number; order_no: string; user_id: number; user_name?: string; product_id: number; product_name: string; product_image: string; points_spent: number; shipping_points: number; physical: boolean; address?: ShippingAddress; carrier?: string; tracking_no?: string; status: string; note: string; ordered_at: string; updated_at: string }
export interface PointTracking { order_id: number; order_no: string; carrier: string; tracking_no: string; order_status: string; query_url: string; updated_at: string }
export type LotteryPrizeType = 'points' | 'membership' | 'wallet' | 'physical' | 'custom'
export interface LotteryPrize { id: number; activity_id: number; name: string; image_url: string; probability_bp: number; stock: number; prize_type: LotteryPrizeType; bound_id?: number; config_json: string; physical: boolean; sort_order: number; created_at: string; updated_at: string }
export interface LotteryActivity { id: number; name: string; cover_url: string; description: string; style: 'wheel' | 'scratch'; cost_points: number; daily_limit: number; total_limit: number; daily_free_attempts: number; starts_at: string; ends_at: string; status: string; sort_order: number; prizes: LotteryPrize[]; created_at: string; updated_at: string }
export interface LotteryWin { id: number; user_id: number; user_name?: string; activity_id: number; activity_name?: string; prize_id: number; prize_name: string; prize_type: LotteryPrizeType; physical: boolean; status: string; order_id?: number; address?: ShippingAddress; delivered_at?: string; won_at: string }
export interface LotteryDrawResult { won: boolean; prize?: LotteryPrize; win?: LotteryWin; points_charged: number; balance: number }
export interface LotteryActivityStats { activity_id: number; activity_name: string; draw_count: number; unique_users: number; win_count: number; delivered_count: number; pending_count: number; points_spent: number; win_rate_bp: number }
export interface PointsDashboard { account_count: number; total_balance: number; total_earned: number; total_spent: number; product_count: number; order_count: number; pending_orders: number; draw_count: number; win_count: number; pending_wins: number; activities: LotteryActivityStats[] }
export interface RobloxMusic { asset_id: number; name: string; description?: string; creator_name?: string; thumbnail_url?: string; favorited: boolean; created_at?: string }

export interface Board { id: number; slug: string; name: string; description: string; icon: string; post_count: number }
export interface PostMedia { id: number; url: string; mime_type: string; width: number; height: number; size_bytes: number }
export interface Post { id: number; board_id: number; board_name: string; author_id: number; author_name: string; author_avatar: string; author_avatar_frame?: AvatarFrame; author_verified: boolean; author_verification_label?: string; author_member: boolean; author_membership_tier_id?: number; title: string; content: string; status: string; pinned: boolean; featured: boolean; views: number; comment_count: number; like_count: number; repost_count: number; liked: boolean; bookmarked: boolean; reposted: boolean; tags?: string[]; media?: PostMedia[]; created_at: string; updated_at: string }
export interface Comment { id: number; post_id: number; parent_id?: number; author_id: number; author_name: string; author_avatar: string; author_avatar_frame?: AvatarFrame; author_verified: boolean; author_verification_label?: string; author_member: boolean; author_membership_tier_id?: number; content: string; media?: PostMedia[]; like_count: number; liked: boolean; created_at: string }
export interface ResourceFile { id: number; resource_id: number; original_name: string; mime_type: string; size_bytes: number; sha256: string; created_at: string }
export interface ResourceMedia { id: number; url: string; mime_type: string; width: number; height: number; size_bytes: number }
export interface Resource { id: number; creator_id: number; creator_name: string; creator_verified: boolean; creator_verification_label?: string; creator_member: boolean; creator_membership_tier_id?: number; title: string; description: string; game: string; version: string; resource_type: string; price_cents: number; status: string; review_reason?: string; download_count: number; sales_count: number; file?: ResourceFile; media?: ResourceMedia[]; created_at: string; updated_at: string }
export interface PublicUser { id: number; custom_uid?: string; display_name: string; avatar_url: string; cover_url: string; bio: string; blue_verified: boolean; verification_label?: string; member_active: boolean; membership_tier_id?: number; avatar_frame?: AvatarFrame; roblox_name?: string; roblox_verified: boolean; created_at: string; progress: UserProgress; badges: Badge[] }
export interface AdminUser { id: number; email: string; display_name: string; avatar_url: string; role: string; status: string; blue_verified: boolean; verification_label?: string; member_active: boolean; membership_tier_id?: number; membership_expires_at?: string; post_count: number; comment_count: number; resource_count: number; created_at: string; updated_at: string }
export interface UserProfile { user: PublicUser; posts: Post[]; resources: Resource[]; follower_count: number; following_count: number; following: boolean }
export interface UserSearchResult extends PublicUser { post_count: number; resource_count: number; hot_score: number }
export interface CommunitySearchResult { query: string; posts: Post[]; resources: Resource[]; users: UserSearchResult[] }
export interface ChatMessage { id: number; conversation_id: number; sender_id: number; sender_name: string; sender_avatar: string; content: string; created_at: string }
export interface Conversation { id: number; kind: 'direct' | 'group'; name: string; created_by: number; members: PublicUser[]; last_message?: ChatMessage; unread_count: number; membership_status: 'accepted' | 'pending' | 'declined'; accepted_member_count: number; pending_invite_count: number; active: boolean; created_at: string; updated_at: string }
export interface ConversationInvite { conversation: Conversation; invited_at: string }
export interface ConversationMember extends PublicUser { membership_status: 'accepted' | 'pending'; invited_by?: number; joined_at: string; responded_at?: string }
export interface ConversationInviteLink { token: string; join_url: string; created_at: string; expires_at: string }
export interface PagedResult<T> { items: T[]; next_offset: number; has_more: boolean }
export interface LikeResult { liked: boolean; like_count: number }
export interface FollowStatus { following: boolean; follower_count: number; following_count: number }
export interface RepostResult { reposted: boolean; repost_count: number }
export interface NotificationItem { id: number; kind: string; actor_id: number; actor_name: string; actor_avatar: string; post_id?: number; comment_id?: number; conversation_id?: number; post_title?: string; conversation_name?: string; read: boolean; created_at: string }
export interface ContentReport { id: number; reporter_id: number; reporter_name: string; reporter_avatar: string; target_type: 'post' | 'comment' | 'profile'; target_id: number; target_user_id: number; target_user_name: string; target_user_avatar: string; target_post_id?: number; target_title: string; target_content: string; target_status: string; reason: string; status: 'pending' | 'accepted' | 'rejected'; review_note?: string; reviewed_by?: number; reviewer_name?: string; reviewed_at?: string; created_at: string }
export interface VerificationApplication { id: number; user_id: number; user_name: string; user_email?: string; verification_type: string; requested_label: string; evidence_url: string; statement: string; status: string; review_note?: string; reviewed_by?: number; reviewed_at?: string; created_at: string; updated_at: string }
export interface PaymentConfig { enabled: boolean; kind: string; gateway_url: string; merchant_id: string; secret?: string; has_secret: boolean; masked_secret?: string; pay_type: string; notify_url: string; return_url: string }
export interface CommerceOrder { id: number; order_no: string; user_id: number; resource_id: number; resource_title: string; amount_cents: number; service_fee_bps: number; service_fee_cents: number; creator_share_cents: number; seller_membership_tier_id?: number; seller_membership_tier_name?: string; status: string; gateway_trade_no?: string; payment_url?: string; paid_at?: string; created_at: string }
export interface CreatorPayout { id: number; creator_id: number; creator_name?: string; creator_email?: string; amount_cents: number; withdrawal_fee_bps: number; withdrawal_fee_cents: number; net_amount_cents: number; membership_tier_id?: number; membership_tier_name?: string; status: string; payout_method: string; payout_account: string; account_name: string; note: string; review_note?: string; reviewed_by?: number; reviewed_at?: string; created_at: string; updated_at: string }
export interface SMTPConfig { enabled: boolean; host: string; port: number; username: string; password?: string; has_password: boolean; masked_password?: string; from_email: string; from_name: string; tls_mode: string }
export interface CaptchaConfig { enabled: boolean; provider: 'gt4' | 'gt3' | 'aliyun'; site_key: string; endpoint: string; has_secret: boolean; masked_secret?: string; secret?: string; adapter_status: string }
export interface CaptchaUpdate { enabled: boolean; provider: 'gt4' | 'gt3' | 'aliyun'; site_key: string; endpoint: string; masked_secret?: string }
export interface PublicOAuthConfig { id: number; enabled: boolean; provider_key: string; provider_name: string }
export interface OAuthConfig {
  id: number
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
  link_url: string
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
  withdrawable_cents: number
  fee_policy: FeePolicy
  entries: WalletEntry[]
  top_ups: WalletTopUpOrder[]
}
export interface WalletTopUpOrder { id: number; order_no: string; user_id: number; amount_cents: number; status: string; gateway_trade_no?: string; payment_url?: string; paid_at?: string; created_at: string }
export interface RedeemCode { id: number; code?: string; code_hint: string; amount_cents: number; status: string; expires_at?: string; redeemed_by?: number; redeemed_by_name?: string; redeemed_at?: string; revoked_at?: string; created_by: number; created_at: string }
export interface FeePolicy { membership_tier_id?: number; membership_tier_name?: string; withdrawal_fee_bps: number; service_fee_bps: number }
export interface MembershipTier { id: number; enabled: boolean; name: string; badge_label: string; badge_url: string; badge_color: string; monthly_price_cents: number; quarterly_price_cents: number; yearly_price_cents: number; post_review_exempt: boolean; feed_priority: boolean; withdrawal_fee_bps: number; service_fee_bps: number; sort_order: number; created_at: string; updated_at: string }
export interface MembershipSettings { enabled: boolean; default_withdrawal_fee_bps: number; default_service_fee_bps: number; tiers: MembershipTier[]; name?: string; badge_label?: string; badge_url?: string; badge_color?: string; monthly_price_cents?: number; quarterly_price_cents?: number; yearly_price_cents?: number; post_review_exempt?: boolean; updated_at: string }
export interface MembershipOrder { id: number; order_no: string; user_id: number; membership_tier_id?: number; tier_name: string; plan: 'monthly' | 'quarterly' | 'yearly'; amount_cents: number; duration_months: number; started_at: string; expires_at: string; status: string; created_at: string; updated_at: string }
export interface MembershipSummary { config: MembershipSettings; active: boolean; current_tier?: MembershipTier; started_at?: string; expires_at?: string; available_cents: number; orders: MembershipOrder[] }

function data<T>(response: { data: { data: T } }): T { return response.data.data }

export async function fetchSiteSettings() { return data<SiteSettings>(await api.get('/site/settings')) }
export async function fetchMembershipConfig() { return data<MembershipSettings>(await api.get('/membership/config')) }
export async function fetchAvatarFrames() { return data<AvatarFrame[]>(await api.get('/avatar-frames')) }
export async function purchaseAvatarFrame(id: number) { return data<AvatarFrame>(await api.post(`/avatar-frames/${id}/purchase`)) }
export async function equipAvatarFrame(frameId: number) { return data<{ avatar_frame: AvatarFrame | null }>(await api.put('/me/avatar-frame', { frame_id: frameId })) }
export async function fetchAdminAvatarFrames() { return data<AvatarFrame[]>(await api.get('/admin/avatar-frames')) }
export async function uploadAdminAvatarFrameImage(file: File) { const form = new FormData(); form.append('file', file); return data<{ image_url: string }>(await api.post('/admin/avatar-frames/image', form)) }
export async function fetchAvatarFrameUploadSettings() { return data<AvatarFrameUploadSettings>(await api.get('/avatar-frame-upload-settings')) }
export async function fetchAdminAvatarFrameUploadSettings() { return data<AvatarFrameUploadSettings>(await api.get('/admin/avatar-frame-upload-settings')) }
export async function updateAdminAvatarFrameUploadSettings(input: Pick<AvatarFrameUploadSettings, 'allow_regular_upload' | 'allow_member_upload'>) { return data<AvatarFrameUploadSettings>(await api.put('/admin/avatar-frame-upload-settings', input)) }
export async function uploadAvatarFrameSubmissionImage(file: File) { const form = new FormData(); form.append('file', file); return data<{ image_url: string }>(await api.post('/avatar-frames/image', form)) }
export async function createAvatarFrameSubmission(input: { name: string; description: string; image_url: string }) { return data<AvatarFrameSubmission>(await api.post('/avatar-frame-submissions', input)) }
export async function fetchMyAvatarFrameSubmissions() { return data<AvatarFrameSubmission[]>(await api.get('/me/avatar-frame-submissions')) }
export async function fetchAdminAvatarFrameSubmissions(status = 'pending') { return data<AvatarFrameSubmission[]>(await api.get('/admin/avatar-frame-submissions', { params: { status } })) }
export async function reviewAdminAvatarFrameSubmission(id: number, status: 'approved' | 'rejected', note = '') { return data<AvatarFrameSubmission>(await api.patch(`/admin/avatar-frame-submissions/${id}`, { status, note })) }
export async function createAdminAvatarFrame(input: Omit<AvatarFrame, 'id' | 'sales_count' | 'owned' | 'equipped' | 'can_use' | 'purchased_at' | 'created_at' | 'updated_at'>) { return data<AvatarFrame>(await api.post('/admin/avatar-frames', input)) }
export async function updateAdminAvatarFrame(id: number, input: Omit<AvatarFrame, 'id' | 'sales_count' | 'owned' | 'equipped' | 'can_use' | 'purchased_at' | 'created_at' | 'updated_at'>) { return data<AvatarFrame>(await api.put(`/admin/avatar-frames/${id}`, input)) }
export async function deleteAdminAvatarFrame(id: number) { return data<{ disabled: boolean }>(await api.delete(`/admin/avatar-frames/${id}`)) }
export async function fetchPublicCaptcha() { return data<CaptchaConfig>(await api.get('/captcha/config')) }
export async function fetchPublicOAuth() { return data<PublicOAuthConfig[]>(await api.get('/oauth/config')) }
export async function fetchBoards() { return data<Board[]>(await api.get('/boards')) }
export async function fetchPosts(params?: { board?: string; q?: string }) { return data<Post[]>(await api.get('/posts', { params })) }
export async function fetchPostsPage(params?: { board?: string; q?: string; offset?: number; limit?: number }) {
  const query: Record<string, string | number> = { paged: 1 }
  const board = params?.board?.trim()
  const q = params?.q?.trim()
  if (board) query.board = board
  if (q) query.q = q
  if (typeof params?.offset === 'number') query.offset = params.offset
  if (typeof params?.limit === 'number') query.limit = params.limit
  return data<PagedResult<Post>>(await api.get('/posts', { params: query }))
}
export async function fetchRecommendedPostsPage(offset = 0, limit = 30) {
  return data<PagedResult<Post>>(await api.get('/posts', { params: { feed: 'for-you', paged: 1, offset, limit } }))
}
export async function fetchCommunitySearch(q: string) { return data<CommunitySearchResult>(await api.get('/search', { params: { q } })) }
export async function fetchFollowingPosts() { return data<Post[]>(await api.get('/me/feed/following')) }
export async function fetchFollowingPostsPage(offset = 0, limit = 30) { return data<PagedResult<Post>>(await api.get('/me/feed/following', { params: { paged: 1, offset, limit } })) }
export async function fetchBookmarkedPosts() { return data<Post[]>(await api.get('/me/bookmarks')) }
export async function fetchPost(id: number) { return data<Post>(await api.get(`/posts/${id}`)) }
export async function fetchComments(id: number) { return data<Comment[]>(await api.get(`/posts/${id}/comments`)) }
export async function createPost(input: { board_id: number; title: string; content: string; tags?: string[]; files?: File[] }) { const form = new FormData(); form.append('board_id', String(input.board_id)); form.append('title', input.title); form.append('content', input.content); for (const tag of input.tags || []) form.append('tags', tag); for (const file of input.files || []) form.append('files', file); return data<Post>(await api.post('/posts', form)) }
export async function createComment(id: number, content: string, files: File[] = [], parentId?: number) {
  if (!files.length) return data<Comment>(await api.post(`/posts/${id}/comments`, { content, parent_id: parentId }))
  const form = new FormData()
  form.append('content', content)
  if (parentId) form.append('parent_id', String(parentId))
  for (const file of files) form.append('files', file)
  return data<Comment>(await api.post(`/posts/${id}/comments`, form))
}
export async function reportPost(id: number, reason: string) { return data<ContentReport>(await api.post(`/posts/${id}/report`, { reason })) }
export async function reportComment(id: number, reason: string) { return data<ContentReport>(await api.post(`/comments/${id}/report`, { reason })) }
export async function fetchResources(q?: string) { return data<Resource[]>(await api.get('/resources', { params: q ? { q } : undefined })) }
export async function fetchResource(id: number) { return data<Resource>(await api.get(`/resources/${id}`)) }
export async function fetchMyResources() { return data<Resource[]>(await api.get('/me/resources')) }
export async function createResource(input: { title: string; description: string; game: string; version: string; resource_type: string; price_cents: number; file: File; preview_files?: File[] }) { const form = new FormData(); form.append('title', input.title); form.append('description', input.description); form.append('game', input.game); form.append('version', input.version); form.append('resource_type', input.resource_type); form.append('price_cents', String(input.price_cents)); form.append('file', input.file); for (const file of input.preview_files || []) form.append('preview_files', file); return data<Resource>(await api.post('/resources', form)) }
export async function fetchAdminResources(status = 'pending') { return data<Resource[]>(await api.get('/admin/resources', { params: { status } })) }
export async function reviewResource(id: number, status: string, reason = '') { return data<Resource>(await api.patch(`/admin/resources/${id}`, { status, reason })) }
export async function createResourceOrder(id: number) { return data<CommerceOrder>(await api.post(`/resources/${id}/orders`)) }
export async function createWalletResourceOrder(id: number) { return data<CommerceOrder>(await api.post(`/resources/${id}/wallet-order`)) }
export async function fetchMyOrders() { return data<CommerceOrder[]>(await api.get('/me/orders')) }
export async function fetchCreatorBalance() { return data<{ available_cents: number; platform_fee_percent: number; service_fee_bps: number; withdrawal_fee_bps: number; membership_tier_id?: number; membership_tier_name?: string }>(await api.get('/me/creator/balance')) }
export async function fetchCreatorPayouts() { return data<CreatorPayout[]>(await api.get('/me/creator/payouts')) }
export async function requestCreatorPayout(input: { amount_cents: number; payout_method: string; payout_account: string; account_name: string; note: string }) { return data<CreatorPayout>(await api.post('/me/creator/payouts', input)) }
export async function fetchPaymentConfig() { return data<PaymentConfig>(await api.get('/admin/payment')) }
export async function updatePaymentConfig(input: PaymentConfig) { return data<PaymentConfig>(await api.put('/admin/payment', input)) }
export async function fetchRedeemCodes() { return data<RedeemCode[]>(await api.get('/admin/wallet/redeem-codes')) }
export async function createRedeemCode(input: { amount_cents: number; expires_at?: string }) { return data<RedeemCode>(await api.post('/admin/wallet/redeem-codes', input)) }
export async function revokeRedeemCode(id: number) { return data<RedeemCode>(await api.post(`/admin/wallet/redeem-codes/${id}/revoke`)) }
export async function fetchAdminPayouts(status = 'pending') { return data<CreatorPayout[]>(await api.get('/admin/payouts', { params: { status } })) }
export async function reviewCreatorPayout(id: number, status: 'paid' | 'rejected', note = '') { return data<CreatorPayout>(await api.patch(`/admin/payouts/${id}`, { status, note })) }
export async function register(input: { email: string; password: string; display_name: string; captcha_token?: string; email_code?: string }) { return data<{ user: User }>(await api.post('/auth/register', input)) }
export async function sendRegistrationVerification(email: string, captcha_token = '') { return data<{ sent: boolean; expires_in: number }>(await api.post('/auth/register/verification', { email, captcha_token })) }
export async function login(input: { email: string; password: string; captcha_token?: string }) { return data<{ user?: User; requires_2fa?: boolean; challenge_token?: string }>(await api.post('/auth/login', input)) }
export async function verifyTwoFactorLogin(challenge_token: string, code: string) { return data<{ user: User }>(await api.post('/auth/login/2fa', { challenge_token, code })) }
export async function fetchTwoFactorStatus() { return data<{ enabled: boolean }>(await api.get('/me/2fa')) }
export async function setupTwoFactor() { return data<{ secret: string; otpauth_uri: string; qr_data_url: string }>(await api.post('/me/2fa/setup')) }
export async function confirmTwoFactor(code: string) { return data<{ enabled: boolean }>(await api.post('/me/2fa/confirm', { code })) }
export async function disableTwoFactor(code: string) { return data<{ enabled: boolean }>(await api.delete('/me/2fa', { data: { code } })) }
export async function logout() { return data<{ logged_out: boolean }>(await api.post('/auth/logout')) }
export async function fetchMe() { return data<User>(await api.get('/me')) }
export async function updateMyProfile(input: { display_name: string; bio: string }) { return data<User>(await api.patch('/me/profile', input)) }
export async function updateMyUID(uid: string) { return data<User>(await api.patch('/me/uid', { uid })) }
export async function uploadMyAvatar(file: File) { const form = new FormData(); form.append('file', file); return data<User>(await api.post('/me/avatar', form)) }
export async function uploadMyCover(file: File) { const form = new FormData(); form.append('file', file); return data<User>(await api.post('/me/cover', form)) }
export async function deleteMyCover() { return data<User>(await api.delete('/me/cover')) }
export async function fetchUserProfile(id: number) { return data<UserProfile>(await api.get(`/users/${id}`)) }
export async function fetchCheckin() { return data<CheckinSummary>(await api.get('/me/checkin')) }
export async function createCheckin() { return data<CheckinSummary>(await api.post('/me/checkin')) }
export async function reportUserProfile(id: number, reason: string) { return data<ContentReport>(await api.post(`/users/${id}/report`, { reason })) }
export async function searchUsers(q = '') { return data<UserSearchResult[]>(await api.get('/users/search', { params: { q } })) }
export async function searchUsersPage(q = '', offset = 0, limit = 30) { return data<PagedResult<UserSearchResult>>(await api.get('/users/search', { params: { q, offset, limit, paged: 1 } })) }
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
export async function fetchConversationInvites() { return data<ConversationInvite[]>(await api.get('/me/conversation-invites')) }
export async function createConversation(input: { kind: 'direct' | 'group'; name?: string; member_ids: number[] }) { return data<Conversation>(await api.post('/conversations', input)) }
export async function respondConversationInvite(id: number, accept: boolean) { return data<Conversation>(await api.post(`/conversation-invites/${id}/respond`, { accept })) }
export async function fetchConversationMembers(id: number) { return data<ConversationMember[]>(await api.get(`/conversations/${id}/members`)) }
export async function inviteConversationMembers(id: number, memberIds: number[]) { return data<{ conversation: Conversation; invited: number }>(await api.post(`/conversations/${id}/members`, { member_ids: memberIds })) }
export async function removeConversationMember(id: number, userId: number) { return data<{ removed: boolean }>(await api.delete(`/conversations/${id}/members/${userId}`)) }
export async function updateConversationName(id: number, name: string) { return data<Conversation>(await api.patch(`/conversations/${id}`, { name })) }
export async function leaveConversation(id: number) { return data<{ left: boolean }>(await api.post(`/conversations/${id}/leave`)) }
export async function deleteConversation(id: number) { return data<{ deleted: boolean }>(await api.delete(`/conversations/${id}`)) }
export async function createConversationInviteLink(id: number) { return data<ConversationInviteLink>(await api.post(`/conversations/${id}/invite-link`)) }
export async function revokeConversationInviteLink(id: number) { return data<{ revoked: boolean }>(await api.delete(`/conversations/${id}/invite-link`)) }
export async function joinConversationByInvite(token: string) { return data<Conversation>(await api.post('/conversation-invite-links/join', { token })) }
export async function remindGroupInvitees(id: number) { return data<{ reminded: number }>(await api.post(`/conversations/${id}/remind`)) }
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
export async function fetchAdminContentReports(status?: 'pending' | 'accepted' | 'rejected') { return data<ContentReport[]>(await api.get('/admin/reports', { params: status ? { status } : undefined })) }
export async function reviewAdminContentReport(id: number, status: 'accepted' | 'rejected', note: string) { return data<ContentReport>(await api.patch(`/admin/reports/${id}`, { status, note })) }
export async function forgotPassword(email: string, captcha_token = '') { return data<{ message: string }>(await api.post('/auth/forgot-password', { email, captcha_token })) }
export async function resetPassword(token: string, password: string) { return data<{ reset: boolean }>(await api.post('/auth/reset-password', { token, password })) }
export async function fetchAdminSite() { return data<SiteSettings>(await api.get('/admin/site')) }
export async function updateAdminSite(input: SiteSettings) { return data<SiteSettings>(await api.put('/admin/site', input)) }
export async function deleteVerificationBadge() { return data<SiteSettings>(await api.delete('/admin/site/verification-badge')) }
export async function fetchAdminMembership() { return data<MembershipSettings>(await api.get('/admin/membership')) }
export async function updateAdminMembership(input: MembershipSettings) { return data<MembershipSettings>(await api.put('/admin/membership', input)) }
export async function createMembershipTier(input: Omit<MembershipTier, 'id' | 'created_at' | 'updated_at'>) { return data<MembershipTier>(await api.post('/admin/membership/tiers', input)) }
export async function updateMembershipTier(id: number, input: MembershipTier) { return data<MembershipTier>(await api.put(`/admin/membership/tiers/${id}`, input)) }
export async function deleteMembershipTier(id: number) { return data<{ deleted: boolean; id: number }>(await api.delete(`/admin/membership/tiers/${id}`)) }
export async function deleteMembershipTierBadge(id: number) { return data<MembershipTier>(await api.delete(`/admin/membership/tiers/${id}/badge`)) }
export async function deleteMembershipBadge() { return data<MembershipSettings>(await api.delete('/admin/membership/badge')) }
export async function fetchSMTP() { return data<SMTPConfig>(await api.get('/admin/smtp')) }
export async function updateSMTP(input: SMTPConfig) { return data<SMTPConfig>(await api.put('/admin/smtp', input)) }
export async function testSMTP(email: string) { return data<{ sent: boolean }>(await api.post('/admin/smtp/test', { email })) }
export async function fetchCaptcha() { return data<CaptchaConfig>(await api.get('/admin/captcha')) }
export async function updateCaptcha(input: CaptchaUpdate) { return data<CaptchaConfig>(await api.put('/admin/captcha', input)) }
export async function fetchOAuth() { return data<OAuthConfig[]>(await api.get('/admin/oauth')) }
export async function createOAuth(input: OAuthConfig) { return data<OAuthConfig>(await api.post('/admin/oauth', input)) }
export async function updateOAuth(input: OAuthConfig) { return data<OAuthConfig>(await api.put(`/admin/oauth/${input.id}`, input)) }
export async function deleteOAuth(id: number) { return data<{ deleted: boolean }>(await api.delete(`/admin/oauth/${id}`)) }
export function resourceDownloadURL(id: number) { return `/api/v1/resources/${id}/download` }
export function adminResourceDownloadURL(id: number) { return `/api/v1/admin/resources/${id}/download` }


export async function fetchAds() { return data<AdSlot[]>(await api.get('/ads')) }
export async function fetchAdminAds() { return data<AdSlot[]>(await api.get('/admin/ads')) }
export async function createAdminAd(input: { title?: string; image_url?: string; link_url?: string; sort_order?: number; enabled?: boolean }) { return data<AdSlot>(await api.post('/admin/ads', input)) }
export async function updateAdminAd(id: number, input: { title?: string; image_url?: string; link_url?: string; sort_order?: number; enabled?: boolean }) { return data<AdSlot>(await api.put('/admin/ads/' + id, input)) }
export async function deleteAdminAd(id: number) { return data<{ deleted: boolean }>(await api.delete('/admin/ads/' + id)) }
export async function fetchNotices() { return data<Notice[]>(await api.get('/notices')) }
export async function fetchAdminNotices() { return data<Notice[]>(await api.get('/admin/notices')) }
export async function createAdminNotice(input: { title: string; content: string; link_url?: string; level?: string; pinned?: boolean; enabled?: boolean }) { return data<Notice>(await api.post('/admin/notices', input)) }
export async function updateAdminNotice(id: number, input: { title?: string; content?: string; link_url?: string; level?: string; pinned?: boolean; enabled?: boolean }) { return data<Notice>(await api.put('/admin/notices/' + id, input)) }
export async function deleteAdminNotice(id: number) { return data<{ deleted: boolean }>(await api.delete('/admin/notices/' + id)) }
export async function pinPost(id: number, pinned: boolean) { return data<Post>(await api.patch('/admin/posts/' + id + '/pin', { pinned })) }
export async function fetchWallet() { return data<WalletSummary>(await api.get('/me/wallet')) }
export async function createWalletTopUp(amountCents: number) { return data<WalletTopUpOrder>(await api.post('/me/wallet/top-ups', { amount_cents: amountCents })) }
export async function redeemWalletCode(code: string) { return data<RedeemCode>(await api.post('/me/wallet/redeem', { code })) }
export async function fetchMembership() { return data<MembershipSummary>(await api.get('/me/membership')) }
export async function subscribeMembership(tierId: number, plan: 'monthly' | 'quarterly' | 'yearly') { return data<MembershipSummary>(await api.post('/me/membership/subscribe', { tier_id: tierId, plan })) }
export function errorMessage(error: any, fallback = '请求失败，请稍后重试') { return error?.response?.data?.error?.message || fallback }
