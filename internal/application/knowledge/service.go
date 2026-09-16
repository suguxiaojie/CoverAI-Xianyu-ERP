package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidInput 表示知识库用例输入不完整或超出约束。
var ErrInvalidInput = errors.New("知识库输入无效")

// ErrNotFound 表示当前用户范围内不存在目标知识资源。
var ErrNotFound = errors.New("知识资源不存在")

// ErrForbidden 表示用户试图绑定不属于自己的店铺或商品。
var ErrForbidden = errors.New("无权操作该知识资源")

// ValidationError 是可安全映射为 HTTP 400 的稳定输入错误。
type ValidationError struct {
	// Message 是不含 SQL、凭证或原始聊天内容的用户提示。
	Message string
}

// Error 返回可展示的稳定输入错误文案。
func (validationErr *ValidationError) Error() string {
	if validationErr == nil || strings.TrimSpace(validationErr.Message) == "" {
		return ErrInvalidInput.Error()
	}
	return validationErr.Message
}

// Repository 定义知识库应用用例需要的最小持久化能力。
// 每个方法都必须使用 userID 做归属隔离，不得仅依赖资源 ID。
type Repository interface {
	// ListBases 返回当前用户的知识库、范围和审核数量。
	ListBases(ctx context.Context, userID int64) ([]Base, error)
	// GetBase 返回当前用户拥有的单个知识库。
	GetBase(ctx context.Context, userID, knowledgeBaseID int64) (Base, error)
	// CreateBase 创建草稿知识库并原子保存店铺／商品范围。
	CreateBase(ctx context.Context, userID int64, draft BaseDraft) (int64, error)
	// UpdateBase 更新知识库文案并原子替换店铺／商品范围。
	UpdateBase(ctx context.Context, userID, knowledgeBaseID int64, draft BaseDraft) error
	// SetBaseStatus 在草稿和已启用之间切换知识库。
	SetBaseStatus(ctx context.Context, userID, knowledgeBaseID int64, status BaseStatus) error
	// DeleteBase 删除当前用户的知识库及其从属内容。
	DeleteBase(ctx context.Context, userID, knowledgeBaseID int64) error
	// ListEntries 返回知识库下全部 FAQ 和文档。
	ListEntries(ctx context.Context, userID, knowledgeBaseID int64) ([]Entry, error)
	// GetEntry 返回当前用户拥有的单个 FAQ 或文档。
	GetEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType) (Entry, error)
	// CreateEntry 原子创建待审核条目及应用层分块。
	CreateEntry(ctx context.Context, userID, knowledgeBaseID int64, draft EntryDraft, chunks []Chunk) (int64, error)
	// UpdateEntry 原子更新条目、重置审核状态并替换分块。
	UpdateEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, draft EntryDraft, chunks []Chunk) error
	// ReviewEntry 保存人工审核结果，通过时将内容状态设为 active。
	ReviewEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType, reviewed bool) error
	// SetEntryEnabled 切换已审核条目是否可进入离线检索。
	SetEntryEnabled(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType, enabled bool) error
	// DeleteEntry 删除当前用户的单个条目及其分块。
	DeleteEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType) error
	// ListFAQAliases 返回当前用户指定 FAQ 的相似问法。
	ListFAQAliases(ctx context.Context, userID, knowledgeBaseID, faqID int64) ([]FAQAlias, error)
	// FAQAliasConflict 判断同一知识库是否已有相同规范化相似问法。
	FAQAliasConflict(ctx context.Context, userID, knowledgeBaseID, faqID int64, normalizedAlias string) (bool, error)
	// CreateFAQAlias 创建用户确认的相似问法。
	CreateFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID int64, draft FAQAliasDraft, normalizedAlias string) (int64, error)
	// DeleteFAQAlias 删除当前用户指定 FAQ 的单条相似问法。
	DeleteFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID, aliasID int64) error
	// ListRetrievableEntries 返回已启用知识库中当前生效的已审核条目。
	ListRetrievableEntries(ctx context.Context, userID int64, knowledgeBaseIDs []int64, nowUnix int64) ([]Entry, error)
	// AddRetrieveLog 只写入问题摘要和门禁结论，不持久化原始问题。
	AddRetrieveLog(ctx context.Context, userID int64, queryDigest string, answerability Answerability, evidenceCount int) error
}

// Service 编排知识库校验、分块、审核和离线检索。
type Service struct {
	// repository 是由组合层注入的最小持久化 Port。
	repository Repository
}

