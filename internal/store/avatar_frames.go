package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

var (
	ErrAvatarFrameUnavailable = errors.New("头像框不存在或已下架")
	ErrAvatarFrameForbidden   = errors.New("当前身份不能使用这个头像框")
	ErrAvatarFrameNotOwned    = errors.New("请先购买这个头像框")
	avatarFrameColorPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	avatarFrameImagePattern   = regexp.MustCompile(`^/api/v1/media/avatar-frames/[0-9a-f]{32,40}\.(png|jpe?g|gif)$`)
)

const avatarFrameColumns = `id, name, description, style, image_url, image_offset_x, image_offset_y, primary_color, secondary_color, price_cents, allowed_regular, allowed_member, allowed_admin, enabled, sort_order, sales_count, created_at, updated_at`

var avatarFrameStyles = map[string]struct{}{
	"ring":   {},
	"double": {},
	"glow":   {},
	"pixel":  {},
	"halo":   {},
	"image":  {},
}

type avatarFrameUserContext struct {
	role         string
	memberActive bool
}

func normalizeAvatarFrame(input domain.AvatarFrame) (domain.AvatarFrame, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Style = strings.ToLower(strings.TrimSpace(input.Style))
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if parsed, parseErr := url.Parse(input.ImageURL); parseErr == nil && parsed.IsAbs() && parsed.Path != "" && parsed.RawQuery == "" && parsed.Fragment == "" {
		input.ImageURL = parsed.Path
	}
	if input.ImageOffsetX < -100 || input.ImageOffsetX > 100 || input.ImageOffsetY < -100 || input.ImageOffsetY > 100 {
		return input, errors.New("头像框图片坐标需要在 -100 到 100 之间")
	}
	input.PrimaryColor = strings.ToLower(strings.TrimSpace(input.PrimaryColor))
	input.SecondaryColor = strings.ToLower(strings.TrimSpace(input.SecondaryColor))
	if len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 80 || containsControl(input.Name) {
		return input, errors.New("头像框名称需要在 2 到 80 个字符之间")
	}
	if len([]rune(input.Description)) > 240 || containsControl(input.Description) {
		return input, errors.New("头像框说明不能超过 240 个字符")
	}
	if _, ok := avatarFrameStyles[input.Style]; !ok {
		return input, errors.New("头像框样式无效")
	}
	if !avatarFrameColorPattern.MatchString(input.PrimaryColor) || !avatarFrameColorPattern.MatchString(input.SecondaryColor) {
		return input, errors.New("头像框颜色必须使用六位十六进制色值")
	}
	if input.ImageURL != "" && !avatarFrameImagePattern.MatchString(input.ImageURL) {
		return input, errors.New("头像框图片地址无效")
	}
	if input.Style == "image" && input.ImageURL == "" {
		return input, errors.New("请先上传头像框图片")
	}
	if input.PriceCents < 0 || input.PriceCents > 10_000_000 {
		return input, errors.New("头像框价格需要在 0 到 100,000 元之间")
	}
	if input.SortOrder < -10000 || input.SortOrder > 10000 {
		return input, errors.New("头像框排序值无效")
	}
	if !input.AllowedRegular && !input.AllowedMember && !input.AllowedAdmin {
		return input, errors.New("至少允许一类用户使用头像框")
	}
	return input, nil
}

func avatarFrameAllowed(frame domain.AvatarFrame, context avatarFrameUserContext) bool {
	if context.role == "admin" {
		return frame.AllowedAdmin
	}
	if context.memberActive {
		return frame.AllowedMember
	}
	return frame.AllowedRegular
}

func scanAvatarFrame(scanner rowScanner) (domain.AvatarFrame, error) {
	var item domain.AvatarFrame
	var allowedRegular, allowedMember, allowedAdmin, enabled int
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.Style, &item.ImageURL, &item.ImageOffsetX, &item.ImageOffsetY, &item.PrimaryColor, &item.SecondaryColor,
		&item.PriceCents, &allowedRegular, &allowedMember, &allowedAdmin, &enabled, &item.SortOrder,
		&item.SalesCount, &item.CreatedAt, &item.UpdatedAt,
	)
	item.AllowedRegular = allowedRegular != 0
	item.AllowedMember = allowedMember != 0
	item.AllowedAdmin = allowedAdmin != 0
	item.Enabled = enabled != 0
	return item, err
}

