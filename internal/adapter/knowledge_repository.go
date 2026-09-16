package adapter

import (
	"context"
	"errors"

	knowledgeapp "xianyu-go/internal/application/knowledge"
	"xianyu-go/internal/db"
)

// KnowledgeRepository 将知识库 DB 持久化模型适配为应用层最小 Repository Port。
type KnowledgeRepository struct {
	// store 是包含知识库与归属隔离能力的进程 Store，不暴露给 HTTP。
	store *db.Store
}

// NewKnowledgeRepository 创建知识库持久化适配器。
func NewKnowledgeRepository(store *db.Store) *KnowledgeRepository {
	return &KnowledgeRepository{store: store}
}

// ListBases 返回当前用户的知识库应用模型。
func (repository *KnowledgeRepository) ListBases(ctx context.Context, userID int64) ([]knowledgeapp.Base, error) {
	// dependencyErr 是知识库 Store 在组合期是否完成初始化的校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return nil, dependencyErr
	}
	// rows 是 DB 层返回的知识库持久化模型。
	rows, listErr := repository.store.Knowledge.ListBases(ctx, userID)
	if listErr != nil {
		return nil, mapKnowledgeError(listErr)
	}
	// result 是不含 DB 对象的知识库应用模型。
	result := make([]knowledgeapp.Base, 0, len(rows))
	// row 是当前待转换的知识库持久化模型。
	for _, row := range rows {
		result = append(result, knowledgeBaseModel(row))
	}
	return result, nil
}

// GetBase 返回当前用户拥有的单个知识库应用模型。
func (repository *KnowledgeRepository) GetBase(ctx context.Context, userID, knowledgeBaseID int64) (knowledgeapp.Base, error) {
	// dependencyErr 是读取单个知识库前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return knowledgeapp.Base{}, dependencyErr
	}
	// row 是 DB 层返回的单个知识库持久化模型。
	row, getErr := repository.store.Knowledge.GetBase(ctx, userID, knowledgeBaseID)
	if getErr != nil {
		return knowledgeapp.Base{}, mapKnowledgeError(getErr)
	}
	return knowledgeBaseModel(row), nil
}

// CreateBase 创建草稿知识库并保存经应用校验的范围。
func (repository *KnowledgeRepository) CreateBase(ctx context.Context, userID int64, draft knowledgeapp.BaseDraft) (int64, error) {
	// dependencyErr 是创建草稿知识库前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return 0, dependencyErr
	}
	// knowledgeBaseID 是 DB 层创建的草稿知识库主键。
	knowledgeBaseID, createErr := repository.store.Knowledge.CreateBase(ctx, userID, knowledgeBaseDraftRow(draft))
	return knowledgeBaseID, mapKnowledgeError(createErr)
}

// UpdateBase 更新知识库文案并原子替换范围。
func (repository *KnowledgeRepository) UpdateBase(ctx context.Context, userID, knowledgeBaseID int64, draft knowledgeapp.BaseDraft) error {
	// dependencyErr 是更新知识库范围前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.UpdateBase(ctx, userID, knowledgeBaseID, knowledgeBaseDraftRow(draft)))
}

// SetBaseStatus 切换当前用户知识库的草稿／启用状态。
func (repository *KnowledgeRepository) SetBaseStatus(ctx context.Context, userID, knowledgeBaseID int64, status knowledgeapp.BaseStatus) error {
	// dependencyErr 是切换知识库状态前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.SetBaseStatus(ctx, userID, knowledgeBaseID, string(status)))
}

// DeleteBase 删除当前用户的知识库。
func (repository *KnowledgeRepository) DeleteBase(ctx context.Context, userID, knowledgeBaseID int64) error {
	// dependencyErr 是删除知识库前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.DeleteBase(ctx, userID, knowledgeBaseID))
}

// ListEntries 返回当前知识库下的 FAQ 和文档应用模型。
func (repository *KnowledgeRepository) ListEntries(ctx context.Context, userID, knowledgeBaseID int64) ([]knowledgeapp.Entry, error) {
	// dependencyErr 是读取 FAQ／文档列表前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return nil, dependencyErr
	}
	// rows 是 DB 层返回的 FAQ 和文档持久化模型。
	rows, listErr := repository.store.Knowledge.ListEntries(ctx, userID, knowledgeBaseID)
	if listErr != nil {
		return nil, mapKnowledgeError(listErr)
	}
	return knowledgeEntryModels(rows), nil
}

