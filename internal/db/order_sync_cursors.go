package db

import (
	"context"
	"database/sql"
	"errors"
)

// OrderSyncCursors 持久化每个闲鱼账号的订单增量同步高水位。
type OrderSyncCursors struct {
	// DB 保存游标表使用的数据库连接。
	DB *sql.DB
	// Dialect 决定跨方言 UPSERT 语法。
	Dialect Dialect
}

// OrderSyncCursor 是账号订单同步游标的持久化模型。
type OrderSyncCursor struct {
	// CookieID 是游标所属账号标识。
	CookieID string
	// HighWaterCreatedAt 是已成功落库订单中的最新平台创建时间。
	HighWaterCreatedAt string
	// HighWaterOrderID 在创建时间相同时用于稳定比较订单边界。
	HighWaterOrderID string
	// LastIncrementalSyncAt 是最近一次增量同步成功结束的 Unix 秒。
	LastIncrementalSyncAt int64
	// LastFullSyncAt 是最近一次完整快照校准成功结束的 Unix 秒。
	LastFullSyncAt int64
	// UpdatedAt 是游标最后更新时间。
	UpdatedAt string
}

// Get 读取账号同步游标；不存在时通过 exists=false 返回，不把缺失当作数据库故障。
func (c *OrderSyncCursors) Get(ctx context.Context, cookieID string) (*OrderSyncCursor, bool, error) {
	// cursor 保存数据库返回的同步边界。
	cursor := &OrderSyncCursor{}
	// err 保存游标查询或扫描错误。
	err := c.DB.QueryRowContext(ctx, `SELECT cookie_id,high_water_created_at,high_water_order_id,
		last_incremental_sync_at,last_full_sync_at,updated_at FROM order_sync_cursors WHERE cookie_id=?`, cookieID).Scan(
		&cursor.CookieID, &cursor.HighWaterCreatedAt, &cursor.HighWaterOrderID,
		&cursor.LastIncrementalSyncAt, &cursor.LastFullSyncAt, &cursor.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cursor, true, nil
}

// Upsert 原子保存账号高水位和最近同步时间，供下一次增量扫描安全提前停止。
func (c *OrderSyncCursors) Upsert(ctx context.Context, cursor OrderSyncCursor) error {
	// query 保存当前数据库方言对应的游标 UPSERT 语句。
	query := `INSERT INTO order_sync_cursors
		(cookie_id,high_water_created_at,high_water_order_id,last_incremental_sync_at,last_full_sync_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(cookie_id) DO UPDATE SET
		high_water_created_at=excluded.high_water_created_at,
		high_water_order_id=excluded.high_water_order_id,
		last_incremental_sync_at=excluded.last_incremental_sync_at,
		last_full_sync_at=excluded.last_full_sync_at,
		updated_at=CURRENT_TIMESTAMP`
	if c.Dialect == DialectMySQL {
		query = `INSERT INTO order_sync_cursors
			(cookie_id,high_water_created_at,high_water_order_id,last_incremental_sync_at,last_full_sync_at)
			VALUES (?,?,?,?,?)
			ON DUPLICATE KEY UPDATE
			high_water_created_at=VALUES(high_water_created_at),
			high_water_order_id=VALUES(high_water_order_id),
			last_incremental_sync_at=VALUES(last_incremental_sync_at),
			last_full_sync_at=VALUES(last_full_sync_at)`
	}
	// err 保存游标插入或更新错误。
	_, err := c.DB.ExecContext(ctx, query, cursor.CookieID, cursor.HighWaterCreatedAt, cursor.HighWaterOrderID,
		cursor.LastIncrementalSyncAt, cursor.LastFullSyncAt)
	return err
}
