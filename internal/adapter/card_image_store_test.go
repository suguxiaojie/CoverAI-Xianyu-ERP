package adapter

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	cardsapp "xianyu-go/internal/application/cards"
)

// testCardPNG 返回一个可由 net/http 魔数检测识别的 1×1 PNG 测试图片。
func testCardPNG(t *testing.T) []byte {
	t.Helper()
	// data、decodeErr 分别保存固定 PNG 夹具字节和 Base64 解码错误。
	data, decodeErr := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if decodeErr != nil {
		t.Fatalf("解码 PNG 夹具失败: %v", decodeErr)
	}
	return data
}

// TestCardImageStoreSaveReadAndDeduplicate 验证图片按用户和内容哈希不可变保存、重复复用并受所有权保护。
func TestCardImageStoreSaveReadAndDeduplicate(t *testing.T) {
	// root 是当前测试独占的受管图片目录。
	root := filepath.Join(t.TempDir(), "card-images")
	// store 是待验证的本地文件适配器。
	store := NewCardImageStore(root)
	// input 是合法 PNG 图片上传输入。
	input := cardsapp.ImageInput{Filename: "用户目录/card.png", ContentType: "text/plain", Data: testCardPNG(t)}
	// first、saveErr 保存首次落盘结果。
	first, saveErr := store.Save(context.Background(), 7, input)
	if saveErr != nil || first.ContentType != "image/png" || !cardsapp.IsManagedImageReference(first.Reference) {
		t.Fatalf("首次保存异常 asset=%+v err=%v", first, saveErr)
	}
	// second、deduplicateErr 保存相同内容再次上传的去重结果。
	second, deduplicateErr := store.Save(context.Background(), 7, input)
	if deduplicateErr != nil || second.Reference != first.Reference {
		t.Fatalf("重复内容应复用引用 first=%q second=%q err=%v", first.Reference, second.Reference, deduplicateErr)
	}
	// loaded、readErr 保存同用户读取后的真实图片内容。
	loaded, readErr := store.ReadOwned(context.Background(), 7, first.Reference)
	if readErr != nil || string(loaded.Data) != string(input.Data) || loaded.ContentType != "image/png" {
		t.Fatalf("读取结果异常 asset=%+v err=%v", loaded, readErr)
	}
	// forbiddenErr 是其他用户尝试读取受管图片时的所有权错误。
	if _, forbiddenErr := store.ReadOwned(context.Background(), 8, first.Reference); !errors.Is(forbiddenErr, cardsapp.ErrForbidden) {
		t.Fatalf("跨用户读取应拒绝: %v", forbiddenErr)
	}
	// entries、directoryErr 分别保存用户目录中的不可变文件列表和读取错误。
	entries, directoryErr := os.ReadDir(filepath.Join(root, "7"))
	if directoryErr != nil || len(entries) != 1 {
		t.Fatalf("内容去重后应只有一个文件 entries=%d err=%v", len(entries), directoryErr)
	}
}

// TestCardImageStoreRejectsInvalidContentAndMissingFile 验证伪图片、空文件和不存在引用不会通过文件边界。
func TestCardImageStoreRejectsInvalidContentAndMissingFile(t *testing.T) {
	// store 是当前测试独占的本地文件适配器。
	store := NewCardImageStore(t.TempDir())
	// invalidInput 是伪造 image/png 声明的普通文本。
	invalidInput := cardsapp.ImageInput{Filename: "fake.png", ContentType: "image/png", Data: []byte("not an image")}
	// saveErr 是伪造 MIME 内容被真实魔数门禁拒绝的结果。
	if _, saveErr := store.Save(context.Background(), 7, invalidInput); saveErr == nil {
		t.Fatal("伪造 MIME 的普通文本不得保存")
	}
	// missingReference 是格式合法但没有对应磁盘文件的受管引用。
	missingReference := cardsapp.BuildManagedImageReference(7, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png")
	// readErr 是格式合法但磁盘文件不存在时的读取结果。
	if _, readErr := store.ReadReference(context.Background(), missingReference); !errors.Is(readErr, cardsapp.ErrNotFound) {
		t.Fatalf("不存在文件应返回未找到: %v", readErr)
	}
}
