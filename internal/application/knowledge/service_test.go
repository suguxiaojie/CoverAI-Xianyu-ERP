package knowledge

import (
	"context"
	"strings"
	"testing"
)

// knowledgeRepositoryFake 保存知识库应用测试需要的可控持久化行为。
type knowledgeRepositoryFake struct {
	// bases 是 ListBases 返回的知识库夹具。
	bases []Base
	// base 是 GetBase 返回的单个知识库夹具。
	base Base
	// entries 是列表或检索返回的知识条目夹具。
	entries []Entry
	// entry 是 SetEntryEnabled 前重新读取的单条知识夹具。
	entry Entry
	// aliases 是 ListFAQAliases 返回的相似问法夹具。
	aliases []FAQAlias
	// aliasConflict 表示规范化相似问法是否已被当前知识库占用。
	aliasConflict bool
	// createdAliasDraft 记录应用服务交给仓储的相似问法输入。
	createdAliasDraft FAQAliasDraft
	// createdNormalizedAlias 记录同义词归一化后的冲突检查文本。
	createdNormalizedAlias string
	// createdBaseDraft 记录应用服务交给仓储的规范化知识库输入。
	createdBaseDraft BaseDraft
	// createdEntryDraft 记录应用服务交给仓储的规范化条目输入。
	createdEntryDraft EntryDraft
	// createdChunks 记录应用层生成的确定性分块。
	createdChunks []Chunk
	// enabledCalled 表示已审核条目是否进入启停仓储方法。
	enabledCalled bool
	// retrieveLogDigest 记录离线调试写入的非明文问题摘要。
	retrieveLogDigest string
	// retrieveLogStatus 记录离线调试写入的门禁结论。
	retrieveLogStatus Answerability
}

// ListBases 返回可控知识库夹具。
func (fake *knowledgeRepositoryFake) ListBases(context.Context, int64) ([]Base, error) {
	return fake.bases, nil
}

// GetBase 返回可控单个知识库夹具。
func (fake *knowledgeRepositoryFake) GetBase(context.Context, int64, int64) (Base, error) {
	return fake.base, nil
}

// CreateBase 记录规范化输入并返回稳定测试主键。
func (fake *knowledgeRepositoryFake) CreateBase(_ context.Context, _ int64, draft BaseDraft) (int64, error) {
	fake.createdBaseDraft = draft
	return 17, nil
}

// UpdateBase 记录规范化知识库输入。
func (fake *knowledgeRepositoryFake) UpdateBase(_ context.Context, _, _ int64, draft BaseDraft) error {
	fake.createdBaseDraft = draft
	return nil
}

// SetBaseStatus 接受合法状态切换。
func (fake *knowledgeRepositoryFake) SetBaseStatus(context.Context, int64, int64, BaseStatus) error {
	return nil
}

// DeleteBase 接受当前用户知识库删除。
func (fake *knowledgeRepositoryFake) DeleteBase(context.Context, int64, int64) error {
	return nil
}

// ListEntries 返回可控 FAQ／文档夹具。
func (fake *knowledgeRepositoryFake) ListEntries(context.Context, int64, int64) ([]Entry, error) {
	return fake.entries, nil
}

// GetEntry 返回启停门禁使用的可控审核状态。
func (fake *knowledgeRepositoryFake) GetEntry(context.Context, int64, int64, int64, ContentType) (Entry, error) {
	return fake.entry, nil
}

// CreateEntry 记录规范化输入和应用分块。
func (fake *knowledgeRepositoryFake) CreateEntry(_ context.Context, _, _ int64, draft EntryDraft, chunks []Chunk) (int64, error) {
	fake.createdEntryDraft, fake.createdChunks = draft, chunks
	return 29, nil
}

// UpdateEntry 记录更新后的规范化输入和应用分块。
func (fake *knowledgeRepositoryFake) UpdateEntry(_ context.Context, _, _, _ int64, draft EntryDraft, chunks []Chunk) error {
	fake.createdEntryDraft, fake.createdChunks = draft, chunks
	return nil
}

