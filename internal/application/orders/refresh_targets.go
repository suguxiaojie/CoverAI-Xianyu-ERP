package orders

import (
	"context"
	"sort"
	"strings"
)

// collectRefreshTargets 扫描本地订单并按筛选条件收集详情目标；增量模式额外轮转补查有限数量的售后可变订单。
func (s *RefreshService) collectRefreshTargets(ctx context.Context, cookieIDs []string, status, mode string, blockedAccounts, newOrderIDs map[string]struct{}) map[string][]refreshTarget {
	// ordersByCookie 保存每个账号需要请求详情的订单。
	ordersByCookie := make(map[string][]refreshTarget)
	// currentCookieID 是当前读取详情目标的账号标识。
	for _, currentCookieID := range cookieIDs {
		if // blocked 表示账号是否因会话过期而跳过详情。
		_, blocked := blockedAccounts[currentCookieID]; blocked {
			continue
		}
		// afterCreatedAt、afterOrderID 保存当前账号订单扫描游标。
		afterCreatedAt, afterOrderID := "", ""
		// lifecycleRechecks 收集增量同步中需要轮转补查的已发货／已收货／已完成订单。
		lifecycleRechecks := make([]refreshTarget, 0)
		// refundCorrections 收集有精确退款申请证据的历史误分类订单，不与普通二十笔轮转竞争。
		refundCorrections := make([]refreshTarget, 0)
		// costedSKUCounts 缓存商品已配置成本的平台 SKU 数量，避免按订单重复查询。
		costedSKUCounts := make(map[string]int)
		for {
			// rows、rowErr 保存当前游标页订单及错误；沿用既有行为，读取失败时结束当前账号扫描。
			rows, rowErr := s.repository.ListOrdersByCookieCursor(ctx, currentCookieID, 500, afterCreatedAt, afterOrderID)
			if rowErr != nil {
				break
			}
			// row 是当前游标页的本地订单行。
			for _, row := range rows {
				// currentStatus 保存当前订单归一化状态。
				currentStatus := NormalizeOrderStatus(row.OrderStatus)
				if mode == RefreshModeCostBackfill {
					if !historicalCostEligibleStatus(currentStatus) || strings.TrimSpace(row.SpecValue) != "" || strings.TrimSpace(row.ItemID) == "" || strings.TrimSpace(row.Amount) == "" {
						continue
					}
					// costedSKUCount 是当前商品已配置成本的平台多规格数量。
					costedSKUCount, cached := costedSKUCounts[row.ItemID]
					if !cached {
						// countErr 是读取当前商品可匹配成本 SKU 数量的错误。
						var countErr error
						costedSKUCount, countErr = s.repository.CountCostedPlatformSKUs(ctx, currentCookieID, row.ItemID)
						if countErr != nil {
							continue
						}
						costedSKUCounts[row.ItemID] = costedSKUCount
					}
					if costedSKUCount > 1 {
						// covered、coveredErr 是当前订单是否已有可靠成本快照及查询错误。
						covered, coveredErr := s.repository.HasOrderCostSnapshot(ctx, row.OrderID)
						if coveredErr == nil && !covered {
							ordersByCookie[currentCookieID] = append(ordersByCookie[currentCookieID], refreshTarget{OrderID: row.OrderID, CurrentStatus: currentStatus, CreatedAt: row.CreatedAt, RefundRequested: row.RefundRequested, RequireSpec: true})
						}
					}
					continue
				}
				if status != "" && status != "all" && currentStatus != status {
					continue
				}
				if // isNewOrder 表示订单是否刚由发现阶段导入。
				_, isNewOrder := newOrderIDs[row.OrderID]; !isNewOrder && isStableRefreshStatus(currentStatus) && strings.TrimSpace(row.Amount) != "" {
					// refundCorrection 在增量和全量模式都补查旧版本误写的退款订单；普通售后轮转仍只属于增量模式。
					refundCorrection := currentStatus == "cancelled" && row.RefundRequested
					if refundCorrection {
						refundCorrections = append(refundCorrections, refreshTarget{OrderID: row.OrderID, CurrentStatus: currentStatus, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, RefundRequested: true})
						continue
					}
					// incrementalRecheck 表示当前订单进入每账号最多二十笔的售后状态轮转。
					incrementalRecheck := normalizeRefreshMode(mode) == RefreshModeIncremental && needsLifecycleRecheck(currentStatus)
					if incrementalRecheck {
						lifecycleRechecks = append(lifecycleRechecks, refreshTarget{OrderID: row.OrderID, CurrentStatus: currentStatus, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, RefundRequested: row.RefundRequested})
					}
					continue
				}
				ordersByCookie[currentCookieID] = append(ordersByCookie[currentCookieID], refreshTarget{OrderID: row.OrderID, CurrentStatus: currentStatus, CreatedAt: row.CreatedAt, RefundRequested: row.RefundRequested})
			}
			if len(rows) < 500 {
				break
			}
			// lastRow 保存当前游标页最后一条订单。
			lastRow := rows[len(rows)-1]
			if lastRow.CreatedAt == afterCreatedAt && lastRow.OrderID == afterOrderID {
				break
			}
			afterCreatedAt, afterOrderID = lastRow.CreatedAt, lastRow.OrderID
		}
		// 最久未更新的稳定订单优先小批量补查，详情写入会刷新 updated_at，使后续增量轮转到其他订单。
		sort.Slice(lifecycleRechecks, func(left, right int) bool {
			if lifecycleRechecks[left].UpdatedAt == lifecycleRechecks[right].UpdatedAt {
				return lifecycleRechecks[left].OrderID < lifecycleRechecks[right].OrderID
			}
			return lifecycleRechecks[left].UpdatedAt < lifecycleRechecks[right].UpdatedAt
		})
		if normalizeRefreshMode(mode) == RefreshModeIncremental && len(lifecycleRechecks) > incrementalLifecycleRecheckLimit {
			lifecycleRechecks = lifecycleRechecks[:incrementalLifecycleRecheckLimit]
		}
		ordersByCookie[currentCookieID] = append(ordersByCookie[currentCookieID], refundCorrections...)
		ordersByCookie[currentCookieID] = append(ordersByCookie[currentCookieID], lifecycleRechecks...)
	}
	return ordersByCookie
}

// historicalCostEligibleStatus 只允许仍参与 Dashboard 成交额的订单读取历史规格。
func historicalCostEligibleStatus(status string) bool {
	switch status {
	case "pending_ship", "shipped", "received", "completed":
		return true
	default:
		return false
	}
}
