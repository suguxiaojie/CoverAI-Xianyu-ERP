package adapter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	keywordsapp "xianyu-go/internal/application/keywords"
	"xianyu-go/internal/db"
)

// KeywordRepository 将关键词和指定商品回复数据库操作适配为应用层 Port。
type KeywordRepository struct {
	// store 保存数据库聚合入口；敏感账号字段不会被本适配器读取。
	store *db.Store
}

// NewKeywordRepository 创建关键词数据库适配器。
func NewKeywordRepository(store *db.Store) *KeywordRepository {
	return &KeywordRepository{store: store}
}

// List 查询指定用户账号的关键词规则，并转换为应用模型。
func (r *KeywordRepository) List(ctx context.Context, userID int64, cookieID string) ([]keywordsapp.Keyword, error) {
	// err 表示账号归属校验失败，阻止跨用户读取规则。
	if // authErr 是规则组列表账号归属校验错误。
	err := r.authorize(ctx, userID, cookieID); err != nil {
		return nil, err
	}
	// rows 保存数据库返回的关键词规则行。
	rows, err := r.store.Keywords.AllRows(ctx, cookieID)
	if err != nil {
		return nil, err
	}
	// result 保存转换后的应用层关键词模型，不携带数据库对象。
	result := make([]keywordsapp.Keyword, 0, len(rows))
	// row 表示当前待转换的关键词数据库行。
	for _, row := range rows {
		result = append(result, keywordModel(row))
	}
	return result, nil
}

// Add 创建一条指定用户账号的关键词规则。
func (r *KeywordRepository) Add(ctx context.Context, userID int64, cookieID string, draft keywordsapp.Draft) (int64, error) {
	// err 表示账号归属校验失败。
	if // authErr 是规则组保存账号归属校验错误。
	err := r.authorize(ctx, userID, cookieID); err != nil {
		return 0, err
	}
	return r.store.Keywords.Add(ctx, cookieID, draft.Keyword, draft.Reply, draft.ItemID, draft.Type, draft.ImageURL)
}

// Replace 原子覆盖指定用户账号的全部关键词规则。
func (r *KeywordRepository) Replace(ctx context.Context, userID int64, cookieID string, drafts []keywordsapp.Draft) error {
	// err 表示账号归属校验失败。
	if // authErr 是规则组删除账号归属校验错误。
	err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	// rows 保存转换后的数据库关键词写入行。
	rows := make([]db.KeywordRow, 0, len(drafts))
	// draft 表示当前待转换的应用层关键词草稿。
	for _, draft := range drafts {
		rows = append(rows, db.KeywordRow{
			CookieID: cookieID,
			Keyword:  draft.Keyword,
			Reply:    draft.Reply,
			ItemID:   draft.ItemID,
			Type:     draft.Type,
			ImageURL: draft.ImageURL,
		})
	}
	return r.store.Keywords.ReplaceForCookie(ctx, cookieID, rows)
}