// ReviewEntry 接受人工审核结果。
func (fake *knowledgeRepositoryFake) ReviewEntry(context.Context, int64, int64, int64, ContentType, bool) error {
	return nil
}

// SetEntryEnabled 记录已审核条目进入启停仓储方法。
func (fake *knowledgeRepositoryFake) SetEntryEnabled(context.Context, int64, int64, int64, ContentType, bool) error {
	fake.enabledCalled = true
	return nil
}

// DeleteEntry 接受单个 FAQ／文档删除。
func (fake *knowledgeRepositoryFake) DeleteEntry(context.Context, int64, int64, int64, ContentType) error {
	return nil
}

// ListFAQAliases 返回可控相似问法夹具。
func (fake *knowledgeRepositoryFake) ListFAQAliases(context.Context, int64, int64, int64) ([]FAQAlias, error) {
	return fake.aliases, nil
}

// FAQAliasConflict 返回可控的规范化问法冲突结果。
func (fake *knowledgeRepositoryFake) FAQAliasConflict(context.Context, int64, int64, int64, string) (bool, error) {
	return fake.aliasConflict, nil
}

// CreateFAQAlias 记录相似问法输入和规范化文本并返回稳定主键。
func (fake *knowledgeRepositoryFake) CreateFAQAlias(_ context.Context, _, _, _ int64, draft FAQAliasDraft, normalizedAlias string) (int64, error) {
	fake.createdAliasDraft, fake.createdNormalizedAlias = draft, normalizedAlias
	return 41, nil
}

// DeleteFAQAlias 接受当前用户相似问法删除。
func (fake *knowledgeRepositoryFake) DeleteFAQAlias(context.Context, int64, int64, int64, int64) error {
	return nil
}

// ListRetrievableEntries 返回可控已审核检索候选。
func (fake *knowledgeRepositoryFake) ListRetrievableEntries(context.Context, int64, []int64, int64) ([]Entry, error) {
	return fake.entries, nil
}

// AddRetrieveLog 记录问题摘要和门禁结论，不保存原始问题。
func (fake *knowledgeRepositoryFake) AddRetrieveLog(_ context.Context, _ int64, queryDigest string, answerability Answerability, _ int) error {
	fake.retrieveLogDigest, fake.retrieveLogStatus = queryDigest, answerability
	return nil
}

// TestServiceNormalizesBaseAndBuildsChunks 验证知识库范围去重和 FAQ 确定性分块。
func TestServiceNormalizesBaseAndBuildsChunks(t *testing.T) {
	// repository 是记录应用层规范化结果的仓储夹具。
	repository := &knowledgeRepositoryFake{}
	// service 是待验证的知识库应用服务。
	service := NewService(repository)
	// baseID 是创建规范化知识库返回的夹具主键。
	baseID, createErr := service.CreateBase(context.Background(), 7, BaseDraft{Name: "  使用教程  ", Description: "  已确认的使用说明  ", AccountIDs: []string{"shop-a", "shop-a"}, ItemScopes: []ItemScope{{AccountID: "shop-a", ItemID: "item-1"}, {AccountID: "shop-a", ItemID: "item-1"}}})
	if createErr != nil || baseID != 17 {
		t.Fatalf("create base id=%d err=%v", baseID, createErr)
	}
	if repository.createdBaseDraft.Name != "使用教程" || len(repository.createdBaseDraft.AccountIDs) != 1 || len(repository.createdBaseDraft.ItemScopes) != 1 {
		t.Fatalf("normalized base=%+v", repository.createdBaseDraft)
	}
	// entryID 是创建 FAQ 返回的夹具主键。
	entryID, entryErr := service.CreateEntry(context.Background(), 7, baseID, EntryDraft{Type: ContentTypeFAQ, Title: "  怎么用？ ", Content: " 请按权威教程操作。 ", RiskLevel: RiskLevelLow})
	if entryErr != nil || entryID != 29 || len(repository.createdChunks) != 1 {
		t.Fatalf("create entry id=%d chunks=%+v err=%v", entryID, repository.createdChunks, entryErr)
	}
	if repository.createdChunks[0].Content != "怎么用？\n请按权威教程操作。" || len(repository.createdChunks[0].ContentHash) != 64 {
		t.Fatalf("faq chunk=%+v", repository.createdChunks[0])
	}
}