// NewService 创建知识库应用服务；缺失 Repository 时后续用例将明确拒绝。
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// ListBases 校验用户后返回其全部知识库。
func (service *Service) ListBases(ctx context.Context, userID int64) ([]Base, error) {
	// validationErr 是列表用例的服务依赖和用户身份校验结果。
	if validationErr := service.validateUser(userID); validationErr != nil {
		return nil, validationErr
	}
	return service.repository.ListBases(ctx, userID)
}

// GetBase 返回当前用户拥有的知识库。
func (service *Service) GetBase(ctx context.Context, userID, knowledgeBaseID int64) (Base, error) {
	// validationErr 是单知识库读取所需的用户和资源标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return Base{}, validationErr
	}
	return service.repository.GetBase(ctx, userID, knowledgeBaseID)
}

// CreateBase 校验名称与范围后创建默认草稿知识库。
func (service *Service) CreateBase(ctx context.Context, userID int64, draft BaseDraft) (int64, error) {
	// validationErr 是创建知识库前的服务依赖和用户身份校验结果。
	if validationErr := service.validateUser(userID); validationErr != nil {
		return 0, validationErr
	}
	// normalizedDraft 是去空格、去重并完成范围校验的知识库输入。
	normalizedDraft, normalizeErr := normalizeBaseDraft(draft)
	if normalizeErr != nil {
		return 0, normalizeErr
	}
	return service.repository.CreateBase(ctx, userID, normalizedDraft)
}

// UpdateBase 校验后更新知识库文案与显式范围。
func (service *Service) UpdateBase(ctx context.Context, userID, knowledgeBaseID int64, draft BaseDraft) error {
	// validationErr 是更新知识库前的用户和资源标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return validationErr
	}
	// normalizedDraft 是去空格、去重并完成范围校验的知识库输入。
	normalizedDraft, normalizeErr := normalizeBaseDraft(draft)
	if normalizeErr != nil {
		return normalizeErr
	}
	return service.repository.UpdateBase(ctx, userID, knowledgeBaseID, normalizedDraft)
}

// SetBaseStatus 只允许将知识库切换为草稿或已启用。
func (service *Service) SetBaseStatus(ctx context.Context, userID, knowledgeBaseID int64, status BaseStatus) error {
	// validationErr 是切换知识库状态前的用户和资源标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return validationErr
	}
	if status != BaseStatusDraft && status != BaseStatusActive {
		return &ValidationError{Message: "知识库状态必须是 draft 或 active"}
	}
	return service.repository.SetBaseStatus(ctx, userID, knowledgeBaseID, status)
}

// DeleteBase 删除当前用户拥有的知识库。
func (service *Service) DeleteBase(ctx context.Context, userID, knowledgeBaseID int64) error {
	// validationErr 是删除知识库前的用户和资源标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return validationErr
	}
	return service.repository.DeleteBase(ctx, userID, knowledgeBaseID)
}

// ListEntries 返回知识库下的 FAQ 和文档。
func (service *Service) ListEntries(ctx context.Context, userID, knowledgeBaseID int64) ([]Entry, error) {
	// validationErr 是读取 FAQ／文档列表前的用户和知识库标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return nil, validationErr
	}
	return service.repository.ListEntries(ctx, userID, knowledgeBaseID)
}

// CreateEntry 创建待审核且默认停用的 FAQ 或文档。
func (service *Service) CreateEntry(ctx context.Context, userID, knowledgeBaseID int64, draft EntryDraft) (int64, error) {
	// validationErr 是创建 FAQ／文档前的用户和知识库标识校验结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return 0, validationErr
	}
	// normalizedDraft 是完成类型、风险和生效边界校验的条目输入。
	normalizedDraft, normalizeErr := normalizeEntryDraft(draft)
	if normalizeErr != nil {
		return 0, normalizeErr
	}
	// chunks 是由应用层确定性生成的分块，不含向量或模型输出。
	chunks := BuildChunks(normalizedDraft)
	return service.repository.CreateEntry(ctx, userID, knowledgeBaseID, normalizedDraft, chunks)
}

// UpdateEntry 更新内容并强制重置为待审核、停用状态。
func (service *Service) UpdateEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, draft EntryDraft) error {
	// validationErr 是更新条目前的用户、知识库、条目和类型校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, entryID, draft.Type); validationErr != nil {
		return validationErr
	}
	// normalizedDraft 是完成类型、风险和生效边界校验的条目输入。
	normalizedDraft, normalizeErr := normalizeEntryDraft(draft)
	if normalizeErr != nil {
		return normalizeErr
	}
	return service.repository.UpdateEntry(ctx, userID, knowledgeBaseID, entryID, normalizedDraft, BuildChunks(normalizedDraft))
}

