package domain

import "time"

type User struct {
	ID                  int64        `json:"id"`
	CustomUID           string       `json:"custom_uid,omitempty"`
	Email               string       `json:"email"`
	DisplayName         string       `json:"display_name"`
	AvatarURL           string       `json:"avatar_url"`
	CoverURL            string       `json:"cover_url"`
	Bio                 string       `json:"bio"`
	Role                string       `json:"role"`
	Status              string       `json:"status"`
	BlueVerified        bool         `json:"blue_verified"`
	VerificationLabel   string       `json:"verification_label,omitempty"`
	MemberActive        bool         `json:"member_active"`
	MembershipTierID    int64        `json:"membership_tier_id,omitempty"`
	MembershipStartedAt *time.Time   `json:"membership_started_at,omitempty"`
	MembershipExpiresAt *time.Time   `json:"membership_expires_at,omitempty"`
	AvatarFrame         *AvatarFrame `json:"avatar_frame,omitempty"`
	RobloxName          string       `json:"roblox_name,omitempty"`
	RobloxID            string       `json:"roblox_id,omitempty"`
	RobloxVerified      bool         `json:"roblox_verified"`
	TwoFactorEnabled    bool         `json:"two_factor_enabled"`
	CreatedAt           time.Time    `json:"created_at"`
}

type PublicUser struct {
	ID                int64        `json:"id"`
	CustomUID         string       `json:"custom_uid,omitempty"`
	DisplayName       string       `json:"display_name"`
	AvatarURL         string       `json:"avatar_url"`
	CoverURL          string       `json:"cover_url"`
	Bio               string       `json:"bio"`
	BlueVerified      bool         `json:"blue_verified"`
	VerificationLabel string       `json:"verification_label,omitempty"`
	MemberActive      bool         `json:"member_active"`
	MembershipTierID  int64        `json:"membership_tier_id,omitempty"`
	AvatarFrame       *AvatarFrame `json:"avatar_frame,omitempty"`
	RobloxName        string       `json:"roblox_name,omitempty"`
	RobloxVerified    bool         `json:"roblox_verified"`
	CreatedAt         time.Time    `json:"created_at"`
	Progress          UserProgress `json:"progress"`
	Badges            []Badge      `json:"badges"`
}

type UserProfile struct {
	User           PublicUser `json:"user"`
	Posts          []Post     `json:"posts"`
	Resources      []Resource `json:"resources"`
	FollowerCount  int64      `json:"follower_count"`
	FollowingCount int64      `json:"following_count"`
	Following      bool       `json:"following"`
}

