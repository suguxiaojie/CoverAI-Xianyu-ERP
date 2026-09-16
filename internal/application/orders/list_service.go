package orders

import (
	"context"
	"errors"
	"time"
)

// ErrForbidden 表示当前用户无权访问订单筛选范围。
var ErrForbidden = errors.New("无权访问订单")

// ErrInvalidListTimeRange 表示订单列表的结束时间没有晚于开始时间。
var ErrInvalidListTimeRange = errors.New("订单时间范围无效")

// ErrInvalidListAmountRange 表示订单列表的金额边界为负数或下界高于上界。
var ErrInvalidListAmountRange = errors.New("订单金额范围无效")

// settlementServiceFeePermille 是平台服务费千分比；16 表示按汇总金额一次性扣除 1.6%。
const settlementServiceFeePermille int64 = 16

// ListQuery 是订单列表用例的分页和筛选条件。
type ListQuery struct {
	// UserID 是当前用户标识。
	UserID int64
	// CookieID 是可选的账号筛选条件。
	CookieID string
	// Status 是可选的订单状态筛选条件。
	Status string
	// Search 是订单号、商品或买家搜索词。
	Search string
	// CreatedFrom 是可选的订单创建时间下界，包含该时刻。
	CreatedFrom time.Time
	// CreatedTo 是可选的订单创建时间上界，不包含该时刻。
	CreatedTo time.Time
	// MinAmountCents 是可选的实付金额包含下界，单位为人民币分。
	MinAmountCents *int64
	// MaxAmountCents 是可选的实付金额包含上界，单位为人民币分。
	MaxAmountCents *int64
	// Page 是从 1 开始的页码。
	Page int
	// PageSize 是单页记录数上限。
	PageSize int
}

// ListResult 是订单列表用例的分页结果。
type ListResult struct {
	// Rows 是当前页订单展示行。
	Rows []OrderRow
	// Total 是符合筛选条件的订单总数。
	Total int
	// Page 是规范化后的页码。
	Page int
	// PageSize 是规范化后的单页记录数。
	PageSize int
	// TotalPages 是根据总数计算出的总页数。
	TotalPages int
	// Settlement 是当前账号、时间、金额和搜索范围内已发货订单的待结算统计。
	Settlement SettlementSummary
}

// SettlementAggregate 是持久化层返回的已发货订单数量和汇总前实付金额。
type SettlementAggregate struct {
	// OrderCount 是当前筛选范围内的已发货订单数。
	OrderCount int
	// GrossAmountCents 是所有已发货订单先累加后的实付总额，单位为人民币分。
	GrossAmountCents int64
}

// SettlementSummary 是扣除汇总金额 1.6% 服务费后的待结算结果。
type SettlementSummary struct {
	// OrderCount 是参与待结算计算的已发货订单数。
	OrderCount int
	// GrossAmountCents 是扣费前已发货订单总额，单位为人民币分。
	GrossAmountCents int64
	// ServiceFeeCents 是总额累加后一次性四舍五入计算的 1.6% 平台服务费。
	ServiceFeeCents int64
	// PendingAmountCents 是总额扣除平台服务费后的待结算金额。
	PendingAmountCents int64
}

// ListRepository 定义订单列表用例所需的最小数据访问能力。
type ListRepository interface {
	// ExistsOwned 判断账号是否归属于指定用户。
	ExistsOwned(ctx context.Context, userID int64, cookieID string) (bool, error)
	// ListOrdersForUser 查询用户范围内的订单展示行。
	ListOrdersForUser(ctx context.Context, filter ListFilter) ([]OrderRow, int, error)
	// SettlementSummaryForUser 汇总相同筛选范围内所有已发货订单的实付金额。
	SettlementSummaryForUser(ctx context.Context, filter ListFilter) (SettlementAggregate, error)
}

// ListService 承载订单列表分页和所有权规则，不依赖 HTTP 或数据库实现。
type ListService struct {
	// repository 保存订单列表用例所需的窄数据访问 Port。
	repository ListRepository
}

