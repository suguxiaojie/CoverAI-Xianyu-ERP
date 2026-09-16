package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

// TestMigrate_AppliesCleanSchema 在临时库上跑迁移，验证全量 schema 干净落地、
// 关键不一致列（orders.system_shipped 等）存在、默认设置就位。
// TestMigrate_AppliesCleanSchema 封装TestMigrateAppliesCleanSchema业务协调。
func TestMigrate_AppliesCleanSchema(t *testing.T) {
	// tmp 用于本次流程后续判断的tmp
	tmp := t.TempDir()
	// dbPath 用于本次流程后续判断的db路径
	dbPath := filepath.Join(tmp, "test.db")

	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	// db、err 用于本次流程后续判断的db、err
	db, _, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// checks 用于本次流程后续判断的checks
	checks := []struct {
		table string
		col   string
	}{
		{"orders", "system_shipped"},
		{"orders", "receiver_city"},
		{"orders", "version"},
		{"orders", "deleted_at"},
		{"orders", "received_at"},
		{"orders", "refunded_at"},
		{"orders", "cancelled_at"},
		{"cards", "image_url"},
		{"cards", "delay_seconds"},
		{"keywords", "item_id"},
		{"item_info", "multi_quantity_delivery"},
		{"item_info", "deleted_at"},
		{"item_skus", "cost_cents"},
		{"item_skus", "synced_at"},
		{"keywords", "group_id"},
		{"keywords", "match_type"},
		{"keywords", "message_scope"},
		{"keywords", "system_types"},
		{"order_cost_snapshots", "unit_cost_cents"},
		{"order_cost_snapshots", "match_source"},
		{"automation_rules", "deleted_at"},
		{"default_replies", "reply_once"},
		{"default_reply_records", "status"},
		{"default_reply_records", "text_sent"},
		{"automation_runs", "action_cursor"},
		{"automation_runs", "action_started"},
		{"default_reply_records", "image_sent"},
		{"users", "is_admin"},
		{"sessions", "session_id"},
		{"notification_channels", "user_id"},
		{"notification_channels", "event_types"},
		{"message_notifications", "event_types"},
		{"scheduled_cookies_refresh_log", "step_details"},
		{"scheduled_cookies_refresh_log", "renew_method"},
		{"scheduled_cookies_refresh_log", "duration_ms"},
		{"scheduled_cookies_refresh_log", "request_count"},
		{"scheduled_login_renew_log", "step_details"},
		{"scheduled_login_renew_log", "updated_cookie_count"},
		{"scheduled_api_cookie_renew_log", "step_details"},
		{"scheduled_api_cookie_renew_log", "request_count"},
		{"risk_control_logs", "processing_status"},
		{"risk_control_logs", "duration_ms"},
		{"notification_outbox", "worker_token"},
		{"order_reconciliations", "idempotency_key"},
		{"security_audit_logs", "keys_json"},
		{"security_audit_logs", "outcome"},
		{"chat_messages", "platform_content_type"},
		{"chat_messages", "system_card_event"},
		{"chat_messages", "system_card_order_id"},
		{"chat_messages", "system_card_action"},
		{"chat_messages", "reply_to_platform_message_id"},
		{"account_task_settings", "auto_request_flower_enabled"},
		{"account_task_settings", "request_flower_after_hours"},
		{"account_task_settings", "request_flower_after_seconds"},
		{"account_task_settings", "auto_receive_flower_enabled"},
		{"account_task_settings", "receive_flower_show_browser"},
		{"account_task_settings", "receive_flower_timeout_seconds"},
		{"knowledge_bases", "status"},
		{"knowledge_faqs", "review_status"},
		{"knowledge_documents", "content_hash"},
		{"knowledge_chunks", "content_hash"},
		{"knowledge_retrieve_logs", "query_digest"},
		{"knowledge_faq_aliases", "normalized_alias"},
		{"keyword_event_reply_records", "status"},
	}
	// c 表示当前遍历过程中的c
	for _, c := range checks {
		if !columnExists(t, db, c.table, c.col) {
			t.Errorf("列缺失: %s.%s（应为收敛后的最终 schema）", c.table, c.col)
		}
	}

	// 默认系统设置应就位（qq_reply_secret_key 应为空，遵循无默认口令安全基线）。
	var val string
	err = db.QueryRow(`SELECT value FROM system_settings WHERE key='theme_color'`).Scan(&val)
	if err != nil || val != "blue" {
		t.Errorf("默认设置 theme_color 异常: val=%q err=%v", val, err)
	}
	err = db.QueryRow(`SELECT value FROM system_settings WHERE key='qq_reply_secret_key'`).Scan(&val)
	if err != nil || val != "" {
		t.Errorf("qq_reply_secret_key 应为空（无默认值安全基线）: val=%q err=%v", val, err)
	}
	err = db.QueryRow(`SELECT value FROM system_settings WHERE key='log_level'`).Scan(&val)
	if err != nil || val != "info" {
		t.Errorf("log_level 默认设置异常: val=%q err=%v", val, err)
	}
	err = db.QueryRow(`SELECT value FROM system_settings WHERE key='renewal_log_retention_days'`).Scan(&val)
	if err != nil || val != "10" {
		t.Errorf("renewal_log_retention_days 默认设置异常: val=%q err=%v", val, err)
	}

	// 二次 Open 应幂等（迁移不重复执行、不报错）。
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// db2、err 用于本次流程后续判断的db2、err
	db2, _, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("二次 Open 幂等失败: %v", err)
	}
	db2.Close()
}

