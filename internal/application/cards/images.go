package cards

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	// ManagedImageScheme 是数据库中受管图片引用使用的内部协议，不允许浏览器或公共 HTTP 客户端直接访问。
	ManagedImageScheme = "card-image"
	// MaxImageBytes 是单张图片卡密允许保存和发货的最大字节数，与 HTTP 请求体限制共同防止内存和磁盘滥用。
	MaxImageBytes = 10 << 20
)

var (
	// ErrImageStoreUnavailable 表示运行时没有装配受管图片文件存储，远程 URL 卡密不受影响。
	ErrImageStoreUnavailable = errors.New("卡密图片存储未启用")
	// ErrImageNotManaged 表示目标卡密使用远程 URL，不能通过受管图片预览端点读取。
	ErrImageNotManaged = errors.New("卡密不是受管图片")
	// managedImageNamePattern 限定内容哈希文件名和受支持扩展名，拒绝目录分隔符与任意客户端路径。
	managedImageNamePattern = regexp.MustCompile(`^[a-f0-9]{64}\.(?:png|jpg|webp)$`)
)

// ImageInput 是用户选择后随卡券表单提交的单张图片；Data 只在当前请求和存储写入期间保留。
type ImageInput struct {
	// Filename 是浏览器提供的展示文件名，基础设施不得把其中的目录信息用于落盘路径。
	Filename string
	// ContentType 是浏览器声明的媒体类型；服务端存储必须以 Data 的真实魔数重新判定。
	ContentType string
	// Data 是受 MaxImageBytes 限制的图片字节，不得写入日志或数据库。
	Data []byte
}

// ImageAsset 是受管图片存储返回的稳定引用或已通过所有权校验的图片内容。
type ImageAsset struct {
	// Reference 是保存到 cards.image_url 的内部引用，只能由 ImageStore 生成。
	Reference string
	// Filename 是平台上传时使用的安全文件名，不包含客户端目录。
	Filename string
	// ContentType 是根据真实文件内容确认的图片媒体类型。
	ContentType string
	// Data 是预览或自动发货所需的图片字节，调用方不得持久化到日志。
	Data []byte
}

// ImageStore 定义卡券应用服务保存图片和按本地用户读取图片所需的最小基础设施能力。
type ImageStore interface {
	// Save 为 userID 保存不可变图片内容并返回稳定内部引用；重复内容可以安全复用同一文件。
	Save(ctx context.Context, userID int64, input ImageInput) (ImageAsset, error)
	// ReadOwned 仅在引用归属于 userID 时读取图片，用于认证后的预览和所有权校验。
	ReadOwned(ctx context.Context, userID int64, reference string) (ImageAsset, error)
}

// BuildManagedImageReference 使用用户标识和受限存储文件名构造内部引用；非法输入返回空字符串。
func BuildManagedImageReference(userID int64, storageName string) string {
	storageName = strings.TrimSpace(storageName)
	if userID <= 0 || !managedImageNamePattern.MatchString(storageName) {
		return ""
	}
	return fmt.Sprintf("%s://%d/%s", ManagedImageScheme, userID, storageName)
}

// ParseManagedImageReference 解析内部引用并返回所属用户和存储文件名；远程 URL 或畸形路径返回 ok=false。
func ParseManagedImageReference(reference string) (userID int64, storageName string, ok bool) {
	// parsed、parseErr 分别保存内部引用的 URL 结构和解析失败原因。
	parsed, parseErr := url.Parse(strings.TrimSpace(reference))
	if parseErr != nil || parsed.Scheme != ManagedImageScheme || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return 0, "", false
	}
	// parsedUserID、userErr 分别保存 host 中的本地用户标识和数字解析错误。
	parsedUserID, userErr := strconv.ParseInt(parsed.Host, 10, 64)
	// parsedName 是去除单个前导斜杠后的内容哈希文件名。
	parsedName := strings.TrimPrefix(parsed.EscapedPath(), "/")
	// unescapedName、unescapeErr 分别保存 URL 解码后的文件名和非法转义错误。
	unescapedName, unescapeErr := url.PathUnescape(parsedName)
	if userErr != nil || parsedUserID <= 0 || unescapeErr != nil || unescapedName != path.Base(unescapedName) || !managedImageNamePattern.MatchString(unescapedName) {
		return 0, "", false
	}
	return parsedUserID, unescapedName, true
}

