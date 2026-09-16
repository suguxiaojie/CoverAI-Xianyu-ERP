package db

import (
	"context"
	"database/sql"
	"strings"
)

// SearchSessions 按用户和账号归属搜索会话摘要及未撤回历史消息，不返回匹配消息原文。
func (s *ChatStore) SearchSessions(ctx context.Context, userID int64, cookieID, search string, limit int) ([]ChatSession, error) {
	if s == nil || s.DB == nil || userID <= 0 || strings.TrimSpace(cookieID) == "" || strings.TrimSpace(search) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	// pattern 是转义 LIKE 通配符后的不区分 ASCII 大小写子串模式。
	pattern := chatSessionSearchPattern(search)
	// rows、queryErr 是账号范围内匹配会话摘要或历史消息的查询结果和错误。
	rows, queryErr := s.DB.QueryContext(ctx, `SELECT cs.cookie_id,cs.chat_id,cs.buyer_id,cs.buyer_name,cs.buyer_avatar_url,
		cs.item_id,cs.item_title,cs.last_message,cs.last_message_at,cs.unread_count,cs.is_pinned,cs.pinned_at
		FROM chat_sessions cs JOIN cookies account ON account.id=cs.cookie_id
		WHERE account.user_id=? AND cs.cookie_id=? AND (
			LOWER(cs.buyer_name) LIKE ? ESCAPE '!' OR LOWER(cs.buyer_id) LIKE ? ESCAPE '!' OR
			LOWER(cs.item_title) LIKE ? ESCAPE '!' OR LOWER(cs.last_message) LIKE ? ESCAPE '!' OR
			EXISTS (SELECT 1 FROM chat_messages message
				WHERE message.cookie_id=cs.cookie_id AND message.chat_id=cs.chat_id AND message.status<>'recalled' AND (
					LOWER(message.content) LIKE ? ESCAPE '!' OR LOWER(message.system_card_title) LIKE ? ESCAPE '!' OR
					LOWER(message.system_card_description) LIKE ? ESCAPE '!' OR LOWER(message.system_card_order_id) LIKE ? ESCAPE '!')))
		ORDER BY cs.is_pinned DESC,cs.pinned_at DESC,cs.last_message_at DESC,cs.chat_id DESC LIMIT ?`,
		userID, strings.TrimSpace(cookieID), pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	return scanChatSessionSearchRows(rows)
}

// chatSessionSearchPattern 把用户输入转换为 LIKE 字面量子串模式；感叹号是跨方言统一转义符。
func chatSessionSearchPattern(search string) string {
	// normalized 是去除首尾空白并统一 ASCII 大小写的搜索文本。
	normalized := strings.ToLower(strings.TrimSpace(search))
	// escaped 依次转义转义符自身、百分号和下划线，避免用户输入扩大匹配范围。
	escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(normalized)
	return "%" + escaped + "%"
}

// scanChatSessionSearchRows 将搜索查询结果转换为会话摘要，并保留置顶与最近消息顺序。
func scanChatSessionSearchRows(rows *sql.Rows) ([]ChatSession, error) {
	// sessions 保存当前账号中命中的非敏感会话摘要。
	var sessions []ChatSession
	for rows.Next() { // rows.Next 当前指向一条匹配会话。
		// session 是当前待扫描的会话摘要。
		var session ChatSession
		if // scanErr 是当前会话字段扫描错误。
		scanErr := rows.Scan(&session.CookieID, &session.ChatID, &session.BuyerID, &session.BuyerName, &session.BuyerAvatar,
			&session.ItemID, &session.ItemTitle, &session.LastMessage, &session.LastMessageAt, &session.UnreadCount, &session.IsPinned, &session.PinnedAt); scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}