// NewListService 创建订单列表应用服务。
func NewListService(repository ListRepository) *ListService {
	return &ListService{repository: repository}
}

// settlementSummaryFromAggregate 在订单总额完成累加后按 1.6% 一次性四舍五入，并计算净待结算金额。
func settlementSummaryFromAggregate(aggregate SettlementAggregate) SettlementSummary {
	if aggregate.GrossAmountCents <= 0 || aggregate.OrderCount <= 0 {
		return SettlementSummary{}
	}
	// wholeThousands、remainderCents 分拆总额以避免直接乘十六造成整数溢出。
	wholeThousands, remainderCents := aggregate.GrossAmountCents/1000, aggregate.GrossAmountCents%1000
	// serviceFeeCents 是总额乘千分之十六后四舍五入到人民币分的服务费。
	serviceFeeCents := wholeThousands*settlementServiceFeePermille + (remainderCents*settlementServiceFeePermille+500)/1000
	return SettlementSummary{
		OrderCount: aggregate.OrderCount, GrossAmountCents: aggregate.GrossAmountCents,
		ServiceFeeCents: serviceFeeCents, PendingAmountCents: aggregate.GrossAmountCents - serviceFeeCents,
	}
}

// List 查询当前用户可见的订单，并集中处理分页和账号所有权规则。
func (s *ListService) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if s == nil || s.repository == nil {
		return ListResult{}, errors.New("订单列表 repository 未初始化")
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	if !query.CreatedFrom.IsZero() && !query.CreatedTo.IsZero() && !query.CreatedTo.After(query.CreatedFrom) {
		return ListResult{}, ErrInvalidListTimeRange
	}
	if query.MinAmountCents != nil && *query.MinAmountCents < 0 || query.MaxAmountCents != nil && *query.MaxAmountCents < 0 || query.MinAmountCents != nil && query.MaxAmountCents != nil && *query.MinAmountCents > *query.MaxAmountCents {
		return ListResult{}, ErrInvalidListAmountRange
	}
	if query.CookieID != "" {
		// owned、err 保存账号归属检查结果及错误。
		owned, err := s.repository.ExistsOwned(ctx, query.UserID, query.CookieID)
		if err != nil {
			return ListResult{}, err
		}
		if !owned {
			return ListResult{}, ErrForbidden
		}
	}
	// offset 是本次列表查询对应的数据库偏移量。
	offset := (query.Page - 1) * query.PageSize
	// rows、total、err 保存订单列表查询结果及错误。
	rows, total, err := s.repository.ListOrdersForUser(ctx, ListFilter{
		UserID: query.UserID, CookieID: query.CookieID, Status: query.Status,
		Search: query.Search, CreatedFrom: query.CreatedFrom, CreatedTo: query.CreatedTo,
		MinAmountCents: query.MinAmountCents, MaxAmountCents: query.MaxAmountCents,
		Limit: query.PageSize, Offset: offset,
	})
	if err != nil {
		return ListResult{}, err
	}
	// settlementAggregate、settlementErr 是忽略页面状态标签、固定统计已发货订单的当前筛选汇总。
	settlementAggregate, settlementErr := s.repository.SettlementSummaryForUser(ctx, ListFilter{
		UserID: query.UserID, CookieID: query.CookieID, Status: "shipped",
		Search: query.Search, CreatedFrom: query.CreatedFrom, CreatedTo: query.CreatedTo,
		MinAmountCents: query.MinAmountCents, MaxAmountCents: query.MaxAmountCents,
	})
	if settlementErr != nil {
		return ListResult{}, settlementErr
	}
	return ListResult{
		Rows: rows, Total: total, Page: query.Page, PageSize: query.PageSize,
		TotalPages: (total + query.PageSize - 1) / query.PageSize,
		Settlement: settlementSummaryFromAggregate(settlementAggregate),
	}, nil
}
