package adapter

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cardsapp "xianyu-go/internal/application/cards"
)

// cardImageStoreRootDefault 是没有显式桌面数据目录时使用的项目相对受管图片目录。
var cardImageStoreRootDefault = filepath.Join("data", "uploads", "card-images")

// CardImageStore 以用户目录和内容哈希保存不可变图片；它不删除旧资产，避免卡券更新影响仍在使用的引用。
type CardImageStore struct {
	// root 是所有用户卡密图片的绝对或工作目录相对根路径，首次上传时才创建。
	root string
}

// NewCardImageStore 创建受管图片文件适配器；root 为空时使用兼容本地开发的数据目录。
func NewCardImageStore(root string) *CardImageStore {
	root = strings.TrimSpace(root)
	if root == "" {
		root = cardImageStoreRootDefault
	}
	return &CardImageStore{root: filepath.Clean(root)}
}

// Save 校验真实图片媒体类型，以用户目录和 SHA-256 内容哈希不可变落盘，并返回内部引用。
func (store *CardImageStore) Save(ctx context.Context, userID int64, input cardsapp.ImageInput) (cardsapp.ImageAsset, error) {
	if store == nil || strings.TrimSpace(store.root) == "" {
		return cardsapp.ImageAsset{}, cardsapp.ErrImageStoreUnavailable
	}
	if userID <= 0 {
		return cardsapp.ImageAsset{}, cardsapp.ErrInvalidUser
	}
	// err 是上传请求在文件操作开始前已经取消的原因。
	if err := ctx.Err(); err != nil {
		return cardsapp.ImageAsset{}, err
	}
	// contentType、extension、validationErr 分别保存真实媒体类型、安全扩展名和图片内容校验错误。
	contentType, extension, validationErr := validateCardImageBytes(input.Data)
	if validationErr != nil {
		return cardsapp.ImageAsset{}, validationErr
	}
	// contentHash 是图片完整字节的 SHA-256，用于用户目录内去重且不包含原始文件名。
	contentHash := sha256.Sum256(input.Data)
	// storageName 是只由内容哈希和可信媒体类型组成的不可变文件名。
	storageName := fmt.Sprintf("%x%s", contentHash, extension)
	// userDirectory 是当前用户的隔离存储目录，权限不允许其他系统用户读取。
	userDirectory := filepath.Join(store.root, strconv.FormatInt(userID, 10))
	// mkdirErr 表示用户隔离目录无法按受限权限创建。
	if mkdirErr := os.MkdirAll(userDirectory, 0o700); mkdirErr != nil {
		return cardsapp.ImageAsset{}, fmt.Errorf("创建卡密图片目录失败: %w", mkdirErr)
	}
	// storagePath 是经过固定目录和受限文件名拼接的最终文件路径。
	storagePath := filepath.Join(userDirectory, storageName)
	// file、openErr 分别保存以 O_EXCL 创建的不可变文件和创建错误；已存在表示同内容已安全保存。
	file, openErr := os.OpenFile(storagePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if openErr == nil {
		// writeErr 表示完整图片字节未能写入新文件；失败时清理本次创建的残缺文件。
		writeErr := writeCardImageFile(file, input.Data)
		// closeErr 表示文件数据写入后关闭失败，仍需作为保存失败报告。
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(storagePath)
			return cardsapp.ImageAsset{}, errors.Join(writeErr, closeErr)
		}
	} else if !errors.Is(openErr, os.ErrExist) {
		return cardsapp.ImageAsset{}, fmt.Errorf("保存卡密图片失败: %w", openErr)
	}
	// reference 是与用户和不可变文件绑定的内部引用，格式异常属于基础设施实现错误。
	reference := cardsapp.BuildManagedImageReference(userID, storageName)
	if reference == "" {
		return cardsapp.ImageAsset{}, fmt.Errorf("生成卡密图片引用失败")
	}
	return cardsapp.ImageAsset{Reference: reference, Filename: storageName, ContentType: contentType}, nil
}

// ReadOwned 只读取 reference 中用户标识与 userID 一致的图片，供认证后的卡券预览使用。
func (store *CardImageStore) ReadOwned(ctx context.Context, userID int64, reference string) (cardsapp.ImageAsset, error) {
	// referenceUserID、storageName、ok 分别保存内部引用所属用户、受限文件名和格式有效性。
	referenceUserID, storageName, ok := cardsapp.ParseManagedImageReference(reference)
	if !ok || referenceUserID != userID {
		return cardsapp.ImageAsset{}, cardsapp.ErrForbidden
	}
	return store.read(ctx, referenceUserID, storageName, reference)
}

