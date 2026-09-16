package adapter

import (
	"context"
	"encoding/json"
	"errors"

	itemapp "xianyu-go/internal/application/items"
	"xianyu-go/internal/db"
)

// ItemCatalogRepository 将商品读取 Port 适配到数据库商品仓储。
type ItemCatalogRepository struct {
	// store 提供商品查询能力。
	store *db.Store
}

// NewItemCatalogRepository 创建商品读取数据库适配器。
func NewItemCatalogRepository(store *db.Store) *ItemCatalogRepository {
	return &ItemCatalogRepository{store: store}
}

// ListForUser 查询用户范围商品并转换为应用模型。
func (repository *ItemCatalogRepository) ListForUser(ctx context.Context, userID int64, cookieID string) ([]itemapp.CatalogItem, error) {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return nil, errors.New("商品读取存储未初始化")
	}
	// rows 和 err 保存用户范围商品行及查询错误。
	rows, err := repository.store.Items.ListForUser(ctx, userID, cookieID)
	if err != nil {
		return nil, err
	}
	// skuRows、skuErr 保存同一用户范围内的 SKU 与本地成本。
	skuRows, skuErr := repository.store.Items.ListSKUsForUser(ctx, userID, cookieID)
	if skuErr != nil {
		return nil, skuErr
	}
	return catalogItemsFromRows(rows, skuRows), nil
}

// ListByCookie 查询账号商品并转换为应用模型。
func (repository *ItemCatalogRepository) ListByCookie(ctx context.Context, cookieID string) ([]itemapp.CatalogItem, error) {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return nil, errors.New("商品读取存储未初始化")
	}
	// rows 和 err 保存账号商品行及查询错误。
	rows, err := repository.store.Items.AllForCookie(ctx, cookieID)
	if err != nil {
		return nil, err
	}
	// skuRows、skuErr 保存当前账号下的 SKU 与本地成本。
	skuRows, skuErr := repository.store.Items.ListSKUsForCookie(ctx, cookieID)
	if skuErr != nil {
		return nil, skuErr
	}
	return catalogItemsFromRows(rows, skuRows), nil
}

// Get 查询单个商品并转换为应用模型。
func (repository *ItemCatalogRepository) Get(ctx context.Context, cookieID, itemID string) (itemapp.CatalogItem, error) {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return itemapp.CatalogItem{}, errors.New("商品读取存储未初始化")
	}
	// row 和 err 保存单个商品行及查询错误。
	row, err := repository.store.Items.Get(ctx, cookieID, itemID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return itemapp.CatalogItem{}, itemapp.ErrCatalogNotFound
		}
		return itemapp.CatalogItem{}, err
	}
	// skuRows、skuErr 保存当前商品 SKU 和本地成本。
	skuRows, skuErr := repository.store.Items.ListSKUs(ctx, cookieID, itemID)
	if skuErr != nil {
		return itemapp.CatalogItem{}, skuErr
	}
	// items 是包含当前 SKU 和本地成本的单商品应用模型切片。
	items := catalogItemsFromRows([]db.ItemInfoRow{{ID: row.ID, CookieID: row.CookieID, ItemID: row.ItemID, ItemTitle: row.ItemTitle, ItemDescription: row.ItemDescription, ItemCategory: row.ItemCategory, ItemPrice: row.ItemPrice, ItemDetail: row.ItemDetail, IsMultiSpec: row.IsMultiSpec, MultiQuantityDelivery: row.MultiQuantityDelivery}}, skuRows)
	return items[0], nil
}

// Upsert 创建或完整保存本地商品，并将应用输入转换为数据库行模型。
func (repository *ItemCatalogRepository) Upsert(ctx context.Context, cookieID string, input itemapp.CatalogWriteInput) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	return repository.store.Items.Upsert(ctx, &db.ItemInfoRow{
		CookieID: cookieID, ItemID: input.ItemID, ItemTitle: input.ItemTitle, ItemDescription: input.ItemDescription,
		ItemCategory: input.ItemCategory, ItemPrice: input.ItemPrice, ItemDetail: input.ItemDetail,
		IsMultiSpec: input.IsMultiSpec, MultiQuantityDelivery: input.MultiQuantityDelivery,
	})
}

// UpsertPublishedItem 保存批量发布成功后的商品目录记录。
func (repository *ItemCatalogRepository) UpsertPublishedItem(ctx context.Context, input itemapp.BatchPublishedItem) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	return repository.store.Items.Upsert(ctx, &db.ItemInfoRow{
		CookieID: input.CookieID, ItemID: input.ItemID, ItemTitle: input.ItemTitle, ItemDescription: input.ItemDescription,
		ItemCategory: input.ItemCategory, ItemPrice: input.ItemPrice, ItemDetail: input.ItemDetail,
		MultiQuantityDelivery: input.MultiQuantityDelivery,
	})
}