// ReviewEntry 保存当前用户的人工审核结果。
func (service *Service) ReviewEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType, reviewed bool) error {
	// validationErr 是保存审核结果前的用户、知识库、条目和类型校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, entryID, contentType); validationErr != nil {
		return validationErr
	}
	return service.repository.ReviewEntry(ctx, userID, knowledgeBaseID, entryID, contentType, reviewed)
}

// SetEntryEnabled 只允许已审核内容进入离线检索。
func (service *Service) SetEntryEnabled(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType, enabled bool) error {
	// validationErr 是切换离线检索状态前的用户、知识库、条目和类型校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, entryID, contentType); validationErr != nil {
		return validationErr
	}
	if enabled {
		// entry 是开启检索前重新读取的权威审核状态。
		entry, readErr := service.repository.GetEntry(ctx, userID, knowledgeBaseID, entryID, contentType)
		if readErr != nil {
			return readErr
		}
		if entry.ReviewStatus != ReviewStatusReviewed || entry.Status != EntryStatusActive {
			return &ValidationError{Message: "只有已审核的正式知识可以启用检索"}
		}
	}
	return service.repository.SetEntryEnabled(ctx, userID, knowledgeBaseID, entryID, contentType, enabled)
}

// DeleteEntry 删除单个 FAQ 或文档及其分块。
func (service *Service) DeleteEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType ContentType) error {
	// validationErr 是删除 FAQ／文档前的用户、知识库、条目和类型校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, entryID, contentType); validationErr != nil {
		return validationErr
	}
	return service.repository.DeleteEntry(ctx, userID, knowledgeBaseID, entryID, contentType)
}

// ListFAQAliases 返回当前用户指定 FAQ 的全部已确认相似问法。
func (service *Service) ListFAQAliases(ctx context.Context, userID, knowledgeBaseID, faqID int64) ([]FAQAlias, error) {
	// validationErr 是 FAQ 相似问法列表的用户和资源标识校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, faqID, ContentTypeFAQ); validationErr != nil {
		return nil, validationErr
	}
	return service.repository.ListFAQAliases(ctx, userID, knowledgeBaseID, faqID)
}

// CreateFAQAlias 校验用户归属、标准问题和跨 FAQ 冲突后创建相似问法。
func (service *Service) CreateFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID int64, draft FAQAliasDraft) (int64, error) {
	// validationErr 是 FAQ 相似问法创建所需的用户和资源标识校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, faqID, ContentTypeFAQ); validationErr != nil {
		return 0, validationErr
	}
	// alias 是去除首尾空白且即将由用户确认保存的相似问法。
	alias := strings.TrimSpace(draft.Alias)
	if len([]rune(alias)) == 0 || len([]rune(alias)) > 200 {
		return 0, &ValidationError{Message: "相似问法必须为 1 到 200 个字符"}
	}
	// source 是限制在三种明确用户动作内的相似问法来源。
	source := draft.Source
	if source == "" {
		source = AliasSourceManual
	}
	if source != AliasSourceManual && source != AliasSourceSystem && source != AliasSourceDebug {
		return 0, &ValidationError{Message: "相似问法来源无效"}
	}
	// normalizedAlias 是执行通用同义词归一化后的冲突检查文本。
	normalizedAlias := normalizeSearchPhrase(alias)
	if normalizedAlias == "" {
		return 0, &ValidationError{Message: "相似问法缺少可检索文字"}
	}
	// targetFAQ 是当前用户拥有且将关联相似问法的 FAQ。
	targetFAQ, getErr := service.repository.GetEntry(ctx, userID, knowledgeBaseID, faqID, ContentTypeFAQ)
	if getErr != nil {
		return 0, getErr
	}
	if normalizeSearchPhrase(targetFAQ.Title) == normalizedAlias {
		return 0, &ValidationError{Message: "相似问法不能与标准问题重复"}
	}
	// entries 是用于阻止相似问法与同知识库其他 FAQ 标准问题冲突的条目列表。
	entries, listErr := service.repository.ListEntries(ctx, userID, knowledgeBaseID)
	if listErr != nil {
		return 0, listErr
	}
	// entry 是当前待比较规范化标准问题的知识条目。
	for _, entry := range entries {
		if entry.Type == ContentTypeFAQ && entry.ID != faqID && normalizeSearchPhrase(entry.Title) == normalizedAlias {
			return 0, &ValidationError{Message: "相似问法已与其他 FAQ 标准问题冲突"}
		}
	}
	// conflict 表示同一知识库是否已保存该规范化问法。
	conflict, conflictErr := service.repository.FAQAliasConflict(ctx, userID, knowledgeBaseID, faqID, normalizedAlias)
	if conflictErr != nil {
		return 0, conflictErr
	}
	if conflict {
		return 0, &ValidationError{Message: "相似问法已被当前知识库使用"}
	}
	return service.repository.CreateFAQAlias(ctx, userID, knowledgeBaseID, faqID, FAQAliasDraft{Alias: alias, Source: source}, normalizedAlias)
}