// TestServiceRequiresReviewBeforeEnabling 验证待审核条目不能进入离线检索。
func TestServiceRequiresReviewBeforeEnabling(t *testing.T) {
	// repository 一开始返回待审核条目。
	repository := &knowledgeRepositoryFake{entry: Entry{ReviewStatus: ReviewStatusPending, Status: EntryStatusDraft}}
	// service 是待验证的知识库应用服务。
	service := NewService(repository)
	// enableErr 是待审核条目尝试启用离线检索的预期门禁错误。
	if enableErr := service.SetEntryEnabled(context.Background(), 7, 3, 5, ContentTypeFAQ, true); enableErr == nil {
		t.Fatal("待审核条目不应允许启用")
	}
	if repository.enabledCalled {
		t.Fatal("审核门禁失败时不应调用启停仓储")
	}
	repository.entry = Entry{ReviewStatus: ReviewStatusReviewed, Status: EntryStatusActive}
	// enableErr 是已审核正式条目启用离线检索的结果。
	if enableErr := service.SetEntryEnabled(context.Background(), 7, 3, 5, ContentTypeFAQ, true); enableErr != nil || !repository.enabledCalled {
		t.Fatalf("reviewed entry enable err=%v called=%v", enableErr, repository.enabledCalled)
	}
}

// TestRetrieveAppliesAnswerabilityGatesAndLogsDigest 验证低风险、实时数据和高风险门禁及脱敏日志。
func TestRetrieveAppliesAnswerabilityGatesAndLogsDigest(t *testing.T) {
	// originalClock 保存测试前的 Unix 秒时间源。
	originalClock := currentUnix
	currentUnix = func() int64 { return 100 }
	t.Cleanup(func() { currentUnix = originalClock })
	// repository 返回一条低风险使用教程。
	repository := &knowledgeRepositoryFake{entries: []Entry{{ID: 1, KnowledgeBaseID: 2, KnowledgeBaseName: "使用教程", Type: ContentTypeFAQ, Title: "这个商品怎么使用？", Content: "请按商品说明完成设置。", RiskLevel: RiskLevelLow}}}
	// service 是待验证的离线检索应用服务。
	service := NewService(repository)
	// safeResult 是使用教程问题的低风险检索结果。
	safeResult, safeErr := service.Retrieve(context.Background(), RetrieveRequest{UserID: 7, Query: "这个商品怎么使用？"})
	if safeErr != nil || safeResult.Answerability != AnswerabilityAnswerable || safeResult.Candidate == "" || len(safeResult.Evidence) != 1 {
		t.Fatalf("safe result=%+v err=%v", safeResult, safeErr)
	}
	if len(repository.retrieveLogDigest) != 64 || repository.retrieveLogDigest == safeResult.Query || repository.retrieveLogStatus != AnswerabilityAnswerable {
		t.Fatalf("retrieve log digest=%q status=%q", repository.retrieveLogDigest, repository.retrieveLogStatus)
	}
	repository.entries[0].RequiresLiveData = true
	// liveResult 是依赖实时业务数据的门禁结果。
	liveResult, liveErr := service.Retrieve(context.Background(), RetrieveRequest{UserID: 7, Query: "这个商品怎么使用？"})
	if liveErr != nil || liveResult.Answerability != AnswerabilityNeedsLiveData {
		t.Fatalf("live result=%+v err=%v", liveResult, liveErr)
	}
	// highRiskResult 是退款问题不受低风险知识绕过的转人工结果。
	highRiskResult, highRiskErr := service.Retrieve(context.Background(), RetrieveRequest{UserID: 7, Query: "我要退款"})
	if highRiskErr != nil || highRiskResult.Answerability != AnswerabilityNeedsHuman {
		t.Fatalf("high risk result=%+v err=%v", highRiskResult, highRiskErr)
	}
}

