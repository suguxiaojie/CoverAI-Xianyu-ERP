package keywords

import (
	"context"
	"errors"
	"testing"
)

// keywordRepositoryFake 保存关键词服务测试所需的可控端口行为。
type keywordRepositoryFake struct {
	// addErr 是创建操作返回的错误。
	addErr error
	// addedDraft 保存最近一次创建输入。
	addedDraft Draft
	// listRows 是列表操作返回的规则。
	listRows []Keyword
	// listErr 是列表操作返回的错误。
	listErr error
	// replaceErr 是批量替换返回的错误。
	replaceErr error
	// updateErr 是更新返回的错误。
	updateErr error
	// deleteErr 是删除返回的错误。
	deleteErr error
	// itemRows 是商品回复列表。
	itemRows []ItemReply
	// itemErr 是商品回复操作返回的错误。
	itemErr error
	// toggledGroupID、toggledEnabled 保存最近一次单规则启停输入。
	toggledGroupID string
	toggledEnabled bool
	// toggledAll 表示最近一次调用是否为全部规则启停。
	toggledAll bool
}

// List 实现测试仓储的关键词列表端口。
func (f *keywordRepositoryFake) List(context.Context, int64, string) ([]Keyword, error) {
	return f.listRows, f.listErr
}

// Add 实现测试仓储的关键词创建端口。
func (f *keywordRepositoryFake) Add(_ context.Context, _ int64, _ string, draft Draft) (int64, error) {
	f.addedDraft = draft
	return 9, f.addErr
}

// Replace 实现测试仓储的关键词批量替换端口。
func (f *keywordRepositoryFake) Replace(context.Context, int64, string, []Draft) error {
	return f.replaceErr
}

// Update 实现测试仓储的关键词更新端口。
func (f *keywordRepositoryFake) Update(context.Context, int64, string, int64, Draft) error {
	return f.updateErr
}

// DeleteByID 实现测试仓储的关键词 ID 删除端口。
func (f *keywordRepositoryFake) DeleteByID(context.Context, int64, string, int64) error {
	return f.deleteErr
}

// DeleteByIndex 实现测试仓储的关键词索引删除端口。
func (f *keywordRepositoryFake) DeleteByIndex(context.Context, int64, string, int) error {
	return f.deleteErr
}

// ListGroups 返回空关键词规则组集合。
func (f *keywordRepositoryFake) ListGroups(context.Context, int64, string) ([]Group, error) {
	return nil, f.listErr
}

// SaveGroup 返回稳定测试规则组标识。
func (f *keywordRepositoryFake) SaveGroup(context.Context, int64, string, GroupDraft) (string, error) {
	return "group-1", f.addErr
}

// DeleteGroup 接受测试规则组删除。
func (f *keywordRepositoryFake) DeleteGroup(context.Context, int64, string, string) error {
	return f.deleteErr
}

// ListGlobalGroups 返回空全局关键词规则组集合。
func (f *keywordRepositoryFake) ListGlobalGroups(context.Context, int64) ([]Group, error) {
	return nil, f.listErr
}

// SaveGlobalGroup 返回稳定全局测试规则组标识。
func (f *keywordRepositoryFake) SaveGlobalGroup(context.Context, int64, GroupDraft) (string, error) {
	return "global-group-1", f.addErr
}

// DeleteGlobalGroup 接受全局测试规则组删除。
func (f *keywordRepositoryFake) DeleteGlobalGroup(context.Context, int64, string) error {
	return f.deleteErr
}

// SetGlobalGroupEnabled 接受测试中的单规则启停操作。
func (f *keywordRepositoryFake) SetGlobalGroupEnabled(_ context.Context, _ int64, groupID string, enabled bool) error {
	f.toggledGroupID, f.toggledEnabled = groupID, enabled
	return f.updateErr
}

// SetAllGlobalGroupsEnabled 接受测试中的全部规则启停操作。
func (f *keywordRepositoryFake) SetAllGlobalGroupsEnabled(_ context.Context, _ int64, enabled bool) error {
	f.toggledAll, f.toggledEnabled = true, enabled
	return f.updateErr
}

// SetGlobalGroupAccountEnabled 接受测试中的单店铺规则启停操作。
func (f *keywordRepositoryFake) SetGlobalGroupAccountEnabled(_ context.Context, _ int64, groupID, _ string, enabled bool) error {
	f.toggledGroupID, f.toggledEnabled = groupID, enabled
	return f.updateErr
}

