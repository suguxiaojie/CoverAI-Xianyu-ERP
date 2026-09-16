package orders

import (
	"context"
	"errors"
	"testing"
	"time"
)

// listRepositoryStub 是订单列表服务测试使用的最小数据访问替身。
type listRepositoryStub struct {
	// owned 表示账号是否属于当前用户。
	owned bool
	// ownerErr 保存账号归属查询错误。
	ownerErr error
	// rows 保存待返回的订单行。
	rows []OrderRow
	// total 保存待返回的订单总数。
	total int
	// listErr 保存订单列表查询错误。
	listErr error
	// filter 保存最近一次收到的列表条件。
	filter ListFilter
	// settlement 保存待返回的已发货订单汇总。
	settlement SettlementAggregate
	// settlementErr 保存待结算汇总查询错误。
	settlementErr error
	// settlementFilter 保存最近一次收到的待结算筛选条件。
	settlementFilter ListFilter
}

// ExistsOwned 返回测试替身预设的账号归属结果。
func (s *listRepositoryStub) ExistsOwned(context.Context, int64, string) (bool, error) {
	return s.owned, s.ownerErr
}

// ListOrdersForUser 返回测试替身预设的订单列表结果。
func (s *listRepositoryStub) ListOrdersForUser(_ context.Context, filter ListFilter) ([]OrderRow, int, error) {
	s.filter = filter
	return s.rows, s.total, s.listErr
}

// SettlementSummaryForUser 返回测试替身预设的已发货订单汇总。
func (s *listRepositoryStub) SettlementSummaryForUser(_ context.Context, filter ListFilter) (SettlementAggregate, error) {
	s.settlementFilter = filter
	return s.settlement, s.settlementErr
}

// TestListServiceNormalizesPagination 验证订单列表服务规范化分页并传递筛选条件。
func TestListServiceNormalizesPagination(t *testing.T) {
	// repository 保存列表服务使用的数据访问替身。
	repository := &listRepositoryStub{owned: true, rows: []OrderRow{{OrderID: "order-1"}}, total: 401, settlement: SettlementAggregate{OrderCount: 2, GrossAmountCents: 62}}
	// service 保存待测试的订单列表服务。
	service := NewListService(repository)
	// createdFrom、createdTo 是待传递到 repository 的时间半开区间。
	createdFrom, createdTo := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	// result、err 保存列表查询结果及错误。
	// minAmountCents、maxAmountCents 是待传递到 repository 的实付金额包含边界。
	minAmountCents, maxAmountCents := int64(13500), int64(69000)
	// result、err 保存规范化分页与组合筛选结果。
	result, err := service.List(context.Background(), ListQuery{UserID: 7, CookieID: "cookie-1", Search: "buyer", CreatedFrom: createdFrom, CreatedTo: createdTo, MinAmountCents: &minAmountCents, MaxAmountCents: &maxAmountCents, Page: 0, PageSize: 999})
	if err != nil {
		t.Fatalf("List error=%v", err)
	}
	if result.Page != 1 || result.PageSize != 200 || result.TotalPages != 3 || len(result.Rows) != 1 {
		t.Fatalf("unexpected result=%+v", result)
	}
	if repository.filter.Offset != 0 || repository.filter.Limit != 200 || repository.filter.Search != "buyer" || !repository.filter.CreatedFrom.Equal(createdFrom) || !repository.filter.CreatedTo.Equal(createdTo) || repository.filter.MinAmountCents == nil || *repository.filter.MinAmountCents != 13500 || repository.filter.MaxAmountCents == nil || *repository.filter.MaxAmountCents != 69000 {
		t.Fatalf("unexpected filter=%+v", repository.filter)
	}
	if repository.settlementFilter.Status != "shipped" || repository.settlementFilter.Search != "buyer" || repository.settlementFilter.CookieID != "cookie-1" || repository.settlementFilter.MinAmountCents == nil || *repository.settlementFilter.MinAmountCents != 13500 {
		t.Fatalf("unexpected settlement filter=%+v", repository.settlementFilter)
	}
	if result.Settlement.OrderCount != 2 || result.Settlement.GrossAmountCents != 62 || result.Settlement.ServiceFeeCents != 1 || result.Settlement.PendingAmountCents != 61 {
		t.Fatalf("unexpected settlement=%+v", result.Settlement)
	}
}

