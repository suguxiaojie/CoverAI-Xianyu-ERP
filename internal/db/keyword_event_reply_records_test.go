package db

import (
	"context"
	"path/filepath"
	"testing"
)

// TestKeywordEventReplyClaimPersistsSuccess 验证订单系统事件只能由一个租约发送并在成功后永久去重。
func TestKeywordEventReplyClaimPersistsSuccess(t *testing.T) {
	// databasePath 是只服务于幂等测试的临时 SQLite 文件。
	databasePath := filepath.Join(t.TempDir(), "keyword-event.db")
	// ctx 是全部本地幂等操作共用的测试上下文。
	ctx := context.Background()
	// database 是迁移到最新 Schema 的隔离数据库。
	database, dialect, openErr := Open(ctx, databasePath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer database.Close()
	// keywords 是待验证订单事件租约的仓储实例。
	keywords := &Keywords{DB: database, Dialect: dialect}
	// claimed 是首个实时卖家卡片取得发送权的结果。
	claimed, claimErr := keywords.ClaimKeywordEventReply(ctx, "shop", "order-1", "group-1", "order_shipped", "token-a", 100, 200)
	if claimErr != nil || !claimed {
		t.Fatalf("first claim=%v err=%v", claimed, claimErr)
	}
	// duplicateClaim 是有效租约期间买家通知尝试重复发送的结果。
	duplicateClaim, duplicateErr := keywords.ClaimKeywordEventReply(ctx, "shop", "order-1", "group-1", "order_shipped", "token-b", 101, 201)
	if duplicateErr != nil || duplicateClaim {
		t.Fatalf("duplicate claim=%v err=%v", duplicateClaim, duplicateErr)
	}
	// releaseErr 是模拟发送前失败释放首个租约的结果。
	if releaseErr := keywords.ReleaseKeywordEventReply(ctx, "shop", "order-1", "group-1", "order_shipped", "token-a"); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	// retryClaim 是发送前失败释放后由下一条实时事件取得发送权的结果。
	retryClaim, retryErr := keywords.ClaimKeywordEventReply(ctx, "shop", "order-1", "group-1", "order_shipped", "token-c", 102, 202)
	if retryErr != nil || !retryClaim {
		t.Fatalf("retry claim=%v err=%v", retryClaim, retryErr)
	}
	// successErr 是第二次租约完成永久幂等状态的结果。
	if successErr := keywords.MarkKeywordEventReplySuccess(ctx, "shop", "order-1", "group-1", "order_shipped", "token-c", 103); successErr != nil {
		t.Fatal(successErr)
	}
	// afterSuccess 是成功记录即使超过旧租约时间也拒绝重复发送的结果。
	afterSuccess, afterSuccessErr := keywords.ClaimKeywordEventReply(ctx, "shop", "order-1", "group-1", "order_shipped", "token-d", 1000, 1100)
	if afterSuccessErr != nil || afterSuccess {
		t.Fatalf("after success claim=%v err=%v", afterSuccess, afterSuccessErr)
	}
}