// ReadReference 读取已经通过卡券应用写入门禁的内部引用，供自动发货按实际账号上传平台。
func (store *CardImageStore) ReadReference(ctx context.Context, reference string) (cardsapp.ImageAsset, error) {
	// referenceUserID、storageName、ok 分别保存持久化内部引用中的用户、文件名和格式有效性。
	referenceUserID, storageName, ok := cardsapp.ParseManagedImageReference(reference)
	if !ok {
		return cardsapp.ImageAsset{}, cardsapp.ErrImageNotManaged
	}
	return store.read(ctx, referenceUserID, storageName, reference)
}

// read 从受限用户目录读取图片并再次校验大小和魔数，防止磁盘文件被替换后绕过上传门禁。
func (store *CardImageStore) read(ctx context.Context, userID int64, storageName, reference string) (cardsapp.ImageAsset, error) {
	if store == nil || strings.TrimSpace(store.root) == "" {
		return cardsapp.ImageAsset{}, cardsapp.ErrImageStoreUnavailable
	}
	// err 是预览或发货请求在磁盘读取前已经取消的原因。
	if err := ctx.Err(); err != nil {
		return cardsapp.ImageAsset{}, err
	}
	// userRoot、openRootErr 分别保存限制所有相对文件访问的用户目录句柄和打开错误。
	userRoot, openRootErr := os.OpenRoot(filepath.Join(store.root, strconv.FormatInt(userID, 10)))
	if openRootErr != nil {
		if errors.Is(openRootErr, os.ErrNotExist) {
			return cardsapp.ImageAsset{}, cardsapp.ErrNotFound
		}
		return cardsapp.ImageAsset{}, fmt.Errorf("打开卡密图片目录失败: %w", openRootErr)
	}
	defer userRoot.Close()
	// file、openErr 分别保存受 OpenRoot 限制的图片文件和读取错误。
	file, openErr := userRoot.Open(storageName)
	if openErr != nil {
		if errors.Is(openErr, os.ErrNotExist) {
			return cardsapp.ImageAsset{}, cardsapp.ErrNotFound
		}
		return cardsapp.ImageAsset{}, fmt.Errorf("打开卡密图片失败: %w", openErr)
	}
	defer file.Close()
	// data、readErr 分别保存受大小上限限制的图片字节和读取错误。
	data, readErr := io.ReadAll(io.LimitReader(file, cardsapp.MaxImageBytes+1))
	if readErr != nil {
		return cardsapp.ImageAsset{}, fmt.Errorf("读取卡密图片失败: %w", readErr)
	}
	// contentType、_, validationErr 分别保存真实媒体类型、未使用的扩展名和磁盘内容校验错误。
	contentType, _, validationErr := validateCardImageBytes(data)
	if validationErr != nil {
		return cardsapp.ImageAsset{}, validationErr
	}
	return cardsapp.ImageAsset{Reference: reference, Filename: storageName, ContentType: contentType, Data: data}, nil
}

// validateCardImageBytes 按真实魔数限制 PNG、JPEG 和 WebP，拒绝空文件、伪造 MIME 与超过 10 MiB 的内容。
func validateCardImageBytes(data []byte) (contentType, extension string, err error) {
	if len(data) == 0 || len(data) > cardsapp.MaxImageBytes {
		return "", "", &cardsapp.ValidationError{Message: "图片大小必须在 1 B 到 10 MiB 之间"}
	}
	// detectedType 是 Go 根据文件头检测的真实媒体类型，不信任浏览器声明。
	detectedType := http.DetectContentType(data)
	switch detectedType {
	case "image/png":
		return detectedType, ".png", nil
	case "image/jpeg":
		return detectedType, ".jpg", nil
	case "image/webp":
		return detectedType, ".webp", nil
	default:
		return "", "", &cardsapp.ValidationError{Message: "只支持 PNG、JPEG 或 WebP 图片"}
	}
}

// writeCardImageFile 完整写入并同步新文件；任何短写或同步失败都由调用方清理本次残缺文件。
func writeCardImageFile(file *os.File, data []byte) error {
	// writtenBytes、writeErr 分别保存写入字节数和底层文件错误。
	writtenBytes, writeErr := file.Write(data)
	if writeErr != nil {
		return fmt.Errorf("写入卡密图片失败: %w", writeErr)
	}
	if writtenBytes != len(data) {
		return io.ErrShortWrite
	}
	// syncErr 表示图片数据无法同步到持久存储，不能返回可用引用。
	if syncErr := file.Sync(); syncErr != nil {
		return fmt.Errorf("同步卡密图片失败: %w", syncErr)
	}
	return nil
}

// 编译期确认受管图片文件适配器满足卡券应用服务的最小存储 Port。
var _ cardsapp.ImageStore = (*CardImageStore)(nil)