// GetEntry 返回当前用户拥有的单个 FAQ 或文档。
func (repository *KnowledgeRepository) GetEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType knowledgeapp.ContentType) (knowledgeapp.Entry, error) {
	// dependencyErr 是读取单个 FAQ／文档前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return knowledgeapp.Entry{}, dependencyErr
	}
	// row 是 DB 层返回的单个条目持久化模型。
	row, getErr := repository.store.Knowledge.GetEntry(ctx, userID, knowledgeBaseID, entryID, string(contentType))
	if getErr != nil {
		return knowledgeapp.Entry{}, mapKnowledgeError(getErr)
	}
	return knowledgeEntryModel(row), nil
}

// CreateEntry 原子创建条目和应用分块。
func (repository *KnowledgeRepository) CreateEntry(ctx context.Context, userID, knowledgeBaseID int64, draft knowledgeapp.EntryDraft, chunks []knowledgeapp.Chunk) (int64, error) {
	// dependencyErr 是创建 FAQ／文档和分块前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return 0, dependencyErr
	}
	// entryID 是 DB 层创建的 FAQ 或文档主键。
	entryID, createErr := repository.store.Knowledge.CreateEntry(ctx, userID, knowledgeBaseID, knowledgeEntryDraftRow(draft), knowledgeChunkRows(chunks))
	return entryID, mapKnowledgeError(createErr)
}

// UpdateEntry 原子更新条目、重置审核状态并替换分块。
func (repository *KnowledgeRepository) UpdateEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, draft knowledgeapp.EntryDraft, chunks []knowledgeapp.Chunk) error {
	// dependencyErr 是更新 FAQ／文档和分块前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.UpdateEntry(ctx, userID, knowledgeBaseID, entryID, knowledgeEntryDraftRow(draft), knowledgeChunkRows(chunks)))
}

// ReviewEntry 保存单个 FAQ 或文档的人工审核结果。
func (repository *KnowledgeRepository) ReviewEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType knowledgeapp.ContentType, reviewed bool) error {
	// dependencyErr 是保存人工审核结果前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.ReviewEntry(ctx, userID, knowledgeBaseID, entryID, string(contentType), reviewed))
}

// SetEntryEnabled 切换单个已审核条目的离线检索状态。
func (repository *KnowledgeRepository) SetEntryEnabled(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType knowledgeapp.ContentType, enabled bool) error {
	// dependencyErr 是切换条目离线检索状态前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.SetEntryEnabled(ctx, userID, knowledgeBaseID, entryID, string(contentType), enabled))
}

// DeleteEntry 删除单个 FAQ 或文档及其分块。
func (repository *KnowledgeRepository) DeleteEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType knowledgeapp.ContentType) error {
	// dependencyErr 是删除 FAQ／文档和分块前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.DeleteEntry(ctx, userID, knowledgeBaseID, entryID, string(contentType)))
}