// Delete 逻辑删除本地商品及其商品级自动化规则。
func (repository *ItemCatalogRepository) Delete(ctx context.Context, cookieID, itemID string) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	return repository.store.Items.Delete(ctx, cookieID, itemID)
}

// SetMultiSpec 更新本地商品的多规格交付开关。
func (repository *ItemCatalogRepository) SetMultiSpec(ctx context.Context, cookieID, itemID string, enabled bool) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	return repository.store.Items.SetMultiSpec(ctx, cookieID, itemID, enabled)
}

// SetMultiQuantity 更新本地商品的多数量交付开关。
func (repository *ItemCatalogRepository) SetMultiQuantity(ctx context.Context, cookieID, itemID string, enabled bool) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	return repository.store.Items.SetMultiQuantity(ctx, cookieID, itemID, enabled)
}

// UpdateSKUCost 更新本地 SKU 单件成本，不调用闲鱼平台。
func (repository *ItemCatalogRepository) UpdateSKUCost(ctx context.Context, cookieID, itemID, skuID string, costCents *int64) error {
	return repository.UpdateSKUCosts(ctx, cookieID, itemID, []itemapp.SKUCostInput{{SKUID: skuID, CostCents: costCents}})
}

// UpdateSKUCosts 原子保存一个商品的全部本地成本，不调用闲鱼平台。
func (repository *ItemCatalogRepository) UpdateSKUCosts(ctx context.Context, cookieID, itemID string, costs []itemapp.SKUCostInput) error {
	if repository == nil || repository.store == nil || repository.store.Items == nil {
		return errors.New("商品写入存储未初始化")
	}
	// updates 是应用输入转换后的数据库成本更新集合。
	updates := make([]db.ItemSKUCostUpdate, 0, len(costs))
	// cost 表示当前待转换的应用成本输入。
	for _, cost := range costs {
		updates = append(updates, db.ItemSKUCostUpdate{SKUID: cost.SKUID, CostCents: cost.CostCents})
	}
	// updateErr 是数据库精确更新 SKU 成本的结果。
	updateErr := repository.store.Items.UpdateSKUCosts(ctx, cookieID, itemID, updates)
	if errors.Is(updateErr, db.ErrNotFound) {
		return itemapp.ErrCatalogNotFound
	}
	return updateErr
}

// catalogItemsFromRows 转换数据库商品行并保持查询顺序。
func catalogItemsFromRows(rows []db.ItemInfoRow, skuRows []db.ItemSKURow) []itemapp.CatalogItem {
	// skusByItem 按账号和商品组合键收集 SKU，避免跨账号相同商品 ID 串数据。
	skusByItem := make(map[string][]itemapp.CatalogSKU)
	// skuRow 表示当前待转换的数据库 SKU 行。
	for _, skuRow := range skuRows {
		// properties 保存从 JSON 解码后的规格名称和值；历史异常 JSON 按空规格处理。
		properties := make([]itemapp.CatalogSKUProperty, 0)
		_ = json.Unmarshal([]byte(skuRow.PropertiesJSON), &properties)
		// key 是账号和商品组成的 SKU 归属键。
		key := skuRow.CookieID + "\x00" + skuRow.ItemID
		skusByItem[key] = append(skusByItem[key], itemapp.CatalogSKU{
			SKUID: skuRow.SKUID, InventoryID: skuRow.InventoryID, Properties: properties,
			PriceCents: skuRow.PriceCents, Quantity: skuRow.Quantity, InitialQuantity: skuRow.InitialQuantity,
			Enabled: skuRow.Enabled, SortOrder: skuRow.SortOrder, CostCents: skuRow.CostCents, SyncedAt: skuRow.SyncedAt,
			LocalOnly: skuRow.SKUID == db.DefaultItemSKUID,
		})
	}
	// items 保存转换后的应用商品模型并保持数据库顺序。
	items := make([]itemapp.CatalogItem, 0, len(rows))
	// row 表示当前待转换的数据库商品行。
	for _, row := range rows {
		// key 是当前商品匹配 SKU 的账号和商品组合键。
		key := row.CookieID + "\x00" + row.ItemID
		items = append(items, itemapp.CatalogItem{ID: row.ID, CookieID: row.CookieID, ItemID: row.ItemID, ItemTitle: row.ItemTitle, ItemDescription: row.ItemDescription, ItemCategory: row.ItemCategory, ItemPrice: row.ItemPrice, ItemDetail: row.ItemDetail, IsMultiSpec: row.IsMultiSpec, MultiQuantityDelivery: row.MultiQuantityDelivery, SKUs: skusByItem[key]})
	}
	return items
}

var _ itemapp.CatalogRepository = (*ItemCatalogRepository)(nil)
var _ itemapp.CatalogMutationRepository = (*ItemCatalogRepository)(nil)