// DeleteFAQAlias 删除当前用户指定 FAQ 的单条相似问法，不改变标准答案审核状态。
func (service *Service) DeleteFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID, aliasID int64) error {
	// validationErr 是删除相似问法前的用户、FAQ 和相似问法标识校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, faqID, ContentTypeFAQ); validationErr != nil {
		return validationErr
	}
	if aliasID <= 0 {
		return &ValidationError{Message: "相似问法 ID 无效"}
	}
	return service.repository.DeleteFAQAlias(ctx, userID, knowledgeBaseID, faqID, aliasID)
}

// SuggestFAQAliases 返回不落库的确定性相似问法建议，用户确认后才允许创建。
func (service *Service) SuggestFAQAliases(ctx context.Context, userID, knowledgeBaseID, faqID int64) ([]string, error) {
	// validationErr 是生成建议所需的用户和 FAQ 标识校验结果。
	if validationErr := service.validateEntryResource(userID, knowledgeBaseID, faqID, ContentTypeFAQ); validationErr != nil {
		return nil, validationErr
	}
	// faq 是用于生成通用问法变体的当前标准 FAQ。
	faq, getErr := service.repository.GetEntry(ctx, userID, knowledgeBaseID, faqID, ContentTypeFAQ)
	if getErr != nil {
		return nil, getErr
	}
	// aliases 是需要从建议中排除的既有相似问法。
	aliases, listErr := service.repository.ListFAQAliases(ctx, userID, knowledgeBaseID, faqID)
	if listErr != nil {
		return nil, listErr
	}
	return buildAliasSuggestions(faq, aliases), nil
}

// validateUser 确认服务和当前认证用户标识可用。
func (service *Service) validateUser(userID int64) error {
	if service == nil || service.repository == nil {
		return fmt.Errorf("知识库持久化服务未初始化")
	}
	if userID <= 0 {
		return &ValidationError{Message: "知识库用户身份无效"}
	}
	return nil
}

// validateResource 确认用户与知识库标识都有效。
func (service *Service) validateResource(userID, knowledgeBaseID int64) error {
	// validationErr 是通用资源校验中的服务依赖和用户身份结果。
	if validationErr := service.validateUser(userID); validationErr != nil {
		return validationErr
	}
	if knowledgeBaseID <= 0 {
		return &ValidationError{Message: "知识库 ID 无效"}
	}
	return nil
}

// validateEntryResource 确认知识库、条目标识和内容类型都合法。
func (service *Service) validateEntryResource(userID, knowledgeBaseID, entryID int64, contentType ContentType) error {
	// validationErr 是条目资源校验中的用户和知识库标识结果。
	if validationErr := service.validateResource(userID, knowledgeBaseID); validationErr != nil {
		return validationErr
	}
	if entryID <= 0 {
		return &ValidationError{Message: "知识条目 ID 无效"}
	}
	if contentType != ContentTypeFAQ && contentType != ContentTypeDocument {
		return &ValidationError{Message: "知识类型必须是 faq 或 document"}
	}
	return nil
}

