CREATE TABLE IF NOT EXISTS site_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  site_name VARCHAR(120) NOT NULL,
  site_description VARCHAR(500) NOT NULL,
  logo_url VARCHAR(500) NOT NULL,
  avatar_url VARCHAR(500) NOT NULL,
  verification_badge_url VARCHAR(500) NOT NULL DEFAULT '',
  primary_color VARCHAR(32) NOT NULL,
  public_url VARCHAR(255) NOT NULL,
  allow_register TINYINT(1) NOT NULL DEFAULT 1,
  require_email_verification TINYINT(1) NOT NULL DEFAULT 0,
  post_review_required TINYINT(1) NOT NULL DEFAULT 1,
  allowed_email_domains TEXT NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS captcha_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  provider VARCHAR(32) NOT NULL DEFAULT 'gt4',
  site_key VARCHAR(255) NOT NULL,
  endpoint VARCHAR(500) NOT NULL,
  secret_ciphertext TEXT NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS smtp_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  host VARCHAR(255) NOT NULL,
  port INT NOT NULL DEFAULT 587,
  username VARCHAR(255) NOT NULL,
  password_ciphertext TEXT NOT NULL,
  from_email VARCHAR(255) NOT NULL,
  from_name VARCHAR(120) NOT NULL,
  tls_mode VARCHAR(16) NOT NULL DEFAULT 'starttls',
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(80) NOT NULL,
  avatar_url VARCHAR(500) NOT NULL,
  cover_url VARCHAR(500) NOT NULL DEFAULT '',
  bio VARCHAR(500) NOT NULL DEFAULT '',
  role VARCHAR(24) NOT NULL DEFAULT 'user',
  status VARCHAR(24) NOT NULL DEFAULT 'active',
  blue_verified TINYINT(1) NOT NULL DEFAULT 0,
  verification_label VARCHAR(80) NOT NULL DEFAULT '',
  roblox_name VARCHAR(120) NOT NULL DEFAULT '',
  roblox_id VARCHAR(64) NOT NULL DEFAULT '',
  roblox_verified TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_users_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_follows (
  follower_id BIGINT NOT NULL,
  followed_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (follower_id, followed_id),
  INDEX idx_user_follows_followed (followed_id, created_at),
  CONSTRAINT fk_user_follows_follower FOREIGN KEY (follower_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_follows_followed FOREIGN KEY (followed_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS auth_sessions (
  token_hash VARBINARY(32) NOT NULL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_sessions_user (user_id),
  INDEX idx_sessions_expires (expires_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS password_resets (
  token_hash VARBINARY(32) NOT NULL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_resets_user (user_id),
  INDEX idx_resets_user_created (user_id, created_at),
  INDEX idx_resets_expires (expires_at),
  CONSTRAINT fk_resets_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS email_verifications (
  email VARCHAR(255) NOT NULL PRIMARY KEY,
  code_hash VARCHAR(255) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  sent_at DATETIME(6) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  INDEX idx_email_verifications_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS boards (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  slug VARCHAR(80) NOT NULL UNIQUE,
  name VARCHAR(80) NOT NULL,
  description VARCHAR(255) NOT NULL,
  icon VARCHAR(64) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS posts (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  board_id BIGINT NOT NULL,
  author_id BIGINT NOT NULL,
  title VARCHAR(180) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  post_type VARCHAR(24) NOT NULL DEFAULT 'discussion',
  status VARCHAR(24) NOT NULL DEFAULT 'published',
  pinned TINYINT(1) NOT NULL DEFAULT 0,
  featured TINYINT(1) NOT NULL DEFAULT 0,
  views BIGINT NOT NULL DEFAULT 0,
  comment_count BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_posts_board (board_id, status, pinned, updated_at),
  INDEX idx_posts_author (author_id),
  INDEX idx_posts_status_updated (status, updated_at),
  FULLTEXT KEY ft_posts_content (title, content),
  CONSTRAINT fk_posts_board FOREIGN KEY (board_id) REFERENCES boards(id),
  CONSTRAINT fk_posts_author FOREIGN KEY (author_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_media (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  post_id BIGINT NOT NULL,
  stored_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(160) NOT NULL,
  width INT NOT NULL,
  height INT NOT NULL,
  size_bytes BIGINT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uniq_post_media_stored_name (stored_name),
  INDEX idx_post_media_post (post_id, sort_order, id),
  CONSTRAINT fk_post_media_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comments (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  post_id BIGINT NOT NULL,
  author_id BIGINT NOT NULL,
  content TEXT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'published',
  created_at DATETIME(6) NOT NULL,
  INDEX idx_comments_post (post_id, status, created_at),
  INDEX idx_comments_author (author_id, status, post_id),
  CONSTRAINT fk_comments_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  CONSTRAINT fk_comments_author FOREIGN KEY (author_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_bookmarks (
  post_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (post_id, user_id),
  INDEX idx_post_bookmarks_user (user_id, created_at),
  CONSTRAINT fk_post_bookmarks_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  CONSTRAINT fk_post_bookmarks_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_reposts (
  post_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (post_id, user_id),
  INDEX idx_post_reposts_user (user_id, created_at),
  CONSTRAINT fk_post_reposts_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  CONSTRAINT fk_post_reposts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  kind VARCHAR(32) NOT NULL,
  post_id BIGINT NULL,
  comment_id BIGINT NULL,
  created_at DATETIME(6) NOT NULL,
  read_at DATETIME(6) NULL,
  INDEX idx_notifications_user (user_id, read_at, created_at),
  CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_notifications_actor FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_notifications_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  CONSTRAINT fk_notifications_comment FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS resources (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id BIGINT NOT NULL,
  title VARCHAR(180) NOT NULL,
  description MEDIUMTEXT NOT NULL,
  game VARCHAR(120) NOT NULL,
  version VARCHAR(80) NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    price_cents BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  review_reason VARCHAR(500) NOT NULL DEFAULT '',
    download_count BIGINT NOT NULL DEFAULT 0,
    sales_count BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_resources_status (status, updated_at),
  INDEX idx_resources_creator (creator_id, updated_at),
  INDEX idx_resources_creator_status (creator_id, status, updated_at),
  CONSTRAINT fk_resources_creator FOREIGN KEY (creator_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS resource_files (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  resource_id BIGINT NOT NULL UNIQUE,
  original_name VARCHAR(255) NOT NULL,
  stored_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(160) NOT NULL,
  size_bytes BIGINT NOT NULL,
  sha256 CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_resource_files_sha (sha256),
  CONSTRAINT fk_resource_files_resource FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS moderation_actions (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  actor_id BIGINT NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id BIGINT NOT NULL,
  action VARCHAR(32) NOT NULL,
  reason VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  INDEX idx_moderation_target (target_type, target_id, created_at),
  CONSTRAINT fk_moderation_actor FOREIGN KEY (actor_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS payment_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  kind VARCHAR(32) NOT NULL DEFAULT 'epay',
  gateway_url VARCHAR(500) NOT NULL,
  merchant_id VARCHAR(255) NOT NULL,
  secret_ciphertext TEXT NOT NULL,
  pay_type VARCHAR(32) NOT NULL DEFAULT 'alipay',
  notify_url VARCHAR(500) NOT NULL,
  return_url VARCHAR(500) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS commerce_orders (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_no VARCHAR(64) NOT NULL UNIQUE,
  user_id BIGINT NOT NULL,
  resource_id BIGINT NOT NULL,
  amount_cents BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  gateway_trade_no VARCHAR(255) NOT NULL DEFAULT '',
  paid_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_orders_user (user_id, created_at),
  INDEX idx_orders_resource (resource_id, status),
  INDEX idx_orders_user_resource_status (user_id, resource_id, status, created_at),
  INDEX idx_orders_gateway (gateway_trade_no),
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_orders_resource FOREIGN KEY (resource_id) REFERENCES resources(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS payment_transactions (
  gateway_trade_no VARCHAR(255) NOT NULL PRIMARY KEY,
  order_id BIGINT NOT NULL UNIQUE,
  amount_cents BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_payment_transactions_order FOREIGN KEY (order_id) REFERENCES commerce_orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS wallet_ledgers (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  entry_type VARCHAR(32) NOT NULL,
  amount_cents BIGINT NOT NULL,
  reference_type VARCHAR(32) NOT NULL,
  reference_id BIGINT NOT NULL,
  note VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uniq_wallet_reference (user_id, reference_type, reference_id, entry_type),
  INDEX idx_wallet_user (user_id, created_at),
  CONSTRAINT fk_wallet_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS creator_payouts (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id BIGINT NOT NULL,
  amount_cents BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  payout_method VARCHAR(32) NOT NULL DEFAULT 'alipay',
  payout_account VARCHAR(255) NOT NULL DEFAULT '',
  account_name VARCHAR(120) NOT NULL DEFAULT '',
  note VARCHAR(500) NOT NULL DEFAULT '',
  review_note VARCHAR(500) NOT NULL DEFAULT '',
  reviewed_by BIGINT NULL,
  reviewed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_payouts_creator (creator_id, created_at),
  CONSTRAINT fk_payouts_creator FOREIGN KEY (creator_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS verification_applications (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  verification_type VARCHAR(32) NOT NULL,
  requested_label VARCHAR(80) NOT NULL,
  evidence_url VARCHAR(500) NOT NULL,
  statement TEXT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  review_note VARCHAR(500) NOT NULL DEFAULT '',
  reviewed_by BIGINT NULL,
  reviewed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_verification_user (user_id, created_at),
  INDEX idx_verification_status (status, created_at),
  CONSTRAINT fk_verification_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ad_slots (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(120) NOT NULL DEFAULT '',
  image_url VARCHAR(500) NOT NULL DEFAULT '',
  link_url VARCHAR(500) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_ad_slots_enabled (enabled, sort_order, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notices (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(160) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  level VARCHAR(16) NOT NULL DEFAULT 'info',
  pinned TINYINT(1) NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_by BIGINT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_notices_enabled (enabled, pinned, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_likes (
  post_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (post_id, user_id),
  INDEX idx_post_likes_user (user_id, created_at),
  CONSTRAINT fk_post_likes_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  CONSTRAINT fk_post_likes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comment_likes (
  comment_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (comment_id, user_id),
  INDEX idx_comment_likes_user (user_id, created_at),
  CONSTRAINT fk_comment_likes_comment FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
  CONSTRAINT fk_comment_likes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_blocks (
  blocker_id BIGINT NOT NULL,
  blocked_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (blocker_id, blocked_id),
  INDEX idx_user_blocks_blocked (blocked_id, created_at),
  CONSTRAINT fk_user_blocks_blocker FOREIGN KEY (blocker_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_user_blocks_blocked FOREIGN KEY (blocked_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS conversations (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  kind VARCHAR(16) NOT NULL DEFAULT 'direct',
  name VARCHAR(120) NOT NULL DEFAULT '',
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_conversations_updated (updated_at),
  CONSTRAINT fk_conversations_creator FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS conversation_members (
  conversation_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  joined_at DATETIME(6) NOT NULL,
  last_read_at DATETIME(6) NULL,
  PRIMARY KEY (conversation_id, user_id),
  INDEX idx_conversation_members_user (user_id, conversation_id),
  CONSTRAINT fk_conversation_members_conversation FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
  CONSTRAINT fk_conversation_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS messages (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  conversation_id BIGINT NOT NULL,
  sender_id BIGINT NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL,
  INDEX idx_messages_conversation (conversation_id, created_at, id),
  CONSTRAINT fk_messages_conversation FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
  CONSTRAINT fk_messages_sender FOREIGN KEY (sender_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS oauth_settings (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  provider_key VARCHAR(64) NOT NULL DEFAULT 'oidc',
  provider_name VARCHAR(80) NOT NULL DEFAULT 'OAuth',
  client_id VARCHAR(255) NOT NULL DEFAULT '',
  client_secret_ciphertext TEXT NOT NULL,
  authorization_url VARCHAR(500) NOT NULL DEFAULT '',
  token_url VARCHAR(500) NOT NULL DEFAULT '',
  userinfo_url VARCHAR(500) NOT NULL DEFAULT '',
  scopes VARCHAR(500) NOT NULL DEFAULT 'openid email profile',
  token_auth_method VARCHAR(32) NOT NULL DEFAULT 'client_secret_post',
  require_verified_email TINYINT(1) NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS oauth_accounts (
  provider_key VARCHAR(64) NOT NULL,
  subject VARCHAR(191) NOT NULL,
  user_id BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  last_login_at DATETIME(6) NOT NULL,
  PRIMARY KEY (provider_key, subject),
  UNIQUE KEY uq_oauth_provider_user (provider_key, user_id),
  INDEX idx_oauth_accounts_user (user_id),
  CONSTRAINT fk_oauth_accounts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS oauth_login_states (
  state_hash VARBINARY(32) NOT NULL PRIMARY KEY,
  code_verifier VARCHAR(128) NOT NULL,
  return_to VARCHAR(255) NOT NULL DEFAULT '/',
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_oauth_states_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
