package cards

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// imageStoreStub 记录卡券图片应用服务对受管存储 Port 的调用，并返回可控资产。
type imageStoreStub struct {
	// savedUserID 是最近一次保存图片时接收的本地用户标识。
	savedUserID int64
	// savedInput 是最近一次保存的图片元数据和瞬时字节。
	savedInput ImageInput
	// saveAsset 是保存成功时返回的内部引用。
	saveAsset ImageAsset
	// saveErr 是保存阶段注入的基础设施错误。
	saveErr error
	// readUserID 是最近一次受所有权保护读取的用户标识。
	readUserID int64
	// readReference 是最近一次读取的内部引用。
	readReference string
	// readAsset 是读取成功时返回的图片内容。
	readAsset ImageAsset
	// readErr 是读取阶段注入的基础设施错误。
	readErr error
}

// Save 记录受管图片保存输入并返回预设资产或错误。
func (store *imageStoreStub) Save(_ context.Context, userID int64, input ImageInput) (ImageAsset, error) {
	store.savedUserID = userID
	store.savedInput = input
	return store.saveAsset, store.saveErr
}

// ReadOwned 记录预览读取的用户和引用并返回预设图片内容。
func (store *imageStoreStub) ReadOwned(_ context.Context, userID int64, reference string) (ImageAsset, error) {
	store.readUserID = userID
	store.readReference = reference
	return store.readAsset, store.readErr
}

// TestManagedImageReferenceRoundTrip 验证内部引用只接受正用户标识、内容哈希文件名和固定图片扩展名。
func TestManagedImageReferenceRoundTrip(t *testing.T) {
	// storageName 是合法 SHA-256 内容哈希组成的 PNG 文件名。
	storageName := strings.Repeat("a", 64) + ".png"
	// reference 是使用合法输入构造的内部图片引用。
	reference := BuildManagedImageReference(7, storageName)
	// userID、parsedName、ok 分别保存解析出的用户、文件名和格式有效性。
	userID, parsedName, ok := ParseManagedImageReference(reference)
	if !ok || userID != 7 || parsedName != storageName || !IsManagedImageReference(reference) {
		t.Fatalf("受管引用往返异常 reference=%q user=%d name=%q ok=%t", reference, userID, parsedName, ok)
	}
	// invalidReference 是当前待拒绝的畸形或越界引用。
	for _, invalidReference := range []string{
		"https://example.com/card.png",
		"card-image://7/../secret.png",
		"card-image://7/" + strings.Repeat("a", 64) + ".gif",
		"card-image://0/" + storageName,
		"card-image://7/" + storageName + "?download=1",
	} {
		if IsManagedImageReference(invalidReference) {
			t.Fatalf("非法受管引用未被拒绝: %q", invalidReference)
		}
	}
}