// normalizeBaseDraft 去除空白、限制文案长度并对店铺／商品范围去重。
func normalizeBaseDraft(draft BaseDraft) (BaseDraft, error) {
	// name 是去除首尾空白后的知识库名称。
	name := strings.TrimSpace(draft.Name)
	// description 是去除首尾空白后的权威范围说明。
	description := strings.TrimSpace(draft.Description)
	if name == "" || len([]rune(name)) > 80 {
		return BaseDraft{}, &ValidationError{Message: "知识库名称必须为 1 到 80 个字符"}
	}
	if description == "" || len([]rune(description)) > 1000 {
		return BaseDraft{}, &ValidationError{Message: "权威内容范围必须为 1 到 1000 个字符"}
	}
	// accounts 是去空格、去重后的店铺范围。
	accounts := uniqueStrings(draft.AccountIDs)
	// items 是去空格、去重且拒绝空标识后的商品范围。
	items, itemErr := uniqueItemScopes(draft.ItemScopes)
	if itemErr != nil {
		return BaseDraft{}, itemErr
	}
	return BaseDraft{Name: name, Description: description, AccountIDs: accounts, ItemScopes: items}, nil
}

// normalizeEntryDraft 校验 FAQ／文档内容、风险等级和生效时间。
func normalizeEntryDraft(draft EntryDraft) (EntryDraft, error) {
	// title 是去除首尾空白后的 FAQ 问题或文档标题。
	title := strings.TrimSpace(draft.Title)
	// content 是去除首尾空白后的标准答案或文档正文。
	content := strings.TrimSpace(draft.Content)
	if draft.Type != ContentTypeFAQ && draft.Type != ContentTypeDocument {
		return EntryDraft{}, &ValidationError{Message: "知识类型必须是 faq 或 document"}
	}
	if title == "" || len([]rune(title)) > 200 {
		return EntryDraft{}, &ValidationError{Message: "问题或文档标题必须为 1 到 200 个字符"}
	}
	if content == "" || len([]rune(content)) > 50000 {
		return EntryDraft{}, &ValidationError{Message: "知识内容必须为 1 到 50000 个字符"}
	}
	if draft.RiskLevel == "" {
		draft.RiskLevel = RiskLevelLow
	}
	if draft.RiskLevel != RiskLevelLow && draft.RiskLevel != RiskLevelMedium && draft.RiskLevel != RiskLevelHigh {
		return EntryDraft{}, &ValidationError{Message: "风险等级必须是 low、medium 或 high"}
	}
	if draft.EffectiveFrom < 0 || draft.EffectiveTo < 0 || draft.EffectiveTo > 0 && draft.EffectiveFrom > 0 && draft.EffectiveTo <= draft.EffectiveFrom {
		return EntryDraft{}, &ValidationError{Message: "知识生效时间范围无效"}
	}
	// contentType 是文档格式；FAQ 使用稳定 faq 值。
	contentType := strings.ToLower(strings.TrimSpace(draft.ContentType))
	if draft.Type == ContentTypeFAQ {
		contentType = "faq"
	} else if contentType == "" {
		contentType = "text"
	}
	if draft.Type == ContentTypeDocument && contentType != "text" && contentType != "markdown" {
		return EntryDraft{}, &ValidationError{Message: "文档格式必须是 text 或 markdown"}
	}
	return EntryDraft{Type: draft.Type, Title: title, Content: content, ContentType: contentType, RiskLevel: draft.RiskLevel, RequiresLiveData: draft.RequiresLiveData, EffectiveFrom: draft.EffectiveFrom, EffectiveTo: draft.EffectiveTo}, nil
}

// uniqueStrings 对去空格字符串保持首次出现顺序去重。
func uniqueStrings(values []string) []string {
	// result 是去重后的有效字符串。
	result := make([]string, 0, len(values))
	// seen 记录已加入结果的标识。
	seen := make(map[string]struct{}, len(values))
	// value 是当前待规范化的范围标识。
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		// exists 表示当前范围标识是否已加入去重结果。
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// uniqueItemScopes 规范化店铺商品范围并按组合标识去重。
func uniqueItemScopes(scopes []ItemScope) ([]ItemScope, error) {
	// result 是去重后的店铺商品范围。
	result := make([]ItemScope, 0, len(scopes))
	// seen 记录已加入的店铺商品组合。
	seen := make(map[string]struct{}, len(scopes))
	// scope 是当前待规范化的商品范围。
	for _, scope := range scopes {
		scope.AccountID = strings.TrimSpace(scope.AccountID)
		scope.ItemID = strings.TrimSpace(scope.ItemID)
		if scope.AccountID == "" || scope.ItemID == "" {
			return nil, &ValidationError{Message: "商品范围必须同时包含店铺和商品 ID"}
		}
		// key 是店铺和商品组成的稳定去重标识。
		key := scope.AccountID + "\x00" + scope.ItemID
		// exists 表示当前店铺商品组合是否已加入去重结果。
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, scope)
	}
	return result, nil
}
