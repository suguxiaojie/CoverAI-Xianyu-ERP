package ws

import (
	"net/url"
	"testing"
)

// TestBuildLocationCardContent 验证自定义文案只进入官方地图页参数，并保留真实坐标和官方预览图。
func TestBuildLocationCardContent(t *testing.T) {
	// content、buildErr 是使用边界内坐标构造的位置卡片及本地校验错误。
	content, buildErr := buildLocationCardContent("CoverAI 实体店", "东门电梯上楼右转", 22.540503, 113.934528)
	if buildErr != nil {
		t.Fatalf("构造位置卡片失败: %v", buildErr)
	}
	// card、cardOK 是 contentType=30 下的位置卡片对象及类型断言结果。
	card, cardOK := content["locationCard"].(map[string]any)
	if content["contentType"] != 30 || !cardOK {
		t.Fatalf("位置卡片协议结构错误: %#v", content)
	}
	if card["title"] != "CoverAI 实体店" || card["content"] != "东门电梯上楼右转" || card["latitude"] != "22.540503" || card["longitude"] != "113.934528" {
		t.Fatalf("位置卡片展示字段错误: %#v", card)
	}
	// action、actionOK 是位置卡片点击动作及类型断言结果。
	action, actionOK := card["action"].(map[string]any)
	// page、pageOK 是固定官方地图页动作及类型断言结果。
	page, pageOK := action["page"].(map[string]any)
	// pageURL、parseErr 是解析后的官方页面地址及 URL 错误。
	pageURL, parseErr := url.Parse(stringValue(page["url"]))
	if !actionOK || !pageOK || parseErr != nil {
		t.Fatalf("位置卡片动作错误: action=%#v page=%#v err=%v", action, page, parseErr)
	}
	if pageURL.Host != "market.m.taobao.com" || pageURL.Query().Get("mainTitle") != "CoverAI 实体店" || pageURL.Query().Get("subTitle") != "东门电梯上楼右转" {
		t.Fatalf("位置卡片官方页面参数错误: %s", pageURL.String())
	}
}

// TestBuildLocationCardContentRejectsInvalidCoordinates 验证无效坐标在任何平台写入前被拒绝。
func TestBuildLocationCardContentRejectsInvalidCoordinates(t *testing.T) {
	// buildErr 是超范围坐标在协议构造阶段必须返回的校验错误。
	if _, buildErr := buildLocationCardContent("店铺", "地址", 91, 120); buildErr == nil {
		t.Fatal("超范围纬度应被拒绝")
	}
}

// stringValue 把测试协议字段安全转换为字符串；非字符串字段返回空值以触发断言失败。
func stringValue(value any) string {
	// text、ok 是字段的字符串值及类型断言结果。
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