func getAvatarFrame(queryer rowQueryer, id int64, forUpdate bool) (domain.AvatarFrame, error) {
	if id < 1 {
		return domain.AvatarFrame{}, sql.ErrNoRows
	}
	query := `SELECT ` + avatarFrameColumns + ` FROM avatar_frames WHERE id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanAvatarFrame(queryer.QueryRow(query, id))
}

func avatarFrameContext(queryer rowQueryer, userID int64, forUpdate bool) (avatarFrameUserContext, error) {
	var context avatarFrameUserContext
	var memberActive int
	query := `SELECT role, COALESCE(membership_tier_id IS NOT NULL AND membership_expires_at > UTC_TIMESTAMP(), 0) FROM users WHERE id = ? AND status = 'active'`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	err := queryer.QueryRow(query, userID).Scan(&context.role, &memberActive)
	context.memberActive = memberActive != 0
	return context, err
}

func (s *Store) ListAvatarFrames(userID int64, includeDisabled bool) ([]domain.AvatarFrame, error) {
	context := avatarFrameUserContext{}
	var err error
	if userID > 0 {
		context, err = avatarFrameContext(s.db, userID, false)
		if err != nil {
			return nil, err
		}
	}
	where := "WHERE f.enabled = 1"
	if includeDisabled {
		where = ""
	}
	rows, err := s.db.Query(`SELECT f.`+strings.ReplaceAll(avatarFrameColumns, ", ", ", f.")+`, uf.purchased_at, COALESCE(e.frame_id IS NOT NULL, 0)
		FROM avatar_frames f
		LEFT JOIN user_avatar_frames uf ON uf.frame_id = f.id AND uf.user_id = ?
		LEFT JOIN user_avatar_frame_equipment e ON e.frame_id = f.id AND e.user_id = ?
		`+where+` ORDER BY f.sort_order, f.id`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.AvatarFrame, 0)
	for rows.Next() {
		var item domain.AvatarFrame
		var allowedRegular, allowedMember, allowedAdmin, enabled, equipped int
		var purchasedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Description, &item.Style, &item.ImageURL, &item.ImageOffsetX, &item.ImageOffsetY, &item.PrimaryColor, &item.SecondaryColor,
			&item.PriceCents, &allowedRegular, &allowedMember, &allowedAdmin, &enabled, &item.SortOrder,
			&item.SalesCount, &item.CreatedAt, &item.UpdatedAt, &purchasedAt, &equipped,
		); err != nil {
			return nil, err
		}
		item.AllowedRegular = allowedRegular != 0
		item.AllowedMember = allowedMember != 0
		item.AllowedAdmin = allowedAdmin != 0
		item.Enabled = enabled != 0
		item.Owned = purchasedAt.Valid
		if purchasedAt.Valid {
			value := purchasedAt.Time
			item.PurchasedAt = &value
		}
		item.CanUse = userID > 0 && avatarFrameAllowed(item, context)
		item.Equipped = equipped != 0 && item.Enabled && item.CanUse
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CreateAvatarFrame(actorID int64, input domain.AvatarFrame) (domain.AvatarFrame, error) {
	input, err := normalizeAvatarFrame(input)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`INSERT INTO avatar_frames (name, description, style, image_url, image_offset_x, image_offset_y, primary_color, secondary_color, price_cents, allowed_regular, allowed_member, allowed_admin, enabled, sort_order, sales_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`, input.Name, input.Description, input.Style, input.ImageURL, input.ImageOffsetX, input.ImageOffsetY, input.PrimaryColor, input.SecondaryColor, input.PriceCents, input.AllowedRegular, input.AllowedMember, input.AllowedAdmin, input.Enabled, input.SortOrder, now, now)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	item, err := getAvatarFrame(s.db, id, false)
	if err == nil {
		_, _ = s.db.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'avatar_frame', ?, 'created', ?, ?)`, actorID, id, item.Name, now)
	}
	return item, err
}

