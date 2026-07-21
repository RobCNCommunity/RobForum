package domain

import "time"

type User struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	DisplayName       string    `json:"display_name"`
	AvatarURL         string    `json:"avatar_url"`
	CoverURL          string    `json:"cover_url"`
	Bio               string    `json:"bio"`
	Role              string    `json:"role"`
	Status            string    `json:"status"`
	BlueVerified      bool      `json:"blue_verified"`
	VerificationLabel string    `json:"verification_label,omitempty"`
	RobloxName        string    `json:"roblox_name,omitempty"`
	RobloxID          string    `json:"roblox_id,omitempty"`
	RobloxVerified    bool      `json:"roblox_verified"`
	CreatedAt         time.Time `json:"created_at"`
}

type PublicUser struct {
	ID                int64     `json:"id"`
	DisplayName       string    `json:"display_name"`
	AvatarURL         string    `json:"avatar_url"`
	CoverURL          string    `json:"cover_url"`
	Bio               string    `json:"bio"`
	BlueVerified      bool      `json:"blue_verified"`
	VerificationLabel string    `json:"verification_label,omitempty"`
	RobloxName        string    `json:"roblox_name,omitempty"`
	RobloxVerified    bool      `json:"roblox_verified"`
	CreatedAt         time.Time `json:"created_at"`
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
	AllowRegister            bool      `json:"allow_register"`
	RequireEmailVerification bool      `json:"require_email_verification"`
	PostReviewRequired       bool      `json:"post_review_required"`
	AllowedEmailDomains      []string  `json:"allowed_email_domains"`
	BannerEnabled            bool      `json:"banner_enabled"`
	BannerText               string    `json:"banner_text"`
	BannerLink               string    `json:"banner_link"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type AdminUser struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	DisplayName       string    `json:"display_name"`
	AvatarURL         string    `json:"avatar_url"`
	Role              string    `json:"role"`
	Status            string    `json:"status"`
	BlueVerified      bool      `json:"blue_verified"`
	VerificationLabel string    `json:"verification_label,omitempty"`
	PostCount         int64     `json:"post_count"`
	CommentCount      int64     `json:"comment_count"`
	ResourceCount     int64     `json:"resource_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
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
	ID                      int64       `json:"id"`
	BoardID                 int64       `json:"board_id"`
	BoardName               string      `json:"board_name"`
	AuthorID                int64       `json:"author_id"`
	AuthorName              string      `json:"author_name"`
	AuthorAvatar            string      `json:"author_avatar"`
	AuthorVerified          bool        `json:"author_verified"`
	AuthorVerificationLabel string      `json:"author_verification_label,omitempty"`
	Title                   string      `json:"title"`
	Content                 string      `json:"content"`
	PostType                string      `json:"post_type"`
	Status                  string      `json:"status"`
	Pinned                  bool        `json:"pinned"`
	Featured                bool        `json:"featured"`
	Views                   int64       `json:"views"`
	CommentCount            int64       `json:"comment_count"`
	LikeCount               int64       `json:"like_count"`
	RepostCount             int64       `json:"repost_count"`
	Liked                   bool        `json:"liked"`
	Bookmarked              bool        `json:"bookmarked"`
	Reposted                bool        `json:"reposted"`
	Media                   []PostMedia `json:"media,omitempty"`
	CreatedAt               time.Time   `json:"created_at"`
	UpdatedAt               time.Time   `json:"updated_at"`
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
	ID                      int64     `json:"id"`
	PostID                  int64     `json:"post_id"`
	AuthorID                int64     `json:"author_id"`
	AuthorName              string    `json:"author_name"`
	AuthorAvatar            string    `json:"author_avatar"`
	AuthorVerified          bool      `json:"author_verified"`
	AuthorVerificationLabel string    `json:"author_verification_label,omitempty"`
	Content                 string    `json:"content"`
	LikeCount               int64     `json:"like_count"`
	Liked                   bool      `json:"liked"`
	CreatedAt               time.Time `json:"created_at"`
}

type FollowStatus struct {
	Following      bool  `json:"following"`
	FollowerCount  int64 `json:"follower_count"`
	FollowingCount int64 `json:"following_count"`
}

type Notification struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	ActorID     int64     `json:"actor_id"`
	ActorName   string    `json:"actor_name"`
	ActorAvatar string    `json:"actor_avatar"`
	PostID      *int64    `json:"post_id,omitempty"`
	CommentID   *int64    `json:"comment_id,omitempty"`
	PostTitle   string    `json:"post_title,omitempty"`
	Read        bool      `json:"read"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserSearchResult struct {
	PublicUser
	PostCount     int64 `json:"post_count"`
	ResourceCount int64 `json:"resource_count"`
	HotScore      int64 `json:"hot_score"`
}

type Conversation struct {
	ID          int64        `json:"id"`
	Kind        string       `json:"kind"`
	Name        string       `json:"name"`
	CreatedBy   int64        `json:"created_by"`
	Members     []PublicUser `json:"members"`
	LastMessage *Message     `json:"last_message,omitempty"`
	UnreadCount int64        `json:"unread_count"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
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
	ID                       int64         `json:"id"`
	CreatorID                int64         `json:"creator_id"`
	CreatorName              string        `json:"creator_name"`
	CreatorVerified          bool          `json:"creator_verified"`
	CreatorVerificationLabel string        `json:"creator_verification_label,omitempty"`
	Title                    string        `json:"title"`
	Description              string        `json:"description"`
	Game                     string        `json:"game"`
	Version                  string        `json:"version"`
	ResourceType             string        `json:"resource_type"`
	PriceCents               int64         `json:"price_cents"`
	Status                   string        `json:"status"`
	ReviewReason             string        `json:"review_reason,omitempty"`
	DownloadCount            int64         `json:"download_count"`
	SalesCount               int64         `json:"sales_count"`
	File                     *ResourceFile `json:"file,omitempty"`
	CreatedAt                time.Time     `json:"created_at"`
	UpdatedAt                time.Time     `json:"updated_at"`
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
	ID             int64      `json:"id"`
	OrderNo        string     `json:"order_no"`
	UserID         int64      `json:"user_id"`
	ResourceID     int64      `json:"resource_id"`
	ResourceTitle  string     `json:"resource_title"`
	AmountCents    int64      `json:"amount_cents"`
	Status         string     `json:"status"`
	GatewayTradeNo string     `json:"gateway_trade_no,omitempty"`
	PaymentURL     string     `json:"payment_url,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreatorPayout struct {
	ID            int64      `json:"id"`
	CreatorID     int64      `json:"creator_id"`
	CreatorName   string     `json:"creator_name,omitempty"`
	CreatorEmail  string     `json:"creator_email,omitempty"`
	AmountCents   int64      `json:"amount_cents"`
	Status        string     `json:"status"`
	PayoutMethod  string     `json:"payout_method"`
	PayoutAccount string     `json:"payout_account"`
	AccountName   string     `json:"account_name"`
	Note          string     `json:"note"`
	ReviewNote    string     `json:"review_note,omitempty"`
	ReviewedBy    *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
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
	Level     string    `json:"level"`
	Pinned    bool      `json:"pinned"`
	Enabled   bool      `json:"enabled"`
	CreatedBy int64     `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	AvailableCents int64         `json:"available_cents"`
	Entries        []WalletEntry `json:"entries"`
}
