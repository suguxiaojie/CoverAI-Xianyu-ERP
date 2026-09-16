// Package keywords 提供关键词回复和指定商品回复的应用层用例。
// 本包只依赖消费者定义的持久化 Port，不依赖 HTTP、数据库模型或具体数据库实现。
package keywords

import (
	"context"
	"errors"
	"strings"
)

// ErrInvalidInput 表示关键词回复用例缺少有效的用户、账号或请求参数。
var ErrInvalidInput = errors.New("关键词回复参数无效")

// ErrInvalidUser 表示调用方没有提供正数用户标识。
var ErrInvalidUser = errors.New("关键词回复用户身份无效")

// ErrNotFound 表示目标账号、关键词或指定商品回复不存在。
var ErrNotFound = errors.New("关键词回复不存在")

// ErrForbidden 表示目标资源存在但不属于当前用户。
var ErrForbidden = errors.New("无权操作该关键词回复")

// supportedSystemEvents 是关键词规则允许配置的稳定结构化系统事件白名单。
var supportedSystemEvents = map[string]struct{}{
	"order_pending_payment": {}, "order_paid": {}, "order_price_adjusted": {}, "order_shipped": {},
	"order_received": {}, "order_completed": {}, "refund_requested": {}, "red_flower_requested": {},
	"red_flower_prompted": {}, "red_flower_sent": {}, "red_flower_received": {},
}

// ValidationError 表示可安全展示给 HTTP 调用方的稳定输入错误。
type ValidationError struct {
	// Message 是不包含数据库或凭证信息的用户提示。
	Message string
}

// Error 返回稳定的输入错误提示。
func (e *ValidationError) Error() string {
	if e == nil || e.Message == "" {
		return "关键词回复输入无效"
	}
	return e.Message
}

// Keyword 是关键词回复的应用层模型，不携带数据库连接或敏感凭证。
type Keyword struct {
	// ID 是关键词规则的持久化标识。
	ID int64
	// CookieID 是规则所属账号标识。
	CookieID string
	// Keyword 是触发匹配文本。
	Keyword string
	// Reply 是文字回复内容。
	Reply string
	// ItemID 是可选的商品范围标识。
	ItemID string
	// Type 是 text 或 image 回复类型。
	Type string
	// ImageURL 是 image 类型回复使用的图片地址。
	ImageURL string
}

// Draft 是创建、更新或批量替换关键词规则的业务输入。
type Draft struct {
	// Keyword 是触发匹配文本。
	Keyword string
	// Reply 是文字回复内容。
	Reply string
	// ItemID 是可选的商品范围标识。
	ItemID string
	// Type 是 text 或 image 回复类型；空值按 text 处理。
	Type string
	// ImageURL 是 image 类型回复使用的图片地址。
	ImageURL string
}

// Group 是多个独立关键词共享同一回复配置的应用模型。
type Group struct {
	// GroupID 是规则组稳定标识。
	GroupID string
	// Keywords 是任意一个命中即可触发回复的关键词集合。
	Keywords []string
	// Reply 是文字回复正文。
	Reply string
	// ItemID 是可选商品范围。
	ItemID string
	// Type 是 text 或 image。
	Type string
	// ImageURL 是图片回复地址。
	ImageURL string
	// MatchType 是 contains、excludes 或 equals。
	MatchType string
	// MessageScope 是 customer 或 system。
	MessageScope string
	// MessageScopes 是 customer、system 或两者组成的来源集合。
	MessageScopes []string
	// SystemTypes 是系统消息类型白名单。
	SystemTypes []string
	// AccountIDs 是执行该全局规则组的闲鱼店铺账号集合。
	AccountIDs []string
	// Enabled 表示该规则组是否参与客户或系统消息匹配。
	Enabled bool
	// AccountStates 是各绑定店铺独立的启用状态。
	AccountStates []AccountState
	// ReplyIntervalSeconds 是同会话重复回复冷却秒数。
	ReplyIntervalSeconds int64
	// SendDelaySeconds 是命中后等待发送的秒数。
	SendDelaySeconds int64
}