// ListFAQAliases 返回当前用户指定 FAQ 的相似问法应用模型。
func (repository *KnowledgeRepository) ListFAQAliases(ctx context.Context, userID, knowledgeBaseID, faqID int64) ([]knowledgeapp.FAQAlias, error) {
	// dependencyErr 是读取相似问法前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return nil, dependencyErr
	}
	// rows 是 DB 层返回的 FAQ 相似问法持久化模型。
	rows, listErr := repository.store.Knowledge.ListFAQAliases(ctx, userID, knowledgeBaseID, faqID)
	if listErr != nil {
		return nil, mapKnowledgeError(listErr)
	}
	// result 是不包含 DB 类型的相似问法应用模型。
	result := make([]knowledgeapp.FAQAlias, 0, len(rows))
	// row 是当前待转换的相似问法持久化模型。
	for _, row := range rows {
		result = append(result, knowledgeapp.FAQAlias{ID: row.ID, KnowledgeBaseID: row.KnowledgeBaseID, FAQID: row.FAQID, Alias: row.Alias, NormalizedAlias: row.NormalizedAlias, Source: knowledgeapp.AliasSource(row.Source), Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return result, nil
}

// FAQAliasConflict 判断同一知识库是否已有相同规范化相似问法。
func (repository *KnowledgeRepository) FAQAliasConflict(ctx context.Context, userID, knowledgeBaseID, faqID int64, normalizedAlias string) (bool, error) {
	// dependencyErr 是检查相似问法冲突前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return false, dependencyErr
	}
	// conflict 是 DB 层用户归属和唯一性查询结果。
	conflict, conflictErr := repository.store.Knowledge.FAQAliasConflict(ctx, userID, knowledgeBaseID, faqID, normalizedAlias)
	return conflict, mapKnowledgeError(conflictErr)
}

// CreateFAQAlias 创建用户确认的 FAQ 相似问法。
func (repository *KnowledgeRepository) CreateFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID int64, draft knowledgeapp.FAQAliasDraft, normalizedAlias string) (int64, error) {
	// dependencyErr 是创建相似问法前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return 0, dependencyErr
	}
	// aliasID 是 DB 层创建的相似问法主键。
	aliasID, createErr := repository.store.Knowledge.CreateFAQAlias(ctx, userID, knowledgeBaseID, faqID, draft.Alias, normalizedAlias, string(draft.Source))
	return aliasID, mapKnowledgeError(createErr)
}

// DeleteFAQAlias 删除当前用户指定 FAQ 的单条相似问法。
func (repository *KnowledgeRepository) DeleteFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID, aliasID int64) error {
	// dependencyErr 是删除相似问法前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return mapKnowledgeError(repository.store.Knowledge.DeleteFAQAlias(ctx, userID, knowledgeBaseID, faqID, aliasID))
}

// ListRetrievableEntries 返回已启用知识库中当前生效的已审核条目。
func (repository *KnowledgeRepository) ListRetrievableEntries(ctx context.Context, userID int64, knowledgeBaseIDs []int64, nowUnix int64) ([]knowledgeapp.Entry, error) {
	// dependencyErr 是读取已审核检索候选前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return nil, dependencyErr
	}
	// rows 是 DB 层返回的离线检索候选。
	rows, listErr := repository.store.Knowledge.ListRetrievableEntries(ctx, userID, knowledgeBaseIDs, nowUnix)
	if listErr != nil {
		return nil, mapKnowledgeError(listErr)
	}
	return knowledgeEntryModels(rows), nil
}

// AddRetrieveLog 只写入问题摘要、门禁结论和证据数。
func (repository *KnowledgeRepository) AddRetrieveLog(ctx context.Context, userID int64, queryDigest string, answerability knowledgeapp.Answerability, evidenceCount int) error {
	// dependencyErr 是写入脱敏检索日志前的 Store 初始化校验结果。
	if dependencyErr := repository.validate(); dependencyErr != nil {
		return dependencyErr
	}
	return repository.store.Knowledge.AddRetrieveLog(ctx, userID, queryDigest, string(answerability), evidenceCount)
}

// validate 确认知识库适配器和 DB Store 在组合期完成构造。
func (repository *KnowledgeRepository) validate() error {
	if repository == nil || repository.store == nil || repository.store.Knowledge == nil {
		return errors.New("知识库数据适配器未初始化")
	}
	return nil
}