func (s *Store) UpdateAvatarFrame(actorID, id int64, input domain.AvatarFrame) (domain.AvatarFrame, error) {
	input, err := normalizeAvatarFrame(input)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`UPDATE avatar_frames SET name = ?, description = ?, style = ?, image_url = ?, image_offset_x = ?, image_offset_y = ?, primary_color = ?, secondary_color = ?, price_cents = ?, allowed_regular = ?, allowed_member = ?, allowed_admin = ?, enabled = ?, sort_order = ?, updated_at = ? WHERE id = ?`, input.Name, input.Description, input.Style, input.ImageURL, input.ImageOffsetX, input.ImageOffsetY, input.PrimaryColor, input.SecondaryColor, input.PriceCents, input.AllowedRegular, input.AllowedMember, input.AllowedAdmin, input.Enabled, input.SortOrder, now, id)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.AvatarFrame{}, err
		}
		return domain.AvatarFrame{}, sql.ErrNoRows
	}
	if !input.Enabled {
		if _, err := s.db.Exec(`DELETE FROM user_avatar_frame_equipment WHERE frame_id = ?`, id); err != nil {
			return domain.AvatarFrame{}, err
		}
	}
	item, err := getAvatarFrame(s.db, id, false)
	if err == nil {
		_, _ = s.db.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'avatar_frame', ?, 'updated', ?, ?)`, actorID, id, item.Name, now)
	}
	return item, err
}

func (s *Store) DisableAvatarFrame(actorID, id int64) error {
	item, err := getAvatarFrame(s.db, id, false)
	if err != nil {
		return err
	}
	item.Enabled = false
	_, err = s.UpdateAvatarFrame(actorID, id, item)
	return err
}

func (s *Store) PurchaseAvatarFrame(userID, frameID int64) (domain.AvatarFrame, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	defer tx.Rollback()
	context, err := avatarFrameContext(tx, userID, true)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	frame, err := getAvatarFrame(tx, frameID, true)
	if errors.Is(err, sql.ErrNoRows) || !frame.Enabled {
		return domain.AvatarFrame{}, ErrAvatarFrameUnavailable
	}
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	if !avatarFrameAllowed(frame, context) {
		return domain.AvatarFrame{}, ErrAvatarFrameForbidden
	}
	var purchasedAt time.Time
	if err := tx.QueryRow(`SELECT purchased_at FROM user_avatar_frames WHERE user_id = ? AND frame_id = ? FOR UPDATE`, userID, frameID).Scan(&purchasedAt); err == nil {
		if err := tx.Commit(); err != nil {
			return domain.AvatarFrame{}, err
		}
		return s.avatarFrameForUser(userID, frameID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.AvatarFrame{}, err
	}
	available, err := walletBalance(tx, userID)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	if available < frame.PriceCents {
		return domain.AvatarFrame{}, errors.New("钱包余额不足")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`INSERT INTO user_avatar_frames (user_id, frame_id, price_paid_cents, purchased_at) VALUES (?, ?, ?, ?)`, userID, frameID, frame.PriceCents, now); err != nil {
		return domain.AvatarFrame{}, err
	}
	if frame.PriceCents > 0 {
		if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'avatar_frame_purchase', ?, 'avatar_frame', ?, ?, ?)`, userID, -frame.PriceCents, frameID, fmt.Sprintf("购买头像框：%s", frame.Name), now); err != nil {
			return domain.AvatarFrame{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE avatar_frames SET sales_count = sales_count + 1, updated_at = ? WHERE id = ?`, now, frameID); err != nil {
		return domain.AvatarFrame{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AvatarFrame{}, err
	}
	return s.avatarFrameForUser(userID, frameID)
}

func (s *Store) EquipAvatarFrame(userID, frameID int64) (*domain.AvatarFrame, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	context, err := avatarFrameContext(tx, userID, true)
	if err != nil {
		return nil, err
	}
	if frameID == 0 {
		if _, err := tx.Exec(`DELETE FROM user_avatar_frame_equipment WHERE user_id = ?`, userID); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}
	frame, err := getAvatarFrame(tx, frameID, true)
	if errors.Is(err, sql.ErrNoRows) || !frame.Enabled {
		return nil, ErrAvatarFrameUnavailable
	}
	if err != nil {
		return nil, err
	}
	if !avatarFrameAllowed(frame, context) {
		return nil, ErrAvatarFrameForbidden
	}
	var owned int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user_avatar_frames WHERE user_id = ? AND frame_id = ?`, userID, frameID).Scan(&owned); err != nil {
		return nil, err
	}
	if owned == 0 {
		return nil, ErrAvatarFrameNotOwned
	}
	if _, err := tx.Exec(`INSERT INTO user_avatar_frame_equipment (user_id, frame_id, equipped_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE frame_id = VALUES(frame_id), equipped_at = VALUES(equipped_at)`, userID, frameID, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	item, err := s.avatarFrameForUser(userID, frameID)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) avatarFrameForUser(userID, frameID int64) (domain.AvatarFrame, error) {
	items, err := s.ListAvatarFrames(userID, true)
	if err != nil {
		return domain.AvatarFrame{}, err
	}
	for _, item := range items {
		if item.ID == frameID {
			return item, nil
		}
	}
	return domain.AvatarFrame{}, sql.ErrNoRows
}

func equippedAvatarFrames(queryer sqlQueryer, userIDs []int64) (map[int64]*domain.AvatarFrame, error) {
	result := make(map[int64]*domain.AvatarFrame)
	if len(userIDs) == 0 {
		return result, nil
	}
	seen := make(map[int64]struct{}, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		if id < 1 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	if len(args) == 0 {
		return result, nil
	}
	rows, err := queryer.Query(`SELECT e.user_id, f.`+strings.ReplaceAll(avatarFrameColumns, ", ", ", f.")+`, u.role, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0)
		FROM user_avatar_frame_equipment e
		JOIN avatar_frames f ON f.id = e.frame_id
		JOIN users u ON u.id = e.user_id
		WHERE e.user_id IN (`+strings.Join(placeholders, ",")+`) AND f.enabled = 1 AND u.status = 'active'`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var item domain.AvatarFrame
		var allowedRegular, allowedMember, allowedAdmin, enabled, memberActive int
		var role string
		if err := rows.Scan(
			&userID, &item.ID, &item.Name, &item.Description, &item.Style, &item.ImageURL, &item.ImageOffsetX, &item.ImageOffsetY, &item.PrimaryColor, &item.SecondaryColor,
			&item.PriceCents, &allowedRegular, &allowedMember, &allowedAdmin, &enabled, &item.SortOrder,
			&item.SalesCount, &item.CreatedAt, &item.UpdatedAt, &role, &memberActive,
		); err != nil {
			return nil, err
		}
		item.AllowedRegular = allowedRegular != 0
		item.AllowedMember = allowedMember != 0
		item.AllowedAdmin = allowedAdmin != 0
		item.Enabled = enabled != 0
		item.Owned = true
		item.Equipped = true
		item.CanUse = avatarFrameAllowed(item, avatarFrameUserContext{role: role, memberActive: memberActive != 0})
		if item.CanUse {
			copy := item
			result[userID] = &copy
		}
	}
	return result, rows.Err()
}

func attachAvatarFramesToPosts(queryer sqlQueryer, posts []domain.Post) error {
	ids := make([]int64, 0, len(posts))
	for _, item := range posts {
		ids = append(ids, item.AuthorID)
	}
	frames, err := equippedAvatarFrames(queryer, ids)
	if err != nil {
		return err
	}
	for index := range posts {
		posts[index].AuthorAvatarFrame = frames[posts[index].AuthorID]
	}
	return nil
}

func attachAvatarFramesToComments(queryer sqlQueryer, comments []domain.Comment) error {
	ids := make([]int64, 0, len(comments))
	for _, item := range comments {
		ids = append(ids, item.AuthorID)
	}
	frames, err := equippedAvatarFrames(queryer, ids)
	if err != nil {
		return err
	}
	for index := range comments {
		comments[index].AuthorAvatarFrame = frames[comments[index].AuthorID]
	}
	return nil
}

func attachAvatarFrameToUser(queryer rowQueryer, userID int64) (*domain.AvatarFrame, error) {
	fullQueryer, ok := queryer.(sqlQueryer)
	if !ok {
		return nil, nil
	}
	frames, err := equippedAvatarFrames(fullQueryer, []int64{userID})
	return frames[userID], err
}