// AccountState 描述一个规则组在单个绑定店铺中的启用状态。
type AccountState struct {
	// AccountID 是绑定店铺账号标识。
	AccountID string
	// Enabled 表示该店铺是否执行规则。
	Enabled bool
}

// GroupDraft 是创建或更新多关键词规则组的输入。
type GroupDraft struct {
	// GroupID 是更新时使用的规则组标识；创建时为空。
	GroupID string
	// Keywords 是待保存的关键词集合。
	Keywords []string
	// Reply 是文字回复正文。
	Reply string
	// ItemID 是可选商品范围。
	ItemID string
	// Type 是 text 或 image。
	Type string
	// ImageURL 是图片回复地址。
	ImageURL string
	// MatchType 是包含、不包含或等于逻辑。
	MatchType string
	// MessageScope 是客户消息或特定系统消息。
	MessageScope string
	// MessageScopes 是可同时勾选的客户消息和系统消息来源。
	MessageScopes []string
	// SystemTypes 是允许触发的系统消息 contentType 白名单。
	SystemTypes []string
	// AccountIDs 是用户勾选的适用店铺账号集合。
	AccountIDs []string
	// Enabled 是可选启用状态；未提供时新建和兼容客户端按开启处理。
	Enabled *bool
	// ReplyIntervalSeconds 是重复回复冷却秒数，范围 0 至 86400。
	ReplyIntervalSeconds int64
	// SendDelaySeconds 是命中后等待发送秒数，范围 0 至 3600。
	SendDelaySeconds int64
}

// ItemReply 是指定商品回复的应用层模型。
type ItemReply struct {
	// ItemID 是指定商品标识。
	ItemID string
	// CookieID 是回复所属账号标识。
	CookieID string
	// ReplyContent 是商品命中后的回复正文。
	ReplyContent string
}

// Repository 定义关键词用例所需的最小持久化能力。
// userID 必须由实现用于归属隔离，避免应用层把跨用户资源交给数据库操作。
type Repository interface {
	// List 返回指定用户账号的关键词规则。
	List(ctx context.Context, userID int64, cookieID string) ([]Keyword, error)
	// Add 创建一条已规范化的关键词规则。
	Add(ctx context.Context, userID int64, cookieID string, draft Draft) (int64, error)
	// Replace 删除并重建指定用户账号的全部关键词规则。
	Replace(ctx context.Context, userID int64, cookieID string, drafts []Draft) error
	// Update 更新指定用户账号中的关键词规则。
	Update(ctx context.Context, userID int64, cookieID string, id int64, draft Draft) error
	// DeleteByID 按持久化标识删除指定用户账号中的关键词规则。
	DeleteByID(ctx context.Context, userID int64, cookieID string, id int64) error
	// DeleteByIndex 按稳定 ID 顺序的零基索引删除规则。
	DeleteByIndex(ctx context.Context, userID int64, cookieID string, index int) error
	// ListGroups 返回按规则组聚合的多关键词回复。
	ListGroups(ctx context.Context, userID int64, cookieID string) ([]Group, error)
	// SaveGroup 创建或替换一个多关键词规则组。
	SaveGroup(ctx context.Context, userID int64, cookieID string, draft GroupDraft) (string, error)
	// DeleteGroup 删除一个完整多关键词规则组。
	DeleteGroup(ctx context.Context, userID int64, cookieID, groupID string) error
	// ListGlobalGroups 返回当前用户跨店铺聚合的全局规则组。
	ListGlobalGroups(ctx context.Context, userID int64) ([]Group, error)
	// SaveGlobalGroup 原子替换全局规则组的全部店铺绑定。
	SaveGlobalGroup(ctx context.Context, userID int64, draft GroupDraft) (string, error)
	// DeleteGlobalGroup 从当前用户全部店铺删除规则组。
	DeleteGlobalGroup(ctx context.Context, userID int64, groupID string) error
	// SetGlobalGroupEnabled 更新当前用户一个全局规则组的启用状态。
	SetGlobalGroupEnabled(ctx context.Context, userID int64, groupID string, enabled bool) error
	// SetAllGlobalGroupsEnabled 更新当前用户全部全局关键词规则的启用状态。
	SetAllGlobalGroupsEnabled(ctx context.Context, userID int64, enabled bool) error
	// SetGlobalGroupAccountEnabled 更新一个规则组在指定店铺中的启用状态。
	SetGlobalGroupAccountEnabled(ctx context.Context, userID int64, groupID, accountID string, enabled bool) error
	// ListItemReplies 返回指定用户全部账号的商品回复。
	ListItemReplies(ctx context.Context, userID int64) ([]ItemReply, error)
	// GetItemReply 读取指定用户账号和商品的回复。
	GetItemReply(ctx context.Context, userID int64, cookieID, itemID string) (ItemReply, error)
	// SetItemReply 覆盖指定用户账号和商品的回复。
	SetItemReply(ctx context.Context, userID int64, cookieID, itemID, content string) error
	// DeleteItemReply 删除指定用户账号和商品的回复。
	DeleteItemReply(ctx context.Context, userID int64, cookieID, itemID string) error
}