type SiteSettings struct {
	SiteName                 string    `json:"site_name"`
	SiteDescription          string    `json:"site_description"`
	LogoURL                  string    `json:"logo_url"`
	AvatarURL                string    `json:"avatar_url"`
	VerificationBadgeURL     string    `json:"verification_badge_url"`
	PrimaryColor             string    `json:"primary_color"`
	PublicURL                string    `json:"public_url"`
	UserAgreementURL         string    `json:"user_agreement_url"`
	CookiesPolicyURL         string    `json:"cookies_policy_url"`
	AllowRegister            bool      `json:"allow_register"`
	RequireEmailVerification bool      `json:"require_email_verification"`
	PostReviewRequired       bool      `json:"post_review_required"`
	AllowedEmailDomains      []string  `json:"allowed_email_domains"`
	BannerEnabled            bool      `json:"banner_enabled"`
	BannerText               string    `json:"banner_text"`
	BannerLink               string    `json:"banner_link"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type MembershipSettings struct {
	Enabled                 bool             `json:"enabled"`
	DefaultWithdrawalFeeBPS int              `json:"default_withdrawal_fee_bps"`
	DefaultServiceFeeBPS    int              `json:"default_service_fee_bps"`
	Tiers                   []MembershipTier `json:"tiers"`
	UpdatedAt               time.Time        `json:"updated_at"`

	// Legacy fields remain in the wire contract for cached clients during the
	// transition from one global membership product to multiple tiers. They
	// mirror the first tier and are not a second source of truth.
	Name                string `json:"name,omitempty"`
	BadgeLabel          string `json:"badge_label,omitempty"`
	BadgeURL            string `json:"badge_url,omitempty"`
	BadgeColor          string `json:"badge_color,omitempty"`
	MonthlyPriceCents   int64  `json:"monthly_price_cents,omitempty"`
	QuarterlyPriceCents int64  `json:"quarterly_price_cents,omitempty"`
	YearlyPriceCents    int64  `json:"yearly_price_cents,omitempty"`
	PostReviewExempt    bool   `json:"post_review_exempt,omitempty"`
}

type MembershipTier struct {
	ID                  int64     `json:"id"`
	Enabled             bool      `json:"enabled"`
	Name                string    `json:"name"`
	BadgeLabel          string    `json:"badge_label"`
	BadgeURL            string    `json:"badge_url"`
	BadgeColor          string    `json:"badge_color"`
	MonthlyPriceCents   int64     `json:"monthly_price_cents"`
	QuarterlyPriceCents int64     `json:"quarterly_price_cents"`
	YearlyPriceCents    int64     `json:"yearly_price_cents"`
	PostReviewExempt    bool      `json:"post_review_exempt"`
	FeedPriority        bool      `json:"feed_priority"`
	WithdrawalFeeBPS    int       `json:"withdrawal_fee_bps"`
	ServiceFeeBPS       int       `json:"service_fee_bps"`
	SortOrder           int       `json:"sort_order"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type FeePolicy struct {
	MembershipTierID   int64  `json:"membership_tier_id,omitempty"`
	MembershipTierName string `json:"membership_tier_name,omitempty"`
	WithdrawalFeeBPS   int    `json:"withdrawal_fee_bps"`
	ServiceFeeBPS      int    `json:"service_fee_bps"`
}

type MembershipOrder struct {
	ID               int64     `json:"id"`
	OrderNo          string    `json:"order_no"`
	UserID           int64     `json:"user_id"`
	MembershipTierID int64     `json:"membership_tier_id,omitempty"`
	TierName         string    `json:"tier_name"`
	Plan             string    `json:"plan"`
	AmountCents      int64     `json:"amount_cents"`
	DurationMonths   int       `json:"duration_months"`
	StartedAt        time.Time `json:"started_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type MembershipSummary struct {
	Config         MembershipSettings `json:"config"`
	Active         bool               `json:"active"`
	CurrentTier    *MembershipTier    `json:"current_tier,omitempty"`
	StartedAt      *time.Time         `json:"started_at,omitempty"`
	ExpiresAt      *time.Time         `json:"expires_at,omitempty"`
	AvailableCents int64              `json:"available_cents"`
	Orders         []MembershipOrder  `json:"orders"`
}

type AvatarFrame struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Style          string     `json:"style"`
	ImageURL       string     `json:"image_url"`
	PrimaryColor   string     `json:"primary_color"`
	SecondaryColor string     `json:"secondary_color"`
	PriceCents     int64      `json:"price_cents"`
	AllowedRegular bool       `json:"allowed_regular"`
	AllowedMember  bool       `json:"allowed_member"`
	AllowedAdmin   bool       `json:"allowed_admin"`
	Enabled        bool       `json:"enabled"`
	SortOrder      int        `json:"sort_order"`
	SalesCount     int64      `json:"sales_count"`
	Owned          bool       `json:"owned"`
	Equipped       bool       `json:"equipped"`
	CanUse         bool       `json:"can_use"`
	PurchasedAt    *time.Time `json:"purchased_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AvatarFrameUploadSettings struct {
	AllowRegularUpload bool      `json:"allow_regular_upload"`
	AllowMemberUpload  bool      `json:"allow_member_upload"`
	CanUpload          bool      `json:"can_upload"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AvatarFrameSubmission struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	UserName        string     `json:"user_name"`
	UserAvatar      string     `json:"user_avatar"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	ImageURL        string     `json:"image_url"`
	Status          string     `json:"status"`
	ReviewNote      string     `json:"review_note"`
	ReviewedBy      int64      `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	ApprovedFrameID int64      `json:"approved_frame_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AdminUser struct {
	ID                  int64      `json:"id"`
	Email               string     `json:"email"`
	DisplayName         string     `json:"display_name"`
	AvatarURL           string     `json:"avatar_url"`
	Role                string     `json:"role"`
	Status              string     `json:"status"`
	BlueVerified        bool       `json:"blue_verified"`
	VerificationLabel   string     `json:"verification_label,omitempty"`
	MemberActive        bool       `json:"member_active"`
	MembershipTierID    int64      `json:"membership_tier_id,omitempty"`
	MembershipExpiresAt *time.Time `json:"membership_expires_at,omitempty"`
	PostCount           int64      `json:"post_count"`
	CommentCount        int64      `json:"comment_count"`
	ResourceCount       int64      `json:"resource_count"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type SMTPConfig struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	HasPassword    bool   `json:"has_password"`
	MaskedPassword string `json:"masked_password,omitempty"`
	FromEmail      string `json:"from_email"`
	FromName       string `json:"from_name"`
	TLSMode        string `json:"tls_mode"`
}

type CaptchaConfig struct {
	Enabled       bool   `json:"enabled"`
	Provider      string `json:"provider"`
	SiteKey       string `json:"site_key"`
	Endpoint      string `json:"endpoint"`
	Secret        string `json:"-"`
	HasSecret     bool   `json:"has_secret"`
	MaskedSecret  string `json:"masked_secret,omitempty"`
	AdapterStatus string `json:"adapter_status"`
}

type OAuthConfig struct {
	ID                   int64     `json:"id"`
	Enabled              bool      `json:"enabled"`
	ProviderKey          string    `json:"provider_key"`
	ProviderName         string    `json:"provider_name"`
	ClientID             string    `json:"client_id"`
	ClientSecret         string    `json:"client_secret,omitempty"`
	HasClientSecret      bool      `json:"has_client_secret"`
	MaskedClientSecret   string    `json:"masked_client_secret,omitempty"`
	ClearClientSecret    bool      `json:"clear_client_secret,omitempty"`
	AuthorizationURL     string    `json:"authorization_url"`
	TokenURL             string    `json:"token_url"`
	UserInfoURL          string    `json:"userinfo_url"`
	Scopes               string    `json:"scopes"`
	TokenAuthMethod      string    `json:"token_auth_method"`
	RequireVerifiedEmail bool      `json:"require_verified_email"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Board struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	PostCount   int64  `json:"post_count"`
}

type Post struct {
	ID                      int64        `json:"id"`
	BoardID                 int64        `json:"board_id"`
	BoardName               string       `json:"board_name"`
	AuthorID                int64        `json:"author_id"`
	AuthorName              string       `json:"author_name"`
	AuthorAvatar            string       `json:"author_avatar"`
	AuthorAvatarFrame       *AvatarFrame `json:"author_avatar_frame,omitempty"`
	AuthorVerified          bool         `json:"author_verified"`
	AuthorVerificationLabel string       `json:"author_verification_label,omitempty"`
	AuthorMember            bool         `json:"author_member"`
	AuthorMembershipTierID  int64        `json:"author_membership_tier_id,omitempty"`
	Title                   string       `json:"title"`
	Content                 string       `json:"content"`
	Status                  string       `json:"status"`
	Pinned                  bool         `json:"pinned"`
	Featured                bool         `json:"featured"`
	Views                   int64        `json:"views"`
	CommentCount            int64        `json:"comment_count"`
	LikeCount               int64        `json:"like_count"`
	RepostCount             int64        `json:"repost_count"`
	Liked                   bool         `json:"liked"`
	Bookmarked              bool         `json:"bookmarked"`
	Reposted                bool         `json:"reposted"`
	Tags                    []string     `json:"tags,omitempty"`
	Media                   []PostMedia  `json:"media,omitempty"`
	CreatedAt               time.Time    `json:"created_at"`
	UpdatedAt               time.Time    `json:"updated_at"`
}

type PostMedia struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	MIMEType  string `json:"mime_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SizeBytes int64  `json:"size_bytes"`
}

type Comment struct {
	ID                      int64        `json:"id"`
	PostID                  int64        `json:"post_id"`
	ParentID                *int64       `json:"parent_id,omitempty"`
	AuthorID                int64        `json:"author_id"`
	AuthorName              string       `json:"author_name"`
	AuthorAvatar            string       `json:"author_avatar"`
	AuthorAvatarFrame       *AvatarFrame `json:"author_avatar_frame,omitempty"`
	AuthorVerified          bool         `json:"author_verified"`
	AuthorVerificationLabel string       `json:"author_verification_label,omitempty"`
	AuthorMember            bool         `json:"author_member"`
	AuthorMembershipTierID  int64        `json:"author_membership_tier_id,omitempty"`
	Content                 string       `json:"content"`
	Media                   []PostMedia  `json:"media,omitempty"`
	LikeCount               int64        `json:"like_count"`
	Liked                   bool         `json:"liked"`
	CreatedAt               time.Time    `json:"created_at"`
}

type FollowStatus struct {
	Following      bool  `json:"following"`
	FollowerCount  int64 `json:"follower_count"`
	FollowingCount int64 `json:"following_count"`
}

type Notification struct {
	ID               int64     `json:"id"`
	Kind             string    `json:"kind"`
	ActorID          int64     `json:"actor_id"`
	ActorName        string    `json:"actor_name"`
	ActorAvatar      string    `json:"actor_avatar"`
	PostID           *int64    `json:"post_id,omitempty"`
	CommentID        *int64    `json:"comment_id,omitempty"`
	ConversationID   *int64    `json:"conversation_id,omitempty"`
	PostTitle        string    `json:"post_title,omitempty"`
	ConversationName string    `json:"conversation_name,omitempty"`
	Read             bool      `json:"read"`
	CreatedAt        time.Time `json:"created_at"`
}

// ContentReport is an authenticated report against public community content.
// Target fields are denormalized at read time so the moderation queue can show
// a post, comment, or profile without requiring three separate endpoints.
type ContentReport struct {
	ID               int64      `json:"id"`
	ReporterID       int64      `json:"reporter_id"`
	ReporterName     string     `json:"reporter_name"`
	ReporterAvatar   string     `json:"reporter_avatar"`
	TargetType       string     `json:"target_type"`
	TargetID         int64      `json:"target_id"`
	TargetUserID     int64      `json:"target_user_id"`
	TargetUserName   string     `json:"target_user_name"`
	TargetUserAvatar string     `json:"target_user_avatar"`
	TargetPostID     *int64     `json:"target_post_id,omitempty"`
	TargetTitle      string     `json:"target_title"`
	TargetContent    string     `json:"target_content"`
	TargetStatus     string     `json:"target_status"`
	Reason           string     `json:"reason"`
	Status           string     `json:"status"`
	ReviewNote       string     `json:"review_note,omitempty"`
	ReviewedBy       *int64     `json:"reviewed_by,omitempty"`
	ReviewerName     string     `json:"reviewer_name,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type UserSearchResult struct {
	PublicUser
	PostCount     int64 `json:"post_count"`
	ResourceCount int64 `json:"resource_count"`
	HotScore      int64 `json:"hot_score"`
}