// IsManagedImageReference 判断图片字段是否为结构完整的受管引用；它不替代用户所有权检查。
func IsManagedImageReference(reference string) bool {
	// _, _, ok 丢弃解析结果，只保留引用格式有效性。
	_, _, ok := ParseManagedImageReference(reference)
	return ok
}

// CreateWithImage 保存用户上传的图片并创建 image 类型卡券；文件保存成功而数据库失败时保留不可变内容供后续去重复用。
func (s *Service) CreateWithImage(ctx context.Context, userID int64, draft Draft, image *ImageInput) (int64, error) {
	if image == nil {
		return s.Create(ctx, userID, draft)
	}
	// err 表示用户身份、仓储或图片存储依赖未满足创建条件。
	if err := s.validateImageOperation(userID); err != nil {
		return 0, err
	}
	if draft.Type != "image" {
		return 0, &ValidationError{Message: "只有图片卡密可以上传图片文件"}
	}
	// validationErr 表示图片落盘前必须满足的名称、类型和延迟等基础字段约束。
	if validationErr := validateDraftBase(draft); validationErr != nil {
		return 0, validationErr
	}
	// asset、saveErr 分别保存不可变图片引用和文件系统写入错误。
	asset, saveErr := s.imageStore.Save(ctx, userID, *image)
	if saveErr != nil {
		return 0, saveErr
	}
	draft.ImageURL = asset.Reference
	// validationErr 表示普通字段或刚生成的受管引用未满足业务约束。
	if validationErr := validateDraftWithManagedReference(draft, asset.Reference); validationErr != nil {
		return 0, validationErr
	}
	return s.repository.Create(ctx, cardFromDraft(0, userID, draft))
}

// UpdateWithImage 保存用户选择的新图片并更新指定卡券；未提供 image 时保留现有远程 URL 或受管引用兼容行为。
func (s *Service) UpdateWithImage(ctx context.Context, userID, cardID int64, draft Draft, image *ImageInput) error {
	if image == nil {
		return s.Update(ctx, userID, cardID, draft)
	}
	// err 表示用户身份、仓储或图片存储依赖未满足更新条件。
	if err := s.validateImageOperation(userID); err != nil {
		return err
	}
	if draft.Type != "image" {
		return &ValidationError{Message: "只有图片卡密可以上传图片文件"}
	}
	// validationErr 表示新图片落盘前必须满足的名称、类型和延迟等基础字段约束。
	if validationErr := validateDraftBase(draft); validationErr != nil {
		return validationErr
	}
	// existing、ownershipErr 分别保存当前卡券和缺失或越权错误，必须先于文件落盘完成校验。
	existing, ownershipErr := s.ownedCard(ctx, userID, cardID)
	if ownershipErr != nil {
		return ownershipErr
	}
	// asset、saveErr 分别保存新图片的不可变引用和文件系统写入错误。
	asset, saveErr := s.imageStore.Save(ctx, userID, *image)
	if saveErr != nil {
		return saveErr
	}
	draft.ImageURL = asset.Reference
	// validationErr 表示更新字段或新受管引用未满足业务约束。
	if validationErr := validateDraftWithManagedReference(draft, asset.Reference); validationErr != nil {
		return validationErr
	}
	return s.repository.Update(ctx, cardFromDraft(existing.ID, existing.UserID, draft))
}

// GetImage 读取当前用户指定 image 卡券的受管图片内容；远程 URL 继续由浏览器直接预览。
func (s *Service) GetImage(ctx context.Context, userID, cardID int64) (ImageAsset, error) {
	// err 表示用户身份、仓储或图片存储依赖未满足读取条件。
	if err := s.validateImageOperation(userID); err != nil {
		return ImageAsset{}, err
	}
	// card、ownershipErr 分别保存已通过所有权校验的卡券和读取错误。
	card, ownershipErr := s.ownedCard(ctx, userID, cardID)
	if ownershipErr != nil {
		return ImageAsset{}, ownershipErr
	}
	if card.Type != "image" || !IsManagedImageReference(card.ImageURL) {
		return ImageAsset{}, ErrImageNotManaged
	}
	return s.imageStore.ReadOwned(ctx, userID, card.ImageURL)
}

// validateImageOperation 检查卡券仓储、用户身份和图片存储是否可用，避免请求期出现空指针或静默降级。
func (s *Service) validateImageOperation(userID int64) error {
	// err 表示基础卡券服务或用户身份不满足操作条件。
	if err := s.validateUser(userID); err != nil {
		return err
	}
	if s.imageStore == nil {
		return ErrImageStoreUnavailable
	}
	return nil
}