// ListGlobalGroups 查询当前用户跨店铺聚合的关键词规则组。
func (s *Service) ListGlobalGroups(ctx context.Context, userID int64) ([]Group, error) {
	if // validationErr 是全局规则列表用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return nil, validationErr
	}
	return s.repository.ListGlobalGroups(ctx, userID)
}

// SaveGlobalGroup 校验共享条件后原子保存全局规则和店铺绑定。
func (s *Service) SaveGlobalGroup(ctx context.Context, userID int64, draft GroupDraft) (string, error) {
	if // validationErr 是全局规则保存用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return "", validationErr
	}
	if len(draft.AccountIDs) == 0 {
		return "", &ValidationError{Message: "至少选择一个适用店铺"}
	}
	// validationCookie 只用于复用规则内容校验；仓储会对全部店铺重新做用户归属校验。
	validationCookie := draft.AccountIDs[0]
	return s.SaveGroup(ctx, userID, validationCookie, draft)
}

// DeleteGlobalGroup 从当前用户全部店铺删除一个全局规则组。
func (s *Service) DeleteGlobalGroup(ctx context.Context, userID int64, groupID string) error {
	if // validationErr 是全局规则删除用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return validationErr
	}
	if strings.TrimSpace(groupID) == "" {
		return &ValidationError{Message: "规则组ID不能为空"}
	}
	return s.repository.DeleteGlobalGroup(ctx, userID, groupID)
}

// SetGlobalGroupEnabled 校验用户和规则组标识后切换一个全局关键词规则。
func (s *Service) SetGlobalGroupEnabled(ctx context.Context, userID int64, groupID string, enabled bool) error {
	if // validationErr 是全局规则启停用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return validationErr
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return &ValidationError{Message: "规则组ID不能为空"}
	}
	return s.repository.SetGlobalGroupEnabled(ctx, userID, groupID, enabled)
}

// SetAllGlobalGroupsEnabled 校验用户身份后批量切换其全部全局关键词规则。
func (s *Service) SetAllGlobalGroupsEnabled(ctx context.Context, userID int64, enabled bool) error {
	if // validationErr 是全部规则启停用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return validationErr
	}
	return s.repository.SetAllGlobalGroupsEnabled(ctx, userID, enabled)
}

// SetGlobalGroupAccountEnabled 校验规则和店铺标识后切换单店铺状态。
func (s *Service) SetGlobalGroupAccountEnabled(ctx context.Context, userID int64, groupID, accountID string, enabled bool) error {
	if // validationErr 是单店铺规则启停的用户身份校验错误。
	validationErr := s.validateUser(userID); validationErr != nil {
		return validationErr
	}
	groupID, accountID = strings.TrimSpace(groupID), strings.TrimSpace(accountID)
	if groupID == "" || accountID == "" {
		return &ValidationError{Message: "规则组和店铺不能为空"}
	}
	return s.repository.SetGlobalGroupAccountEnabled(ctx, userID, groupID, accountID, enabled)
}