// TestServiceTogglesGlobalKeywordRules 验证单规则和全部规则启停经过身份及组标识校验后进入仓储。
func TestServiceTogglesGlobalKeywordRules(t *testing.T) {
	// repository 是记录启停调用的测试仓储。
	repository := &keywordRepositoryFake{}
	// service 是本测试使用的关键词应用服务。
	service := NewService(repository)
	if // toggleErr 是单规则关闭操作错误。
	toggleErr := service.SetGlobalGroupEnabled(context.Background(), 7, " group-1 ", false); toggleErr != nil {
		t.Fatal(toggleErr)
	}
	if repository.toggledGroupID != "group-1" || repository.toggledEnabled {
		t.Fatalf("single toggle group=%q enabled=%v", repository.toggledGroupID, repository.toggledEnabled)
	}
	if // toggleAllErr 是全部规则开启操作错误。
	toggleAllErr := service.SetAllGlobalGroupsEnabled(context.Background(), 7, true); toggleAllErr != nil {
		t.Fatal(toggleAllErr)
	}
	if !repository.toggledAll || !repository.toggledEnabled {
		t.Fatalf("bulk toggle called=%v enabled=%v", repository.toggledAll, repository.toggledEnabled)
	}
	if // invalidErr 是空规则组标识的预期输入错误。
	invalidErr := service.SetGlobalGroupEnabled(context.Background(), 7, " ", true); invalidErr == nil {
		t.Fatal("空规则组标识应被拒绝")
	}
}

// ListItemReplies 实现测试仓储的商品回复列表端口。
func (f *keywordRepositoryFake) ListItemReplies(context.Context, int64) ([]ItemReply, error) {
	return f.itemRows, f.itemErr
}

// GetItemReply 实现测试仓储的商品回复读取端口。
func (f *keywordRepositoryFake) GetItemReply(context.Context, int64, string, string) (ItemReply, error) {
	if f.itemErr != nil {
		return ItemReply{}, f.itemErr
	}
	if len(f.itemRows) == 0 {
		return ItemReply{}, ErrNotFound
	}
	return f.itemRows[0], nil
}

// SetItemReply 实现测试仓储的商品回复写入端口。
func (f *keywordRepositoryFake) SetItemReply(context.Context, int64, string, string, string) error {
	return f.itemErr
}

// DeleteItemReply 实现测试仓储的商品回复删除端口。
func (f *keywordRepositoryFake) DeleteItemReply(context.Context, int64, string, string) error {
	return f.itemErr
}

// TestServiceNormalizesAndCreatesKeyword 验证成功创建会规范化输入并传给仓储。
func TestServiceNormalizesAndCreatesKeyword(t *testing.T) {
	// repository 是本测试使用的可控关键词仓储。
	repository := &keywordRepositoryFake{}
	// service 是待验证的关键词应用服务。
	service := NewService(repository)
	// id、err 保存创建结果。
	id, err := service.Add(context.Background(), 7, "account-1", Draft{Keyword: "  价格 ", Reply: "  50元 ", Type: "TEXT"})
	if err != nil || id != 9 {
		t.Fatalf("创建失败 id=%d err=%v", id, err)
	}
	if repository.addedDraft.Keyword != "价格" || repository.addedDraft.Reply != "50元" || repository.addedDraft.Type != "text" {
		t.Fatalf("输入未规范化: %+v", repository.addedDraft)
	}
}

// TestSystemOnlyGroupAllowsNoKeywordsButCustomerRequiresKeywords 验证具体系统事件可独立触发，而客户来源仍必须配置关键词。
func TestSystemOnlyGroupAllowsNoKeywordsButCustomerRequiresKeywords(t *testing.T) {
	// service 是使用内存仓储的关键词应用服务。
	service := NewService(&keywordRepositoryFake{})
	// groupID、systemErr 是仅系统事件、无关键词的全局规则保存结果。
	groupID, systemErr := service.SaveGlobalGroup(context.Background(), 7, GroupDraft{AccountIDs: []string{"account-1"}, MessageScopes: []string{"system"}, SystemTypes: []string{"order_shipped"}, Type: "text", Reply: "订单已发货"})
	if systemErr != nil || groupID != "global-group-1" {
		t.Fatalf("group=%q err=%v", groupID, systemErr)
	}
	// _, customerErr 是客户消息没有关键词时的预期校验错误。
	_, customerErr := service.SaveGlobalGroup(context.Background(), 7, GroupDraft{AccountIDs: []string{"account-1"}, MessageScopes: []string{"customer"}, Type: "text", Reply: "回复"})
	if customerErr == nil {
		t.Fatal("客户消息规则缺少关键词应拒绝")
	}
}