// Update 更新指定用户账号的一条关键词规则。
func (r *KeywordRepository) Update(ctx context.Context, userID int64, cookieID string, id int64, draft keywordsapp.Draft) error {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	// err 表示数据库更新错误或目标规则不存在。
	err := r.store.Keywords.UpdateByID(ctx, db.KeywordRow{
		ID: id, CookieID: cookieID, Keyword: draft.Keyword, Reply: draft.Reply,
		ItemID: draft.ItemID, Type: draft.Type, ImageURL: draft.ImageURL,
	})
	if errors.Is(err, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return err
}

// DeleteByID 按 ID 删除指定用户账号的一条关键词规则。
func (r *KeywordRepository) DeleteByID(ctx context.Context, userID int64, cookieID string, id int64) error {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	// err 表示数据库删除错误或目标规则不存在。
	err := r.store.Keywords.DeleteByID(ctx, cookieID, id)
	if errors.Is(err, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return err
}

// DeleteByIndex 按稳定 ID 顺序的零基索引删除关键词规则。
func (r *KeywordRepository) DeleteByIndex(ctx context.Context, userID int64, cookieID string, index int) error {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	// err 表示数据库删除错误或索引没有对应规则。
	err := r.store.Keywords.DeleteByIndex(ctx, cookieID, index)
	if errors.Is(err, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return err
}

// ListGroups 按 group_id 聚合同一回复配置下的多个关键词。
func (r *KeywordRepository) ListGroups(ctx context.Context, userID int64, cookieID string) ([]keywordsapp.Group, error) {
	if // authErr 是规则组列表账号归属校验错误。
	authErr := r.authorize(ctx, userID, cookieID); authErr != nil {
		return nil, authErr
	}
	// rows、listErr 是账号全部关键词持久化行和读取错误。
	rows, listErr := r.store.Keywords.AllRows(ctx, cookieID)
	if listErr != nil {
		return nil, listErr
	}
	// groups 保存按首次出现顺序输出的规则组。
	groups := make([]keywordsapp.Group, 0)
	// indexes 保存规则组标识到结果下标的映射。
	indexes := make(map[string]int)
	// row 是当前归入规则组的关键词行。
	for _, row := range rows {
		// groupID 是迁移后稳定规则组标识；异常空值按单行 ID 回退。
		groupID := row.GroupID
		if groupID == "" {
			groupID = fmt.Sprintf("legacy-%d", row.ID)
		}
		// index、exists 是当前组结果下标和存在标记。
		index, exists := indexes[groupID]
		if !exists {
			index = len(groups)
			indexes[groupID] = index
			// systemTypes 是逗号分隔持久化值转换后的系统消息白名单。
			systemTypes := make([]string, 0)
			if strings.TrimSpace(row.SystemTypes) != "" {
				systemTypes = strings.Split(row.SystemTypes, ",")
			}
			groups = append(groups, keywordsapp.Group{GroupID: groupID, Reply: row.Reply, ItemID: row.ItemID, Type: row.Type, ImageURL: row.ImageURL, MatchType: row.MatchType, MessageScope: row.MessageScope, MessageScopes: strings.Split(row.MessageScope, ","), SystemTypes: systemTypes, Enabled: row.Enabled, ReplyIntervalSeconds: row.ReplyIntervalSeconds, SendDelaySeconds: row.SendDelaySeconds})
		}
		if strings.TrimSpace(row.Keyword) != "" {
			groups[index].Keywords = append(groups[index].Keywords, row.Keyword)
		}
	}
	return groups, nil
}

// SaveGroup 创建或替换当前用户账号下的多关键词规则组。
func (r *KeywordRepository) SaveGroup(ctx context.Context, userID int64, cookieID string, draft keywordsapp.GroupDraft) (string, error) {
	if // authErr 是规则组保存账号归属校验错误。
	authErr := r.authorize(ctx, userID, cookieID); authErr != nil {
		return "", authErr
	}
	return r.store.Keywords.SaveGroup(ctx, cookieID, draft.GroupID, draft.Keywords, draft.Reply, draft.ItemID, draft.Type, draft.ImageURL, draft.MatchType, draft.MessageScope, strings.Join(draft.SystemTypes, ","), draft.ReplyIntervalSeconds, draft.SendDelaySeconds)
}

// DeleteGroup 删除当前用户账号下一个完整规则组。
func (r *KeywordRepository) DeleteGroup(ctx context.Context, userID int64, cookieID, groupID string) error {
	if // authErr 是规则组删除账号归属校验错误。
	authErr := r.authorize(ctx, userID, cookieID); authErr != nil {
		return authErr
	}
	// deleteErr 是数据库删除结果。
	deleteErr := r.store.Keywords.DeleteGroup(ctx, cookieID, groupID)
	if errors.Is(deleteErr, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return deleteErr
}

// ListGlobalGroups 跨当前用户全部店铺聚合同一 group_id 的全局关键词规则。
func (r *KeywordRepository) ListGlobalGroups(ctx context.Context, userID int64) ([]keywordsapp.Group, error) {
	if userID <= 0 || r == nil || r.store == nil {
		return nil, keywordsapp.ErrInvalidUser
	}
	// cookieIDs、accountErr 是当前用户全部店铺账号和读取错误。
	cookieIDs, accountErr := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if accountErr != nil {
		return nil, accountErr
	}
	// result、indexes 保存跨店铺聚合结果和组标识索引。
	result, indexes := make([]keywordsapp.Group, 0), make(map[string]int)
	// cookieID 是当前读取规则组的店铺账号。
	for _, cookieID := range cookieIDs {
		// groups、listErr 是当前店铺规则组和读取错误。
		groups, listErr := r.ListGroups(ctx, userID, cookieID)
		if listErr != nil {
			return nil, listErr
		}
		// group 是当前合并到全局视图的店铺规则组。
		for _, group := range groups {
			// index、exists 是全局组下标和存在标记。
			index, exists := indexes[group.GroupID]
			if !exists {
				index = len(result)
				indexes[group.GroupID] = index
				group.AccountIDs = nil
				result = append(result, group)
			} else {
				result[index].Enabled = result[index].Enabled && group.Enabled
			}
			result[index].AccountIDs = append(result[index].AccountIDs, cookieID)
			result[index].AccountStates = append(result[index].AccountStates, keywordsapp.AccountState{AccountID: cookieID, Enabled: group.Enabled})
		}
	}
	return result, nil
}

// SaveGlobalGroup 校验全部店铺归属后原子替换规则组绑定。
func (r *KeywordRepository) SaveGlobalGroup(ctx context.Context, userID int64, draft keywordsapp.GroupDraft) (string, error) {
	// ownedCookieIDs、accountErr 是当前用户全部店铺账号和读取错误。
	ownedCookieIDs, accountErr := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if accountErr != nil {
		return "", accountErr
	}
	// owned 保存允许绑定的店铺账号集合。
	owned := make(map[string]struct{}, len(ownedCookieIDs))
	// cookieID 是当前加入归属集合的账号。
	for _, cookieID := range ownedCookieIDs {
		owned[cookieID] = struct{}{}
	}
	// selected 保存去重后的目标店铺账号。
	selected := make([]string, 0, len(draft.AccountIDs))
	// seen 保存已经加入目标集合的账号。
	seen := make(map[string]struct{}, len(draft.AccountIDs))
	// cookieID 是当前校验的用户选择店铺。
	for _, cookieID := range draft.AccountIDs {
		// _, exists 只读取当前店铺是否属于用户。
		if _, exists := owned[cookieID]; !exists {
			return "", keywordsapp.ErrForbidden
		}
		// _, exists 只读取当前店铺是否已经加入目标集合。
		if _, exists := seen[cookieID]; !exists {
			seen[cookieID] = struct{}{}
			selected = append(selected, cookieID)
		}
	}
	// enabled 是应用服务已经补齐默认值后的规则启用状态。
	enabled := true
	if draft.Enabled != nil {
		enabled = *draft.Enabled
	}
	return r.store.Keywords.SaveGlobalGroup(ctx, ownedCookieIDs, selected, draft.GroupID, draft.Keywords, draft.Reply, draft.ItemID, draft.Type, draft.ImageURL, draft.MatchType, draft.MessageScope, strings.Join(draft.SystemTypes, ","), enabled, draft.ReplyIntervalSeconds, draft.SendDelaySeconds)
}

// DeleteGlobalGroup 从当前用户全部店铺删除一个规则组。
func (r *KeywordRepository) DeleteGlobalGroup(ctx context.Context, userID int64, groupID string) error {
	// cookieIDs、accountErr 是当前用户全部店铺账号和读取错误。
	cookieIDs, accountErr := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if accountErr != nil {
		return accountErr
	}
	// deleteErr 是数据库全局组删除结果。
	deleteErr := r.store.Keywords.DeleteGlobalGroup(ctx, cookieIDs, groupID)
	if errors.Is(deleteErr, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return deleteErr
}

// SetGlobalGroupEnabled 在当前用户全部店铺绑定中切换一个全局规则组。
func (r *KeywordRepository) SetGlobalGroupEnabled(ctx context.Context, userID int64, groupID string, enabled bool) error {
	// cookieIDs、accountErr 是当前用户全部店铺账号和读取错误。
	cookieIDs, accountErr := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if accountErr != nil {
		return accountErr
	}
	// updateErr 是数据库规则组启停结果。
	updateErr := r.store.Keywords.SetGlobalGroupEnabled(ctx, cookieIDs, groupID, enabled)
	if errors.Is(updateErr, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return updateErr
}

// SetAllGlobalGroupsEnabled 切换当前用户全部店铺中的所有关键词规则组。
func (r *KeywordRepository) SetAllGlobalGroupsEnabled(ctx context.Context, userID int64, enabled bool) error {
	// cookieIDs、accountErr 是当前用户全部店铺账号和读取错误。
	cookieIDs, accountErr := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if accountErr != nil {
		return accountErr
	}
	return r.store.Keywords.SetAllGlobalGroupsEnabled(ctx, cookieIDs, enabled)
}

// SetGlobalGroupAccountEnabled 校验店铺归属后切换该店铺中的规则状态。
func (r *KeywordRepository) SetGlobalGroupAccountEnabled(ctx context.Context, userID int64, groupID, accountID string, enabled bool) error {
	if // authErr 是单店铺规则启停的账号归属校验错误。
	authErr := r.authorize(ctx, userID, accountID); authErr != nil {
		return authErr
	}
	// updateErr 是数据库单店铺启停结果。
	updateErr := r.store.Keywords.SetGlobalGroupAccountEnabled(ctx, accountID, groupID, enabled)
	if errors.Is(updateErr, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	return updateErr
}

// ListItemReplies 查询当前用户所有账号的商品回复，并保持数据库返回顺序。
func (r *KeywordRepository) ListItemReplies(ctx context.Context, userID int64) ([]keywordsapp.ItemReply, error) {
	// err 表示适配器依赖或用户身份校验失败。
	if err := r.validateUser(userID); err != nil {
		return nil, err
	}
	// cookieIDs 保存当前用户拥有的账号标识，不包含账号凭证。
	cookieIDs, err := r.store.Cookies.ListOwnedIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	// result 保存跨账号聚合后的商品回复应用模型。
	result := make([]keywordsapp.ItemReply, 0)
	// cookieID 表示当前遍历的用户账号。
	for _, cookieID := range cookieIDs {
		// queryErr 表示当前账号商品回复读取失败。
		// rows 保存当前账号的商品回复数据库行。
		rows, queryErr := r.store.ItemReps.AllForUser(ctx, cookieID)
		if queryErr != nil {
			return nil, queryErr
		}
		// row 表示当前待转换的商品回复行。
		for _, row := range rows {
			result = append(result, itemReplyModel(row))
		}
	}
	return result, nil
}

// GetItemReply 读取指定用户账号和商品的商品回复。
func (r *KeywordRepository) GetItemReply(ctx context.Context, userID int64, cookieID, itemID string) (keywordsapp.ItemReply, error) {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return keywordsapp.ItemReply{}, err
	}
	// row、err 保存商品回复数据库行及读取错误。
	row, err := r.store.ItemReps.Get(ctx, cookieID, itemID)
	if errors.Is(err, db.ErrNotFound) {
		return keywordsapp.ItemReply{}, keywordsapp.ErrNotFound
	}
	if err != nil {
		return keywordsapp.ItemReply{}, err
	}
	return itemReplyModel(*row), nil
}

// SetItemReply 覆盖指定用户账号和商品的商品回复。
func (r *KeywordRepository) SetItemReply(ctx context.Context, userID int64, cookieID, itemID, content string) error {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	return r.store.ItemReps.Set(ctx, cookieID, itemID, content)
}

// DeleteItemReply 删除指定用户账号和商品的商品回复。
func (r *KeywordRepository) DeleteItemReply(ctx context.Context, userID int64, cookieID, itemID string) error {
	// err 表示账号归属校验失败。
	if err := r.authorize(ctx, userID, cookieID); err != nil {
		return err
	}
	return r.store.ItemReps.Delete(ctx, cookieID, itemID)
}

// authorize 只查询账号归属，不读取或解密任何凭证字段。
func (r *KeywordRepository) authorize(ctx context.Context, userID int64, cookieID string) error {
	// err 表示适配器依赖或用户身份校验失败。
	if err := r.validateUser(userID); err != nil {
		return err
	}
	if cookieID == "" {
		return keywordsapp.ErrInvalidInput
	}
	// owned、err 保存当前用户对账号的直接归属结果及查询错误。
	owned, err := r.store.Cookies.ExistsOwned(ctx, userID, cookieID)
	if err != nil {
		return err
	}
	if owned {
		return nil
	}
	// ownerID、err 保存账号所有者标识及读取错误，用于区分不存在和跨用户访问。
	ownerID, err := r.store.Cookies.GetOwnerID(ctx, cookieID)
	if errors.Is(err, db.ErrNotFound) {
		return keywordsapp.ErrNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != userID {
		return keywordsapp.ErrForbidden
	}
	return nil
}

// validateUser 检查数据库适配器和用户身份是否可用。
func (r *KeywordRepository) validateUser(userID int64) error {
	if r == nil || r.store == nil || r.store.Cookies == nil || r.store.Keywords == nil || r.store.ItemReps == nil {
		return errors.New("关键词数据库适配器未初始化")
	}
	if userID <= 0 {
		return keywordsapp.ErrInvalidUser
	}
	return nil
}

// keywordModel 将数据库关键词行转换为应用模型。
func keywordModel(row db.KeywordRow) keywordsapp.Keyword {
	return keywordsapp.Keyword{ID: row.ID, CookieID: row.CookieID, Keyword: row.Keyword, Reply: row.Reply, ItemID: row.ItemID, Type: row.Type, ImageURL: row.ImageURL}
}

// itemReplyModel 将数据库商品回复行转换为应用模型。
func itemReplyModel(row db.ItemReply) keywordsapp.ItemReply {
	return keywordsapp.ItemReply{ItemID: row.ItemID, CookieID: row.CookieID, ReplyContent: row.ReplyContent}
}

var _ keywordsapp.Repository = (*KeywordRepository)(nil)