// ListGroups 查询指定用户账号的多关键词规则组。
func (s *Service) ListGroups(ctx context.Context, userID int64, cookieID string) ([]Group, error) {
	if // validationErr 是规则组列表依赖、用户或账号校验错误。
	err := s.validate(userID, cookieID); err != nil {
		return nil, err
	}
	return s.repository.ListGroups(ctx, userID, cookieID)
}

// SaveGroup 校验关键词集合和共享回复后创建或替换规则组。
func (s *Service) SaveGroup(ctx context.Context, userID int64, cookieID string, draft GroupDraft) (string, error) {
	if // validationErr 是规则组保存依赖、用户或账号校验错误。
	err := s.validate(userID, cookieID); err != nil {
		return "", err
	}
	// seen 保存组内已出现的小写关键词，防止重复标签。
	seen := make(map[string]struct{}, len(draft.Keywords))
	// normalizedKeywords 保存去空白、去重后的关键词。
	normalizedKeywords := make([]string, 0, len(draft.Keywords))
	// keyword 是当前规范化的组内关键词。
	for _, keyword := range draft.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		// key 是用于不区分英文大小写去重的关键词。
		key := strings.ToLower(keyword)
		// _, exists 只读取组内是否已经包含当前关键词。
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalizedKeywords = append(normalizedKeywords, keyword)
	}
	// matchType 是规范化后的匹配逻辑。
	matchType := strings.ToLower(strings.TrimSpace(draft.MatchType))
	if matchType == "" {
		matchType = "contains"
	}
	if matchType != "contains" && matchType != "excludes" && matchType != "equals" {
		return "", &ValidationError{Message: "匹配方式必须是 contains、excludes 或 equals"}
	}
	// requestedScopes 是新集合字段或旧单值字段提供的消息来源输入。
	requestedScopes := draft.MessageScopes
	if len(requestedScopes) == 0 && strings.TrimSpace(draft.MessageScope) != "" {
		requestedScopes = []string{draft.MessageScope}
	}
	if len(requestedScopes) == 0 {
		requestedScopes = []string{"customer"}
	}
	// messageScopes 保存去重后的 customer/system 来源集合。
	messageScopes := make([]string, 0, 2)
	// seenScopes 保存已经加入的消息来源。
	seenScopes := make(map[string]struct{}, 2)
	// scope 是当前规范化的消息来源。
	for _, scope := range requestedScopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope != "customer" && scope != "system" {
			return "", &ValidationError{Message: "消息来源只能是 customer 或 system"}
		}
		// _, exists 只读取消息来源是否已经加入规则组。
		if _, exists := seenScopes[scope]; !exists {
			seenScopes[scope] = struct{}{}
			messageScopes = append(messageScopes, scope)
		}
	}
	// _, includesCustomer 只读取规则是否允许客户消息。
	if _, includesCustomer := seenScopes["customer"]; includesCustomer && len(normalizedKeywords) == 0 {
		return "", &ValidationError{Message: "客户消息规则至少需要一个关键词"}
	}
	// validationKeyword 为仅系统事件规则提供不落库的校验占位词。
	validationKeyword := "__system_event__"
	if len(normalizedKeywords) > 0 {
		validationKeyword = normalizedKeywords[0]
	}
	// normalized 是复用单关键词规则校验得到的共享回复配置。
	normalized, normalizeErr := normalizeDraft(Draft{Keyword: validationKeyword, Reply: draft.Reply, ItemID: draft.ItemID, Type: draft.Type, ImageURL: draft.ImageURL})
	if normalizeErr != nil {
		return "", normalizeErr
	}
	// systemTypes 保存去重后的稳定平台 contentType 白名单。
	systemTypes := make([]string, 0, len(draft.SystemTypes))
	// systemType 是当前待校验的平台结构化消息类型。
	for _, systemType := range draft.SystemTypes {
		systemType = strings.TrimSpace(systemType)
		// _, supported 只读取当前事件是否位于可配置白名单。
		if _, supported := supportedSystemEvents[systemType]; supported {
			systemTypes = append(systemTypes, systemType)
		}
	}
	// _, includesSystem 只读取规则组是否允许系统消息。
	if _, includesSystem := seenScopes["system"]; includesSystem && len(systemTypes) == 0 {
		return "", &ValidationError{Message: "至少选择一种系统消息"}
	}
	// enabled 是规则保存后的启用状态；旧客户端未传值时保持默认开启。
	enabled := true
	if draft.Enabled != nil {
		enabled = *draft.Enabled
	}
	if draft.ReplyIntervalSeconds < 0 || draft.ReplyIntervalSeconds > 86400 {
		return "", &ValidationError{Message: "重复回复间隔必须在 0 秒到 24 小时之间"}
	}
	if draft.SendDelaySeconds < 0 || draft.SendDelaySeconds > 3600 {
		return "", &ValidationError{Message: "发送延迟必须在 0 秒到 1 小时之间"}
	}
	// normalizedDraft 是完成内容、来源、事件白名单和启用状态校验后的规则组输入。
	normalizedDraft := GroupDraft{GroupID: strings.TrimSpace(draft.GroupID), Keywords: normalizedKeywords, Reply: normalized.Reply, ItemID: normalized.ItemID, Type: normalized.Type, ImageURL: normalized.ImageURL, MatchType: matchType, MessageScope: strings.Join(messageScopes, ","), MessageScopes: messageScopes, SystemTypes: systemTypes, AccountIDs: draft.AccountIDs, Enabled: &enabled, ReplyIntervalSeconds: draft.ReplyIntervalSeconds, SendDelaySeconds: draft.SendDelaySeconds}
	if len(draft.AccountIDs) > 0 {
		return s.repository.SaveGlobalGroup(ctx, userID, normalizedDraft)
	}
	return s.repository.SaveGroup(ctx, userID, cookieID, normalizedDraft)
}