// TestMigrate_UpgradesDatabaseWithMainChatVersions 验证已发布 main 的 00029/00030
// 聊天迁移可以原样升级到包含平台撤回和交易卡片状态的当前最终版本。
func TestMigrate_UpgradesDatabaseWithMainChatVersions(t *testing.T) {
	// tmpDir 保存隔离的已发布 main 数据库目录，测试结束后由 testing 清理。
	tmpDir := t.TempDir()
	// dbPath 指向模拟已运行至 main 00030 的 SQLite 文件。
	dbPath := filepath.Join(tmpDir, "main-chat-v30.db")
	// rawDB 在调用 Open 前控制 Goose 只执行已发布的 main 迁移。
	rawDB, openErr := sql.Open("sqlite", sqliteDSN(dbPath))
	if openErr != nil {
		t.Fatalf("open legacy database: %v", openErr)
	}
	defer rawDB.Close()

	// dialectErr 保存 Goose SQLite 方言设置失败，失败时不能构造已发布迁移基线。
	if // dialectErr 是 schema 37 测试数据库的 Goose 方言设置错误。
	dialectErr := goose.SetDialect("sqlite3"); dialectErr != nil {
		t.Fatalf("set goose dialect: %v", dialectErr)
	}
	goose.SetBaseFS(migrationsFS)
	// upErr 将数据库推进到已发布 main 的 00030，验证之后的 dev 迁移连续接续。
	upErr := goose.UpTo(rawDB, "migrations/sqlite", 30)
	if upErr != nil {
		t.Fatalf("apply released main migrations: %v", upErr)
	}

	// ctx 提供迁移 API 所需的调用上下文；升级本身不依赖请求生命周期。
	ctx := context.Background()
	// migrateErr 保存从 main 00030 接续 dev 00031 至 00034 时的迁移失败。
	if migrateErr := Migrate(ctx, rawDB, DialectSQLite); migrateErr != nil {
		t.Fatalf("upgrade from main 00030: %v", migrateErr)
	}
	if !tableExists(t, rawDB, "order_reconciliations") {
		t.Fatal("order_reconciliations should be created by the dev schema baseline migration")
	}
	if !columnExists(t, rawDB, "chat_messages", "read_status") || !columnExists(t, rawDB, "chat_messages", "read_at") {
		t.Fatal("chat read tracking columns should remain after dev schema baseline upgrade")
	}
	if !columnExists(t, rawDB, "chat_messages", "platform_message_id") || !columnExists(t, rawDB, "chat_messages", "recalled_at") {
		t.Fatal("chat recall columns should be created by migration 00034")
	}
	if !columnExists(t, rawDB, "chat_messages", "system_card_event") || !columnExists(t, rawDB, "chat_messages", "system_card_order_id") {
		t.Fatal("chat system card columns should be created by migration 00036")
	}
	if !tableExists(t, rawDB, "historical_order_cost_overrides") {
		t.Fatal("historical order cost overrides should be created by migration 00051")
	}
	// finalVersion 验证迁移账本已推进到包含自动确认收货提醒配置的最新 dev schema 版本。
	finalVersion, versionErr := goose.GetDBVersion(rawDB)
	if versionErr != nil {
		t.Fatalf("read final migration version: %v", versionErr)
	}
	if finalVersion != 53 {
		t.Fatalf("final migration version=%d, want 53", finalVersion)
	}
}