type CommunitySearchResult struct {
	Query     string             `json:"query"`
	Posts     []Post             `json:"posts"`
	Resources []Resource         `json:"resources"`
	Users     []UserSearchResult `json:"users"`
}

type Conversation struct {
	ID                  int64        `json:"id"`
	Kind                string       `json:"kind"`
	Name                string       `json:"name"`
	CreatedBy           int64        `json:"created_by"`
	Members             []PublicUser `json:"members"`
	LastMessage         *Message     `json:"last_message,omitempty"`
	UnreadCount         int64        `json:"unread_count"`
	MembershipStatus    string       `json:"membership_status"`
	AcceptedMemberCount int64        `json:"accepted_member_count"`
	PendingInviteCount  int64        `json:"pending_invite_count"`
	Active              bool         `json:"active"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type ConversationInvite struct {
	Conversation Conversation `json:"conversation"`
	InvitedAt    time.Time    `json:"invited_at"`
}

type ConversationMember struct {
	PublicUser
	MembershipStatus string     `json:"membership_status"`
	InvitedBy        *int64     `json:"invited_by,omitempty"`
	JoinedAt         time.Time  `json:"joined_at"`
	RespondedAt      *time.Time `json:"responded_at,omitempty"`
}

type ConversationInviteLink struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PostPage struct {
	Items      []Post `json:"items"`
	NextOffset int    `json:"next_offset"`
	HasMore    bool   `json:"has_more"`
}

type UserSearchPage struct {
	Items      []UserSearchResult `json:"items"`
	NextOffset int                `json:"next_offset"`
	HasMore    bool               `json:"has_more"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	SenderAvatar   string    `json:"sender_avatar"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type Resource struct {
	ID                       int64           `json:"id"`
	CreatorID                int64           `json:"creator_id"`
	CreatorName              string          `json:"creator_name"`
	CreatorVerified          bool            `json:"creator_verified"`
	CreatorVerificationLabel string          `json:"creator_verification_label,omitempty"`
	CreatorMember            bool            `json:"creator_member"`
	CreatorMembershipTierID  int64           `json:"creator_membership_tier_id,omitempty"`
	Title                    string          `json:"title"`
	Description              string          `json:"description"`
	Game                     string          `json:"game"`
	Version                  string          `json:"version"`
	ResourceType             string          `json:"resource_type"`
	PriceCents               int64           `json:"price_cents"`
	Status                   string          `json:"status"`
	ReviewReason             string          `json:"review_reason,omitempty"`
	DownloadCount            int64           `json:"download_count"`
	SalesCount               int64           `json:"sales_count"`
	File                     *ResourceFile   `json:"file,omitempty"`
	Media                    []ResourceMedia `json:"media,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

type ResourceMedia struct {
	ID         int64  `json:"id"`
	ResourceID int64  `json:"resource_id,omitempty"`
	URL        string `json:"url"`
	MIMEType   string `json:"mime_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SizeBytes  int64  `json:"size_bytes"`
}

type VerificationApplication struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	UserName         string     `json:"user_name"`
	UserEmail        string     `json:"user_email,omitempty"`
	VerificationType string     `json:"verification_type"`
	RequestedLabel   string     `json:"requested_label"`
	EvidenceURL      string     `json:"evidence_url"`
	Statement        string     `json:"statement"`
	Status           string     `json:"status"`
	ReviewNote       string     `json:"review_note,omitempty"`
	ReviewedBy       *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ResourceFile struct {
	ID           int64     `json:"id"`
	ResourceID   int64     `json:"resource_id"`
	OriginalName string    `json:"original_name"`
	MIMEType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}

type PaymentConfig struct {
	Enabled      bool   `json:"enabled"`
	Kind         string `json:"kind"`
	GatewayURL   string `json:"gateway_url"`
	MerchantID   string `json:"merchant_id"`
	Secret       string `json:"secret,omitempty"`
	HasSecret    bool   `json:"has_secret"`
	MaskedSecret string `json:"masked_secret,omitempty"`
	PayType      string `json:"pay_type"`
	NotifyURL    string `json:"notify_url"`
	ReturnURL    string `json:"return_url"`
}

type CommerceOrder struct {
	ID                       int64      `json:"id"`
	OrderNo                  string     `json:"order_no"`
	UserID                   int64      `json:"user_id"`
	ResourceID               int64      `json:"resource_id"`
	ResourceTitle            string     `json:"resource_title"`
	AmountCents              int64      `json:"amount_cents"`
	ServiceFeeBPS            int        `json:"service_fee_bps"`
	ServiceFeeCents          int64      `json:"service_fee_cents"`
	CreatorShareCents        int64      `json:"creator_share_cents"`
	SellerMembershipTierID   int64      `json:"seller_membership_tier_id,omitempty"`
	SellerMembershipTierName string     `json:"seller_membership_tier_name,omitempty"`
	Status                   string     `json:"status"`
	GatewayTradeNo           string     `json:"gateway_trade_no,omitempty"`
	PaymentURL               string     `json:"payment_url,omitempty"`
	PaidAt                   *time.Time `json:"paid_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

type CreatorPayout struct {
	ID                 int64      `json:"id"`
	CreatorID          int64      `json:"creator_id"`
	CreatorName        string     `json:"creator_name,omitempty"`
	CreatorEmail       string     `json:"creator_email,omitempty"`
	AmountCents        int64      `json:"amount_cents"`
	WithdrawalFeeBPS   int        `json:"withdrawal_fee_bps"`
	WithdrawalFeeCents int64      `json:"withdrawal_fee_cents"`
	NetAmountCents     int64      `json:"net_amount_cents"`
	MembershipTierID   int64      `json:"membership_tier_id,omitempty"`
	MembershipTierName string     `json:"membership_tier_name,omitempty"`
	Status             string     `json:"status"`
	PayoutMethod       string     `json:"payout_method"`
	PayoutAccount      string     `json:"payout_account"`
	AccountName        string     `json:"account_name"`
	Note               string     `json:"note"`
	ReviewNote         string     `json:"review_note,omitempty"`
	ReviewedBy         *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AdSlot struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	ImageURL  string    `json:"image_url"`
	LinkURL   string    `json:"link_url"`
	SortOrder int       `json:"sort_order"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Notice struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	LinkURL   string    `json:"link_url"`
	Level     string    `json:"level"`
	Pinned    bool      `json:"pinned"`
	Enabled   bool      `json:"enabled"`
	CreatedBy int64     `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Badge struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Color       string    `json:"color"`
	AwardedAt   time.Time `json:"awarded_at"`
}

type UserProgress struct {
	Experience          int64  `json:"experience"`
	TotalCheckins       int    `json:"total_checkins"`
	CurrentStreak       int    `json:"current_streak"`
	LongestStreak       int    `json:"longest_streak"`
	LastCheckinDate     string `json:"last_checkin_date,omitempty"`
	CheckedInToday      bool   `json:"checked_in_today"`
	Level               int    `json:"level"`
	LevelName           string `json:"level_name"`
	LevelMinExperience  int64  `json:"level_min_experience"`
	NextLevelExperience int64  `json:"next_level_experience"`
	LevelProgress       int    `json:"level_progress"`
}

type CheckinRecord struct {
	Date              string    `json:"date"`
	ExperienceAwarded int       `json:"experience_awarded"`
	Streak            int       `json:"streak"`
	CreatedAt         time.Time `json:"created_at"`
}

type CheckinSummary struct {
	Progress     UserProgress    `json:"progress"`
	History      []CheckinRecord `json:"history"`
	Badges       []Badge         `json:"badges"`
	NewlyAwarded []Badge         `json:"newly_awarded"`
}

type WalletEntry struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	EntryType     string    `json:"entry_type"`
	AmountCents   int64     `json:"amount_cents"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   int64     `json:"reference_id"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

type WalletSummary struct {
	AvailableCents    int64              `json:"available_cents"`
	WithdrawableCents int64              `json:"withdrawable_cents"`
	FeePolicy         FeePolicy          `json:"fee_policy"`
	Entries           []WalletEntry      `json:"entries"`
	TopUps            []WalletTopUpOrder `json:"top_ups"`
}

type WalletTopUpOrder struct {
	ID             int64      `json:"id"`
	OrderNo        string     `json:"order_no"`
	UserID         int64      `json:"user_id"`
	AmountCents    int64      `json:"amount_cents"`
	Status         string     `json:"status"`
	GatewayTradeNo string     `json:"gateway_trade_no,omitempty"`
	PaymentURL     string     `json:"payment_url,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type RedeemCode struct {
	ID             int64      `json:"id"`
	Code           string     `json:"code,omitempty"`
	CodeHint       string     `json:"code_hint"`
	AmountCents    int64      `json:"amount_cents"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	RedeemedBy     *int64     `json:"redeemed_by,omitempty"`
	RedeemedByName string     `json:"redeemed_by_name,omitempty"`
	RedeemedAt     *time.Time `json:"redeemed_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedBy      int64      `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

type PointAccount struct {
	UserID      int64     `json:"user_id"`
	Balance     int64     `json:"balance"`
	TotalEarned int64     `json:"total_earned"`
	TotalSpent  int64     `json:"total_spent"`
	UpdatedAt   time.Time `json:"updated_at"`
	UserName    string    `json:"user_name,omitempty"`
	UserAvatar  string    `json:"user_avatar,omitempty"`
}

type PointLedger struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Amount       int64     `json:"amount"`
	EntryType    string    `json:"type"`
	SourceID     int64     `json:"source_id,omitempty"`
	BalanceAfter int64     `json:"balance_after"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

type PointSummary struct {
	Account PointAccount  `json:"account"`
	Entries []PointLedger `json:"entries"`
}

type PointProduct struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	ImageURL       string    `json:"image_url"`
	PointsRequired int64     `json:"points_required"`
	ShippingPoints int64     `json:"shipping_points"`
	Stock          int64     `json:"stock"`
	Physical       bool      `json:"physical"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ShippingAddress struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Province string `json:"province"`
	City     string `json:"city"`
	District string `json:"district"`
	Detail   string `json:"detail"`
}

type PointOrder struct {
	ID             int64            `json:"id"`
	OrderNo        string           `json:"order_no"`
	UserID         int64            `json:"user_id"`
	UserName       string           `json:"user_name,omitempty"`
	ProductID      int64            `json:"product_id"`
	ProductName    string           `json:"product_name"`
	ProductImage   string           `json:"product_image"`
	PointsSpent    int64            `json:"points_spent"`
	ShippingPoints int64            `json:"shipping_points"`
	Physical       bool             `json:"physical"`
	Address        *ShippingAddress `json:"address,omitempty"`
	Carrier        string           `json:"carrier,omitempty"`
	TrackingNo     string           `json:"tracking_no,omitempty"`
	Status         string           `json:"status"`
	Note           string           `json:"note"`
	OrderedAt      time.Time        `json:"ordered_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type PointOrderFilter struct {
	Status    string
	ProductID int64
	UserID    int64
	StartedAt *time.Time
	EndedAt   *time.Time
}

type PointTracking struct {
	OrderID     int64     `json:"order_id"`
	OrderNo     string    `json:"order_no"`
	Carrier     string    `json:"carrier"`
	TrackingNo  string    `json:"tracking_no"`
	OrderStatus string    `json:"order_status"`
	QueryURL    string    `json:"query_url"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LotteryActivity struct {
	ID                int64          `json:"id"`
	Name              string         `json:"name"`
	CoverURL          string         `json:"cover_url"`
	Description       string         `json:"description"`
	Style             string         `json:"style"`
	CostPoints        int64          `json:"cost_points"`
	DailyLimit        int            `json:"daily_limit"`
	TotalLimit        int            `json:"total_limit"`
	DailyFreeAttempts int            `json:"daily_free_attempts"`
	StartsAt          time.Time      `json:"starts_at"`
	EndsAt            time.Time      `json:"ends_at"`
	Status            string         `json:"status"`
	SortOrder         int            `json:"sort_order"`
	Prizes            []LotteryPrize `json:"prizes,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type RobloxMusic struct {
	AssetID      int64     `json:"asset_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	CreatorName  string    `json:"creator_name,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	Favorited    bool      `json:"favorited"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type RobloxMusicSubmission struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	UserName   string     `json:"user_name"`
	UserAvatar string     `json:"user_avatar,omitempty"`
	AssetID    int64      `json:"asset_id"`
	Name       string     `json:"name"`
	ImageURL   string     `json:"image_url"`
	Status     string     `json:"status"`
	ReviewNote string     `json:"review_note,omitempty"`
	ReviewedBy int64      `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type LotteryPrize struct {
	ID            int64     `json:"id"`
	ActivityID    int64     `json:"activity_id"`
	Name          string    `json:"name"`
	ImageURL      string    `json:"image_url"`
	ProbabilityBP int       `json:"probability_bp"`
	Stock         int64     `json:"stock"`
	PrizeType     string    `json:"prize_type"`
	BoundID       int64     `json:"bound_id,omitempty"`
	ConfigJSON    string    `json:"config_json"`
	Physical      bool      `json:"physical"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type LotteryWin struct {
	ID           int64            `json:"id"`
	UserID       int64            `json:"user_id"`
	UserName     string           `json:"user_name,omitempty"`
	ActivityID   int64            `json:"activity_id"`
	ActivityName string           `json:"activity_name,omitempty"`
	PrizeID      int64            `json:"prize_id"`
	PrizeName    string           `json:"prize_name"`
	PrizeType    string           `json:"prize_type"`
	Physical     bool             `json:"physical"`
	Status       string           `json:"status"`
	OrderID      int64            `json:"order_id,omitempty"`
	Address      *ShippingAddress `json:"address,omitempty"`
	DeliveredAt  *time.Time       `json:"delivered_at,omitempty"`
	WonAt        time.Time        `json:"won_at"`
}

type LotteryDrawResult struct {
	Won           bool          `json:"won"`
	Prize         *LotteryPrize `json:"prize,omitempty"`
	Win           *LotteryWin   `json:"win,omitempty"`
	PointsCharged int64         `json:"points_charged"`
	Balance       int64         `json:"balance"`
}

type LotteryActivityStats struct {
	ActivityID     int64  `json:"activity_id"`
	ActivityName   string `json:"activity_name"`
	DrawCount      int64  `json:"draw_count"`
	UniqueUsers    int64  `json:"unique_users"`
	WinCount       int64  `json:"win_count"`
	DeliveredCount int64  `json:"delivered_count"`
	PendingCount   int64  `json:"pending_count"`
	PointsSpent    int64  `json:"points_spent"`
	WinRateBP      int64  `json:"win_rate_bp"`
}

type PointsDashboard struct {
	AccountCount  int64                  `json:"account_count"`
	TotalBalance  int64                  `json:"total_balance"`
	TotalEarned   int64                  `json:"total_earned"`
	TotalSpent    int64                  `json:"total_spent"`
	ProductCount  int64                  `json:"product_count"`
	OrderCount    int64                  `json:"order_count"`
	PendingOrders int64                  `json:"pending_orders"`
	DrawCount     int64                  `json:"draw_count"`
	WinCount      int64                  `json:"win_count"`
	PendingWins   int64                  `json:"pending_wins"`
	Activities    []LotteryActivityStats `json:"activities"`
}