// DeleteGroup 删除指定账号下的完整规则组。
func (s *Service) DeleteGroup(ctx context.Context, userID int64, cookieID, groupID string) error {
	if // validationErr 是规则组删除依赖、用户或账号校验错误。
	err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if strings.TrimSpace(groupID) == "" {
		return &ValidationError{Message: "规则组ID不能为空"}
	}
	return s.repository.DeleteGroup(ctx, userID, cookieID, groupID)
}

// Service 编排关键词输入校验、账号归属和持久化操作。
type Service struct {
	// repository 保存由适配器实现的最小关键词持久化 Port。
	repository Repository
}

// NewService 创建关键词回复应用服务。
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// List 查询指定用户账号的全部关键词规则。
func (s *Service) List(ctx context.Context, userID int64, cookieID string) ([]Keyword, error) {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, userID, cookieID)
}

// Add 校验并创建一条关键词规则。
func (s *Service) Add(ctx context.Context, userID int64, cookieID string, draft Draft) (int64, error) {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return 0, err
	}
	// normalized、err 保存规范化后的规则输入及校验结果。
	normalized, err := normalizeDraft(draft)
	if err != nil {
		return 0, err
	}
	return s.repository.Add(ctx, userID, cookieID, normalized)
}

// Replace 校验并原子替换指定账号的全部关键词规则。
func (s *Service) Replace(ctx context.Context, userID int64, cookieID string, drafts []Draft) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	// normalized 保存全部通过校验的批量规则输入。
	normalized := make([]Draft, 0, len(drafts))
	// draft 表示当前待规范化的批量规则输入。
	for _, draft := range drafts {
		// item、err 保存规范化规则及校验结果。
		item, err := normalizeDraft(draft)
		if err != nil {
			return err
		}
		normalized = append(normalized, item)
	}
	return s.repository.Replace(ctx, userID, cookieID, normalized)
}

// Update 校验并更新指定 ID 的关键词规则。
func (s *Service) Update(ctx context.Context, userID int64, cookieID string, id int64, draft Draft) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if id <= 0 {
		return &ValidationError{Message: "无效关键词ID"}
	}
	// normalized、err 保存规范化后的规则输入及校验结果。
	normalized, err := normalizeDraft(draft)
	if err != nil {
		return err
	}
	return s.repository.Update(ctx, userID, cookieID, id, normalized)
}