// knowledgeBaseModel 将 DB 知识库读模型转换为应用模型。
func knowledgeBaseModel(row db.KnowledgeBaseRow) knowledgeapp.Base {
	// itemScopes 是应用层不依赖 DB 类型的店铺商品范围。
	itemScopes := make([]knowledgeapp.ItemScope, 0, len(row.ItemScopes))
	// scope 是当前待转换的 DB 店铺商品范围。
	for _, scope := range row.ItemScopes {
		itemScopes = append(itemScopes, knowledgeapp.ItemScope{AccountID: scope.CookieID, ItemID: scope.ItemID})
	}
	return knowledgeapp.Base{ID: row.ID, UserID: row.UserID, Name: row.Name, Description: row.Description, Status: knowledgeapp.BaseStatus(row.Status), AccountIDs: row.CookieIDs, ItemScopes: itemScopes, EntryCount: row.EntryCount, ReviewedCount: row.ReviewedCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// knowledgeBaseDraftRow 将规范化应用输入转换为 DB 写入模型。
func knowledgeBaseDraftRow(draft knowledgeapp.BaseDraft) db.KnowledgeBaseDraftRow {
	// itemScopes 是交给 DB 层原子替换的店铺商品范围。
	itemScopes := make([]db.KnowledgeItemScopeRow, 0, len(draft.ItemScopes))
	// scope 是当前待转换的应用层店铺商品范围。
	for _, scope := range draft.ItemScopes {
		itemScopes = append(itemScopes, db.KnowledgeItemScopeRow{CookieID: scope.AccountID, ItemID: scope.ItemID})
	}
	return db.KnowledgeBaseDraftRow{Name: draft.Name, Description: draft.Description, CookieIDs: draft.AccountIDs, ItemScopes: itemScopes}
}

// knowledgeEntryModels 批量将 DB FAQ／文档转换为应用模型。
func knowledgeEntryModels(rows []db.KnowledgeEntryRow) []knowledgeapp.Entry {
	// result 是不含 DB 对象的 FAQ／文档应用模型列表。
	result := make([]knowledgeapp.Entry, 0, len(rows))
	// row 是当前待转换的 FAQ／文档持久化模型。
	for _, row := range rows {
		result = append(result, knowledgeEntryModel(row))
	}
	return result
}

// knowledgeEntryModel 将单个 DB FAQ／文档转换为应用模型。
func knowledgeEntryModel(row db.KnowledgeEntryRow) knowledgeapp.Entry {
	return knowledgeapp.Entry{ID: row.ID, KnowledgeBaseID: row.KnowledgeBaseID, KnowledgeBaseName: row.KnowledgeBaseName, Type: knowledgeapp.ContentType(row.Type), Title: row.Title, Content: row.Content, ContentType: row.ContentType, Status: knowledgeapp.EntryStatus(row.Status), ReviewStatus: knowledgeapp.ReviewStatus(row.ReviewStatus), RiskLevel: knowledgeapp.RiskLevel(row.RiskLevel), RequiresLiveData: row.RequiresLiveData, AllowAutoReply: row.AllowAutoReply, Enabled: row.Enabled, Aliases: row.Aliases, EffectiveFrom: row.EffectiveFrom, EffectiveTo: row.EffectiveTo, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// knowledgeEntryDraftRow 将规范化 FAQ／文档输入转换为 DB 写入模型。
func knowledgeEntryDraftRow(draft knowledgeapp.EntryDraft) db.KnowledgeEntryDraftRow {
	return db.KnowledgeEntryDraftRow{Type: string(draft.Type), Title: draft.Title, Content: draft.Content, ContentType: draft.ContentType, ContentHash: knowledgeapp.ContentDigest(draft.Title + "\n" + draft.Content), RiskLevel: string(draft.RiskLevel), RequiresLiveData: draft.RequiresLiveData, EffectiveFrom: draft.EffectiveFrom, EffectiveTo: draft.EffectiveTo}
}

// knowledgeChunkRows 将应用层确定性分块转换为 DB 写入模型。
func knowledgeChunkRows(chunks []knowledgeapp.Chunk) []db.KnowledgeChunkRow {
	// result 是交给 DB 层原子写入的分块列表。
	result := make([]db.KnowledgeChunkRow, 0, len(chunks))
	// chunk 是当前待转换的应用层文本片段。
	for _, chunk := range chunks {
		result = append(result, db.KnowledgeChunkRow{Index: chunk.Index, Content: chunk.Content, ContentHash: chunk.ContentHash})
	}
	return result
}

// mapKnowledgeError 将 DB 归属与不存在错误转换为应用层稳定错误。
func mapKnowledgeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, db.ErrNotFound) {
		return knowledgeapp.ErrNotFound
	}
	if errors.Is(err, db.ErrForbidden) {
		return knowledgeapp.ErrForbidden
	}
	return err
}

// 编译期确认知识库适配器完整实现应用层最小 Port。
var _ knowledgeapp.Repository = (*KnowledgeRepository)(nil)