// TestMigrate41AddsChatReplyTarget 验证 schema 40 历史消息在升级后保留，并取得默认为空的原生引用目标列。
func TestMigrate41AddsChatReplyTarget(t *testing.T) {
	// databasePath 是停在 schema 40 的隔离 SQLite 数据库路径。
	databasePath := filepath.Join(t.TempDir(), "schema-40.db")
	// rawDB 是手动控制 Goose 迁移版本的数据库连接。
	rawDB, openErr := sql.Open("sqlite", sqliteDSN(databasePath))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer rawDB.Close()
	if // dialectErr 是设置 Goose SQLite 方言的错误。
	dialectErr := goose.SetDialect("sqlite3"); dialectErr != nil {
		t.Fatal(dialectErr)
	}
	goose.SetBaseFS(migrationsFS)
	if // migrateErr 是将隔离数据库推进到 schema 40 的错误。
	migrateErr := goose.UpTo(rawDB, "migrations/sqlite", 40); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	if // userErr 是创建历史聊天账号所属 ERP 用户的错误。
	_, userErr := rawDB.Exec(`INSERT INTO users(id,username,email,password_hash) VALUES(?,?,?,?)`, 41, "reply-migration-owner", "reply-migration@example.com", "test-hash"); userErr != nil {
		t.Fatal(userErr)
	}
	if // accountErr 是创建不包含真实凭证的迁移账号夹具的错误。
	_, accountErr := rawDB.Exec(`INSERT INTO cookies(id,value,user_id) VALUES(?,?,?)`, "reply-migration-account", "test-cookie", 41); accountErr != nil {
		t.Fatal(accountErr)
	}
	if // sessionErr 是创建 schema 40 历史会话的错误。
	_, sessionErr := rawDB.Exec(`INSERT INTO chat_sessions(cookie_id,chat_id,last_message,last_message_at,unread_count,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, "reply-migration-account", "reply-migration-chat", "history", 100, 0, 100, 100); sessionErr != nil {
		t.Fatal(sessionErr)
	}
	if // messageErr 是创建 schema 40 普通历史消息的错误。
	_, messageErr := rawDB.Exec(`INSERT INTO chat_messages(cookie_id,chat_id,message_key,direction,content,sent_at) VALUES(?,?,?,?,?,?)`, "reply-migration-account", "reply-migration-chat", "history.PNM", "incoming", "history", 100); messageErr != nil {
		t.Fatal(messageErr)
	}
	if // migrateErr 是从 schema 40 升级到当前最终版本的错误。
	migrateErr := Migrate(context.Background(), rawDB, DialectSQLite); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	// replyTarget 是历史普通消息升级后的安全空默认值。
	var replyTarget string
	if // readErr 是读取新列默认值的错误。
	readErr := rawDB.QueryRow(`SELECT reply_to_platform_message_id FROM chat_messages WHERE message_key=?`, "history.PNM").Scan(&replyTarget); readErr != nil || replyTarget != "" {
		t.Fatalf("reply_target=%q err=%v", replyTarget, readErr)
	}
}

// TestMigrate40AddsPersistentChatSessionPinning 验证 schema 39 升级后保留会话并新增默认关闭的置顶字段。
func TestMigrate40AddsPersistentChatSessionPinning(t *testing.T) {
	// databasePath 是停在 schema 39 的隔离 SQLite 数据库路径。
	databasePath := filepath.Join(t.TempDir(), "schema-39.db")
	// rawDB 是手动控制 Goose 迁移版本的数据库连接。
	rawDB, openErr := sql.Open("sqlite", sqliteDSN(databasePath))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer rawDB.Close()
	if // dialectErr 是设置 Goose SQLite 方言的错误。
	dialectErr := goose.SetDialect("sqlite3"); dialectErr != nil {
		t.Fatal(dialectErr)
	}
	goose.SetBaseFS(migrationsFS)
	if // migrateErr 是将测试数据库推进到 schema 39 的错误。
	migrateErr := goose.UpTo(rawDB, "migrations/sqlite", 39); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	if // userErr 是创建升级前会话所属用户的错误。
	_, userErr := rawDB.Exec(`INSERT INTO users(id,username,email,password_hash) VALUES(?,?,?,?)`, 40, "pin-migration-owner", "pin-migration@example.com", "test-hash"); userErr != nil {
		t.Fatal(userErr)
	}
	if // accountErr 是创建升级前会话所属账号的错误。
	_, accountErr := rawDB.Exec(`INSERT INTO cookies(id,value,user_id) VALUES(?,?,?)`, "pin-migration-account", "test-cookie", 40); accountErr != nil {
		t.Fatal(accountErr)
	}
	if // sessionErr 是创建 schema 39 历史会话的错误。
	_, sessionErr := rawDB.Exec(`INSERT INTO chat_sessions(cookie_id,chat_id,last_message,last_message_at,unread_count,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, "pin-migration-account", "pin-migration-chat", "history", 100, 0, 100, 100); sessionErr != nil {
		t.Fatal(sessionErr)
	}
	if // migrateErr 是从 schema 39 升级到当前版本的错误。
	migrateErr := Migrate(context.Background(), rawDB, DialectSQLite); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	// isPinned 是升级后历史会话的安全默认置顶状态。
	var isPinned bool
	// pinnedAt 是升级后历史会话的默认置顶时间，未置顶时必须为零。
	var pinnedAt int64
	if // readErr 是读取升级后会话默认值的错误。
	readErr := rawDB.QueryRow(`SELECT is_pinned,pinned_at FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, "pin-migration-account", "pin-migration-chat").Scan(&isPinned, &pinnedAt); readErr != nil || isPinned || pinnedAt != 0 {
		t.Fatalf("is_pinned=%v pinned_at=%d err=%v", isPinned, pinnedAt, readErr)
	}
}

// TestMigrate39AddsIndependentLifecycleMilestones 验证 schema 38 升级后保留订单并新增收货、退款和取消时间。
func TestMigrate39AddsIndependentLifecycleMilestones(t *testing.T) {
	// databasePath 是停在 schema 38 的隔离 SQLite 数据库。
	databasePath := filepath.Join(t.TempDir(), "schema-38.db")
	// rawDB 是手动控制迁移版本的数据库连接。
	rawDB, openErr := sql.Open("sqlite", sqliteDSN(databasePath))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer rawDB.Close()
	// dialectErr 是设置 Goose SQLite 方言的错误。
	dialectErr := goose.SetDialect("sqlite3")
	if dialectErr != nil {
		t.Fatal(dialectErr)
	}
	goose.SetBaseFS(migrationsFS)
	if // migrateErr 是测试数据库推进到 schema 38 的错误。
	migrateErr := goose.UpTo(rawDB, "migrations/sqlite", 38); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	if // insertErr 是写入升级前订单夹具的错误。
	_, insertErr := rawDB.Exec(`INSERT INTO orders(order_id,order_status) VALUES (?,?)`, "lifecycle-order", "shipped"); insertErr != nil {
		t.Fatal(insertErr)
	}
	if // migrateErr 是从 schema 38 升级到当前版本的错误。
	migrateErr := Migrate(context.Background(), rawDB, DialectSQLite); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	// status、receivedAt、refundedAt、cancelledAt 是升级后保留的状态和新增空里程碑。
	var status, receivedAt, refundedAt, cancelledAt string
	if // readErr 是读取升级后订单字段的错误。
	readErr := rawDB.QueryRow(`SELECT order_status,COALESCE(received_at,''),COALESCE(refunded_at,''),COALESCE(cancelled_at,'') FROM orders WHERE order_id=?`, "lifecycle-order").Scan(&status, &receivedAt, &refundedAt, &cancelledAt); readErr != nil {
		t.Fatal(readErr)
	}
	if status != "shipped" || receivedAt != "" || refundedAt != "" || cancelledAt != "" {
		t.Fatalf("status=%q received=%q refunded=%q cancelled=%q", status, receivedAt, refundedAt, cancelledAt)
	}
}

// TestMigrate38ConvertsEnabledHoursAndResetsDisabledDefaults 验证旧小时配置只在已启用账号上保留等值秒数。
func TestMigrate38ConvertsEnabledHoursAndResetsDisabledDefaults(t *testing.T) {
	// databasePath 是停在 schema 37 的隔离 SQLite 数据库。
	databasePath := filepath.Join(t.TempDir(), "schema-37.db")
	// rawDB 是手动控制迁移版本的数据库连接。
	rawDB, openErr := sql.Open("sqlite", sqliteDSN(databasePath))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer rawDB.Close()
	// dialectErr 是 schema 37 测试数据库的 Goose 方言设置错误。
	dialectErr := goose.SetDialect("sqlite3")
	if dialectErr != nil {
		t.Fatal(dialectErr)
	}
	goose.SetBaseFS(migrationsFS)
	if // migrateErr 是测试数据库推进到 schema 37 的错误。
	migrateErr := goose.UpTo(rawDB, "migrations/sqlite", 37); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	// store 提供创建测试用户和两个账号所需的 schema 37 仓储。
	store := NewStore(rawDB, DialectSQLite)
	// created、userErr 是测试管理员创建结果。
	created, userErr := store.Users.Create(context.Background(), "schema38", "schema38@example.com", "pw")
	if userErr != nil || !created {
		t.Fatalf("create user=%v err=%v", created, userErr)
	}
	// user 是两个测试账号的归属用户。
	user, findErr := store.Users.GetByUsername(context.Background(), "schema38")
	if findErr != nil {
		t.Fatal(findErr)
	}
	// accountID 是当前待创建的旧配置账号标识。
	for _, accountID := range []string{"enabled-hours", "disabled-hours"} {
		if // saveErr 是 schema 37 测试账号保存错误。
		saveErr := store.Cookies.Save(context.Background(), accountID, "unb=1; _m_h5_tk=token_1", user.ID); saveErr != nil {
			t.Fatal(saveErr)
		}
	}
	// now 是 schema 37 设置行必需的创建和更新时间。
	now := time.Now().UTC().Unix()
	// _, insertErr 写入一个已启用两小时配置和一个未启用五小时配置。
	_, insertErr := rawDB.ExecContext(context.Background(), `INSERT INTO account_task_settings
		(cookie_id,auto_request_flower_enabled,request_flower_after_hours,created_at,updated_at) VALUES
		(?,1,2,?,?),(?,0,5,?,?)`, "enabled-hours", now, now, "disabled-hours", now, now)
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	if // migrateErr 是 schema 37 升级到当前版本的错误。
	migrateErr := Migrate(context.Background(), rawDB, DialectSQLite); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	// enabledSeconds、disabledSeconds 是 schema 38 转换后的权威秒数。
	var enabledSeconds, disabledSeconds int
	if // queryErr 是已启用旧小时配置的秒数查询错误。
	queryErr := rawDB.QueryRowContext(context.Background(), `SELECT request_flower_after_seconds FROM account_task_settings WHERE cookie_id=?`, "enabled-hours").Scan(&enabledSeconds); queryErr != nil {
		t.Fatal(queryErr)
	}
	if // queryErr 是未启用旧小时配置的新默认秒数查询错误。
	queryErr := rawDB.QueryRowContext(context.Background(), `SELECT request_flower_after_seconds FROM account_task_settings WHERE cookie_id=?`, "disabled-hours").Scan(&disabledSeconds); queryErr != nil {
		t.Fatal(queryErr)
	}
	if enabledSeconds != 7200 || disabledSeconds != 10 {
		t.Fatalf("enabled seconds=%d disabled seconds=%d", enabledSeconds, disabledSeconds)
	}
}

// columnExists 封装columnExists业务协调。
func columnExists(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		// name 用于本次流程后续判断的名称
		var name string
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == col {
			return true
		}
	}
	return false
}

// TestLatestMigrationsDownUpSQLite 封装TestLatestMigrationsDownUpSQLite业务协调。
func TestLatestMigrationsDownUpSQLite(t *testing.T) {
	// tmp 用于本次流程后续判断的tmp
	tmp := t.TempDir()
	// dbPath 用于本次流程后续判断的db路径
	dbPath := filepath.Join(tmp, "rollback.db")
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	// d、err 用于本次流程后续判断的d、err
	d, _, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if // err 用于本次流程后续判断的err
	err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	goose.SetBaseFS(migrationsFS)
	// 读取当前最新迁移版本，动态回滚到 13，避免新增迁移后固定次数失效。
	// version、err 保存当前迁移版本及读取错误。
	version, err := goose.GetDBVersion(d)
	if err != nil {
		t.Fatalf("get migration version: %v", err)
	}
	// i 表示本次回滚操作序号。
	for i := 0; version >= 14; i++ {
		if // err 保存当前迁移回滚错误，任一方言步骤失败即终止验证。
		err := goose.Down(d, "migrations/sqlite"); err != nil {
			t.Fatalf("down migration #%d: %v", i+1, err)
		}
		version, err = goose.GetDBVersion(d)
		if err != nil {
			t.Fatalf("get migration version after down: %v", err)
		}
	}
	if columnExists(t, d, "notification_channels", "event_types") {
		t.Fatal("event_types should be removed after migration 14 down")
	}
	if tableExists(t, d, "risk_control_logs") {
		t.Fatal("risk_control_logs should be removed after migration 14 down")
	}
	if columnExists(t, d, "default_reply_records", "status") {
		t.Fatal("default_reply_records.status should be removed after migration 16 down")
	}
	if columnExists(t, d, "account_tokens", "cookie_fingerprint") {
		t.Fatal("account_tokens.cookie_fingerprint should be removed after migration 22 down")
	}
	if columnExists(t, d, "item_publish_batch_rows", "category_json") {
		t.Fatal("item_publish_batch_rows.category_json should be removed after migration 23 down")
	}
	if columnExists(t, d, "item_info", "deleted_at") || columnExists(t, d, "automation_rules", "deleted_at") {
		t.Fatal("soft-delete columns should be removed after migration 26 down")
	}
	if columnExists(t, d, "orders", "deleted_at") {
		t.Fatal("orders.deleted_at should be removed after migration 27 down")
	}
	if columnExists(t, d, "item_publish_batches", "location_json") {
		t.Fatal("item_publish_batches.location_json should be removed after migration 28 down")
	}
	// table 表示当前遍历过程中的table
	for _, table := range []string{"account_task_settings", "account_task_runs", "chat_sessions", "chat_messages"} {
		if tableExists(t, d, table) {
			t.Fatalf("table should be removed after migration 24 down: %s", table)
		}
	}

	if // err 用于本次流程后续判断的err
	err := goose.Up(d, "migrations/sqlite"); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	// c 表示当前遍历过程中的c
	for _, c := range []struct {
		table string
		col   string
	}{
		{"notification_channels", "event_types"},
		{"message_notifications", "event_types"},
		{"scheduled_cookies_refresh_log", "step_details"},
		{"scheduled_login_renew_log", "updated_cookie_count"},
		{"scheduled_api_cookie_renew_log", "request_count"},
		{"risk_control_logs", "processing_status"},
		{"default_reply_records", "status"},
		{"default_reply_records", "text_sent"},
		{"account_tokens", "cookie_fingerprint"},
		{"item_publish_batch_rows", "category_json"},
		{"item_publish_batches", "location_json"},
		{"account_task_settings", "auto_rate_enabled"},
		{"account_task_runs", "run_key"},
		{"chat_sessions", "unread_count"},
		{"chat_messages", "message_key"},
		{"chat_messages", "read_status"},
		{"chat_messages", "read_at"},
		{"notification_outbox", "uncertain_at"},
	} {
		if !columnExists(t, d, c.table, c.col) {
			t.Fatalf("column missing after re-up: %s.%s", c.table, c.col)
		}
	}
	// val 用于本次流程后续判断的val
	var val string
	if // err 用于本次流程后续判断的err
	err := d.QueryRow(`SELECT value FROM system_settings WHERE key='renewal_log_retention_days'`).Scan(&val); err != nil || val != "10" {
		t.Fatalf("renewal_log_retention_days after re-up: val=%q err=%v", val, err)
	}
}

// tableExists 封装tableExists业务协调。
func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	// name 用于本次流程后续判断的名称
	var name string
	// err 用于本次流程后续判断的err
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err != nil {
		return false
	}
	return name == table
}