// DeleteByID 删除指定 ID 的关键词规则。
func (s *Service) DeleteByID(ctx context.Context, userID int64, cookieID string, id int64) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if id <= 0 {
		return &ValidationError{Message: "无效关键词ID"}
	}
	return s.repository.DeleteByID(ctx, userID, cookieID, id)
}

// DeleteByIndex 按规则列表中的零基索引删除关键词。
func (s *Service) DeleteByIndex(ctx context.Context, userID int64, cookieID string, index int) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if index < 0 {
		return &ValidationError{Message: "无效关键词索引"}
	}
	return s.repository.DeleteByIndex(ctx, userID, cookieID, index)
}

// ListItemReplies 查询当前用户拥有账号的指定商品回复。
func (s *Service) ListItemReplies(ctx context.Context, userID int64) ([]ItemReply, error) {
	// err 表示服务依赖或用户身份校验结果。
	if err := s.validateUser(userID); err != nil {
		return nil, err
	}
	return s.repository.ListItemReplies(ctx, userID)
}

// GetItemReply 查询指定商品回复；不存在时返回 ErrNotFound。
func (s *Service) GetItemReply(ctx context.Context, userID int64, cookieID, itemID string) (ItemReply, error) {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return ItemReply{}, err
	}
	if strings.TrimSpace(itemID) == "" {
		return ItemReply{}, &ValidationError{Message: "商品ID不能为空"}
	}
	return s.repository.GetItemReply(ctx, userID, cookieID, itemID)
}

// SetItemReply 校验商品标识并覆盖指定商品回复。
func (s *Service) SetItemReply(ctx context.Context, userID int64, cookieID, itemID, content string) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if strings.TrimSpace(itemID) == "" {
		return &ValidationError{Message: "商品ID不能为空"}
	}
	return s.repository.SetItemReply(ctx, userID, cookieID, itemID, content)
}

// DeleteItemReply 删除指定商品回复。
func (s *Service) DeleteItemReply(ctx context.Context, userID int64, cookieID, itemID string) error {
	// err 表示服务依赖、用户身份或账号标识校验结果。
	if err := s.validate(userID, cookieID); err != nil {
		return err
	}
	if strings.TrimSpace(itemID) == "" {
		return &ValidationError{Message: "商品ID不能为空"}
	}
	return s.repository.DeleteItemReply(ctx, userID, cookieID, itemID)
}

// validate 检查服务依赖、用户身份和账号标识。
func (s *Service) validate(userID int64, cookieID string) error {
	// err 表示服务依赖或用户身份校验结果。
	if err := s.validateUser(userID); err != nil {
		return err
	}
	if s.repository == nil || strings.TrimSpace(cookieID) == "" {
		return ErrInvalidInput
	}
	return nil
}

// validateUser 检查服务依赖和用户身份。
func (s *Service) validateUser(userID int64) error {
	if s == nil || s.repository == nil {
		return ErrInvalidInput
	}
	if userID <= 0 {
		return ErrInvalidUser
	}
	return nil
}

// normalizeDraft 统一回复类型和内容字段，并拒绝不完整输入。
func normalizeDraft(draft Draft) (Draft, error) {
	draft.Keyword = strings.TrimSpace(draft.Keyword)
	draft.Type = strings.ToLower(strings.TrimSpace(draft.Type))
	draft.ItemID = strings.TrimSpace(draft.ItemID)
	draft.Reply = strings.TrimSpace(draft.Reply)
	draft.ImageURL = strings.TrimSpace(draft.ImageURL)
	if draft.Keyword == "" {
		return Draft{}, &ValidationError{Message: "keyword 必填"}
	}
	if draft.Type == "" {
		draft.Type = "text"
	}
	switch draft.Type {
	case "text":
		if draft.Reply == "" {
			return Draft{}, &ValidationError{Message: "文字回复内容不能为空"}
		}
		draft.ImageURL = ""
	case "image":
		if draft.ImageURL == "" {
			return Draft{}, &ValidationError{Message: "图片回复 URL 不能为空"}
		}
		draft.Reply = ""
	default:
		return Draft{}, &ValidationError{Message: "回复类型必须是 text 或 image"}
	}
	return draft, nil
}