// TestAliasesNormalizeSynonymsAndImproveRetrieval 验证通用同义词和用户确认相似问法共享同一权威答案。
func TestAliasesNormalizeSynonymsAndImproveRetrieval(t *testing.T) {
	if normalizeSearchPhrase("怎么充值") != normalizeSearchPhrase("如何充值") || normalizeSearchPhrase("怎样充值") != "如何充值" {
		t.Fatal("通用中文同义表达应归一到同一检索文本")
	}
	// repository 保存充值 FAQ 和相似问法写入结果。
	repository := &knowledgeRepositoryFake{entry: Entry{ID: 6, KnowledgeBaseID: 2, Type: ContentTypeFAQ, Title: "购买后如何完成兑换？", Content: "请按步骤完成充值。"}, entries: []Entry{{ID: 6, KnowledgeBaseID: 2, KnowledgeBaseName: "使用教程", Type: ContentTypeFAQ, Title: "购买后如何完成兑换？", Content: "请按步骤完成充值。", RiskLevel: RiskLevelLow}}}
	// service 是待验证的相似问法应用服务。
	service := NewService(repository)
	// aliasID 是创建“怎么充值”相似问法返回的稳定主键。
	aliasID, createErr := service.CreateFAQAlias(context.Background(), 7, 2, 6, FAQAliasDraft{Alias: "怎么充值", Source: AliasSourceDebug})
	if createErr != nil || aliasID != 41 || repository.createdNormalizedAlias != "如何充值" {
		t.Fatalf("alias id=%d normalized=%q err=%v", aliasID, repository.createdNormalizedAlias, createErr)
	}
	repository.entries[0].Aliases = []string{"怎么充值"}
	// result 是通过相似问法完全命中的离线检索结果。
	result, retrieveErr := service.Retrieve(context.Background(), RetrieveRequest{UserID: 7, Query: "怎样充值"})
	if retrieveErr != nil || result.Answerability != AnswerabilityAnswerable || result.Candidate != "请按步骤完成充值。" || len(result.Evidence) != 1 || result.Evidence[0].Score < 0.9 {
		t.Fatalf("result=%+v err=%v", result, retrieveErr)
	}
	// suggestions 是不落库且去除同义重复后的确定性常见问法。
	suggestions, suggestErr := service.SuggestFAQAliases(context.Background(), 7, 2, 6)
	if suggestErr != nil || len(suggestions) == 0 {
		t.Fatalf("suggestions=%v err=%v", suggestions, suggestErr)
	}
}

// TestBuildChunksSplitsLongUnicodeDocuments 验证超长中文段落不截断 UTF-8 字符且保持稳定顺序。
func TestBuildChunksSplitsLongUnicodeDocuments(t *testing.T) {
	// content 是超过两个分块上限的中文文档正文。
	content := strings.Repeat("知", maxChunkRunes*2+7)
	// chunks 是对超长文档生成的确定性分块。
	chunks := BuildChunks(EntryDraft{Type: ContentTypeDocument, Title: "长文档", Content: content, ContentType: "markdown", RiskLevel: RiskLevelLow})
	if len(chunks) != 3 || len([]rune(chunks[0].Content)) != maxChunkRunes || len([]rune(chunks[2].Content)) != 7 {
		t.Fatalf("chunks=%d sizes=%d/%d", len(chunks), len([]rune(chunks[0].Content)), len([]rune(chunks[len(chunks)-1].Content)))
	}
	if chunks[0].Index != 0 || chunks[2].Index != 2 || chunks[0].ContentHash == chunks[2].ContentHash {
		t.Fatalf("chunk metadata=%+v", chunks)
	}
}

// 编译期确认测试夹具完整实现知识库持久化 Port。
var _ Repository = (*knowledgeRepositoryFake)(nil)