// TestServiceManagedImageLifecycle 验证上传创建、同卡保留引用、替换图片和鉴权预览使用同一用户边界。
func TestServiceManagedImageLifecycle(t *testing.T) {
	// firstReference 是初次上传生成的受管图片引用。
	firstReference := BuildManagedImageReference(7, strings.Repeat("1", 64)+".png")
	// secondReference 是编辑替换图片后生成的新受管引用。
	secondReference := BuildManagedImageReference(7, strings.Repeat("2", 64)+".jpg")
	// repository 是记录卡券创建和更新模型的持久化替身。
	repository := &cardRepositoryStub{createdID: 19, card: Card{ID: 19, UserID: 7, Type: "image", ImageURL: firstReference}}
	// imageStore 是返回两次不同引用的受管图片存储替身。
	imageStore := &imageStoreStub{saveAsset: ImageAsset{Reference: firstReference}, readAsset: ImageAsset{Reference: firstReference, ContentType: "image/png", Data: []byte("png")}}
	// service 是同时装配卡券仓储和图片存储的应用服务。
	service := NewServiceWithImages(repository, imageStore)
	// input 是当前请求内保存的图片字节和展示元数据。
	input := &ImageInput{Filename: "card.png", ContentType: "image/png", Data: []byte("png")}
	// createdID、createErr 保存图片卡券创建结果。
	createdID, createErr := service.CreateWithImage(context.Background(), 7, Draft{Name: "教程图", Type: "image", Enabled: true}, input)
	if createErr != nil || createdID != 19 || repository.createdCard.ImageURL != firstReference || imageStore.savedUserID != 7 {
		t.Fatalf("上传创建异常 id=%d card=%+v user=%d err=%v", createdID, repository.createdCard, imageStore.savedUserID, createErr)
	}
	// keepErr 是不上传新文件时继续保存原受管引用的更新结果。
	keepErr := service.Update(context.Background(), 7, 19, Draft{Name: "教程图更新", Type: "image", ImageURL: firstReference, Enabled: true})
	if keepErr != nil || repository.updatedCard.ImageURL != firstReference {
		t.Fatalf("保留原受管引用失败 card=%+v err=%v", repository.updatedCard, keepErr)
	}
	// forgedReference 是试图从普通 JSON 更新中引用其他受管文件的地址。
	forgedReference := BuildManagedImageReference(8, strings.Repeat("3", 64)+".png")
	// forgedErr 是普通 JSON 请求尝试替换为其他受管引用时的校验错误。
	if forgedErr := service.Update(context.Background(), 7, 19, Draft{Name: "越界图", Type: "image", ImageURL: forgedReference}); forgedErr == nil {
		t.Fatal("普通更新不得改为未由本次上传生成的受管引用")
	}
	imageStore.saveAsset = ImageAsset{Reference: secondReference}
	// replaceErr 是上传新文件替换受管引用的更新结果。
	replaceErr := service.UpdateWithImage(context.Background(), 7, 19, Draft{Name: "新教程图", Type: "image", Enabled: true}, input)
	if replaceErr != nil || repository.updatedCard.ImageURL != secondReference {
		t.Fatalf("替换图片失败 card=%+v err=%v", repository.updatedCard, replaceErr)
	}
	// asset、readErr 保存认证预览读取结果。
	asset, readErr := service.GetImage(context.Background(), 7, 19)
	if readErr != nil || string(asset.Data) != "png" || imageStore.readUserID != 7 || imageStore.readReference != firstReference {
		t.Fatalf("预览读取异常 asset=%+v user=%d ref=%q err=%v", asset, imageStore.readUserID, imageStore.readReference, readErr)
	}
}

// TestServiceManagedImageErrors 验证缺少图片存储、错误类型上传和远程 URL 预览均返回可区分错误。
func TestServiceManagedImageErrors(t *testing.T) {
	// repository 是归属于用户 7 的远程图片卡券替身。
	repository := &cardRepositoryStub{card: Card{ID: 5, UserID: 7, Type: "image", ImageURL: "https://example.com/card.png"}}
	// service 是未装配受管图片存储的卡券服务。
	service := NewService(repository)
	// input 是满足非空条件的最小图片输入。
	input := &ImageInput{Filename: "card.png", Data: []byte("png")}
	// createErr 是运行时缺少图片存储时的明确不可用错误。
	if _, createErr := service.CreateWithImage(context.Background(), 7, Draft{Name: "图", Type: "image"}, input); !errors.Is(createErr, ErrImageStoreUnavailable) {
		t.Fatalf("缺少存储应返回不可用错误: %v", createErr)
	}
	// imageStore 是已装配但不执行真实文件操作的图片存储替身。
	imageStore := &imageStoreStub{}
	service = NewServiceWithImages(repository, imageStore)
	// typeErr 是非图片卡券携带文件时的类型校验错误。
	if _, typeErr := service.CreateWithImage(context.Background(), 7, Draft{Name: "文本", Type: "text", TextContent: "x"}, input); typeErr == nil {
		t.Fatal("非图片卡券不得携带上传文件")
	}
	// previewErr 是远程 URL 卡券误用受管预览端点时的错误分类。
	if _, previewErr := service.GetImage(context.Background(), 7, 5); !errors.Is(previewErr, ErrImageNotManaged) {
		t.Fatalf("远程 URL 不应由受管预览读取: %v", previewErr)
	}
}

// 编译期确认图片存储替身覆盖卡券应用服务要求的最小 Port。
var _ ImageStore = (*imageStoreStub)(nil)