// TestServiceRejectsInvalidInput 验证用户、账号、关键词和类型参数在仓储调用前被拒绝。
func TestServiceRejectsInvalidInput(t *testing.T) {
	// cases 是覆盖关键参数边界的服务调用集合。
	cases := []struct {
		// name 是子测试名称。
		name string
		// call 是待验证的服务调用。
		call func(*Service) error
	}{
		{name: "invalid user", call: func(service *Service) error {
			// err 表示无效用户调用返回的参数错误。
			_, err := service.Add(context.Background(), 0, "account", Draft{Keyword: "k", Reply: "r"})
			return err
		}},
		{name: "invalid account", call: func(service *Service) error {
			// err 表示空账号标识调用返回的参数错误。
			_, err := service.Add(context.Background(), 1, "", Draft{Keyword: "k", Reply: "r"})
			return err
		}},
		{name: "missing keyword", call: func(service *Service) error {
			// err 表示缺少关键词调用返回的校验错误。
			_, err := service.Add(context.Background(), 1, "account", Draft{Reply: "r"})
			return err
		}},
		{name: "missing image", call: func(service *Service) error {
			// err 表示图片规则缺少图片地址的校验错误。
			_, err := service.Add(context.Background(), 1, "account", Draft{Keyword: "k", Type: "image"})
			return err
		}},
		{name: "unsupported type", call: func(service *Service) error {
			// err 表示不支持的回复类型校验错误。
			_, err := service.Add(context.Background(), 1, "account", Draft{Keyword: "k", Type: "api", Reply: "r"})
			return err
		}},
	}
	// testCase 表示当前待验证的参数边界。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// err 是当前参数调用返回的错误。
			err := testCase.call(NewService(&keywordRepositoryFake{}))
			if err == nil {
				t.Fatal("无效参数应返回错误")
			}
		})
	}
}

// TestServicePreservesOwnershipAndInfrastructureErrors 验证仓储返回的跨用户和基础设施错误不被吞掉。
func TestServicePreservesOwnershipAndInfrastructureErrors(t *testing.T) {
	// backendErr 是模拟数据库故障的哨兵错误。
	backendErr := errors.New("database unavailable")
	// cases 是不同底层错误阶段的服务调用集合。
	cases := []struct {
		// name 是子测试名称。
		name string
		// repository 是返回当前错误的仓储。
		repository *keywordRepositoryFake
		// want 是期望的错误。
		want error
	}{
		{name: "forbidden", repository: &keywordRepositoryFake{listErr: ErrForbidden}, want: ErrForbidden},
		{name: "infrastructure", repository: &keywordRepositoryFake{listErr: backendErr}, want: backendErr},
	}
	// testCase 表示当前待验证的错误边界。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// err 是列表服务返回的错误。
			_, err := NewService(testCase.repository).List(context.Background(), 1, "account")
			if !errors.Is(err, testCase.want) {
				t.Fatalf("err=%v want=%v", err, testCase.want)
			}
		})
	}
}

// TestServiceItemReplyValidationAndPropagation 验证指定商品回复参数校验及底层错误传播。
func TestServiceItemReplyValidationAndPropagation(t *testing.T) {
	// backendErr 是模拟商品回复数据库故障的哨兵错误。
	backendErr := errors.New("item reply unavailable")
	// repository 是返回底层故障的测试仓储。
	repository := &keywordRepositoryFake{itemErr: backendErr}
	// service 是待验证的关键词应用服务。
	service := NewService(repository)
	// err 表示空商品标识返回的校验错误。
	if err := service.SetItemReply(context.Background(), 1, "account", "", "reply"); err == nil {
		t.Fatal("空商品 ID 应被拒绝")
	}
	// err 表示商品回复持久化阶段返回的基础设施错误。
	if err := service.SetItemReply(context.Background(), 1, "account", "item", "reply"); !errors.Is(err, backendErr) {
		t.Fatalf("基础设施错误未透传: %v", err)
	}
}

var _ Repository = (*keywordRepositoryFake)(nil)