// TestListServiceRejectsInvalidTimeRange 验证结束时间必须严格晚于开始时间。
func TestListServiceRejectsInvalidTimeRange(t *testing.T) {
	// service 使用不会触发真实数据库的列表 repository 替身。
	service := NewListService(&listRepositoryStub{owned: true})
	// boundary 是同时作为开始和结束的无效边界。
	boundary := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	// err 保存应用层时间范围校验结果。
	_, err := service.List(context.Background(), ListQuery{UserID: 7, CreatedFrom: boundary, CreatedTo: boundary})
	if !errors.Is(err, ErrInvalidListTimeRange) {
		t.Fatalf("error=%v, want ErrInvalidListTimeRange", err)
	}
}

// TestListServiceRejectsInvalidAmountRange 验证金额边界非负且最低金额不能高于最高金额。
func TestListServiceRejectsInvalidAmountRange(t *testing.T) {
	// service 使用不会触发真实数据库的列表 repository 替身。
	service := NewListService(&listRepositoryStub{owned: true})
	// negative、low、high 分别表示非法负值、合法低值和更小的上界。
	negative, low, high := int64(-1), int64(2000), int64(1000)
	// negativeErr 是负金额下界的应用校验结果。
	_, negativeErr := service.List(context.Background(), ListQuery{UserID: 7, MinAmountCents: &negative})
	if !errors.Is(negativeErr, ErrInvalidListAmountRange) {
		t.Fatalf("negative error=%v, want ErrInvalidListAmountRange", negativeErr)
	}
	// reversedErr 是下界高于上界时的应用校验结果。
	_, reversedErr := service.List(context.Background(), ListQuery{UserID: 7, MinAmountCents: &low, MaxAmountCents: &high})
	if !errors.Is(reversedErr, ErrInvalidListAmountRange) {
		t.Fatalf("reversed error=%v, want ErrInvalidListAmountRange", reversedErr)
	}
}

// TestListServiceRejectsUnownedCookie 验证订单列表服务阻止跨用户账号筛选。
func TestListServiceRejectsUnownedCookie(t *testing.T) {
	// service 保存不拥有目标账号的列表服务。
	service := NewListService(&listRepositoryStub{owned: false})
	// err 保存跨用户账号筛选返回的错误。
	_, err := service.List(context.Background(), ListQuery{UserID: 7, CookieID: "other"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("error=%v, want ErrForbidden", err)
	}
}

// TestListServicePropagatesRepositoryError 验证订单列表服务保留数据访问错误。
func TestListServicePropagatesRepositoryError(t *testing.T) {
	// expected 保存测试预设的数据访问错误。
	expected := errors.New("数据库不可用")
	// service 保存返回数据访问错误的列表服务。
	service := NewListService(&listRepositoryStub{owned: true, listErr: expected})
	// err 保存列表查询返回的错误。
	_, err := service.List(context.Background(), ListQuery{UserID: 7})
	if !errors.Is(err, expected) {
		t.Fatalf("error=%v, want repository error", err)
	}
}

// TestListServicePropagatesSettlementRepositoryError 验证待结算汇总失败时不返回缺失统计的订单列表。
func TestListServicePropagatesSettlementRepositoryError(t *testing.T) {
	// expected 是待结算汇总仓储返回的预设错误。
	expected := errors.New("待结算汇总不可用")
	// service 使用列表成功但汇总失败的数据访问替身。
	service := NewListService(&listRepositoryStub{owned: true, settlementErr: expected})
	// err 是订单列表用例的最终错误。
	_, err := service.List(context.Background(), ListQuery{UserID: 7})
	if !errors.Is(err, expected) {
		t.Fatalf("error=%v, want settlement repository error", err)
	}
}
