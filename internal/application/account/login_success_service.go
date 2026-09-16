package account

import "context"

// LoginStatusPort 定义登录成功后判断账号是否启用所需的最小状态查询能力。
type LoginStatusPort interface {
	// GetStatus 返回账号当前是否启用；该查询不读取或解密凭证内容。
	GetStatus(context.Context, string) bool
}

// LoginRestartPort 定义登录成功后重启账号运行时所需的最小能力。
type LoginRestartPort interface {
	// Restart 使用刚写入的持久化凭证重启账号运行实例。
	Restart(context.Context, string) error
}

// LoginSuccessService 编排登录成功后的关键运行时重启，不依赖 HTTP、Server 或非关键平台资料请求。
type LoginSuccessService struct {
	// statuses 提供账号启用状态查询能力。
	statuses LoginStatusPort
	// runtime 提供账号运行时重启能力。
	runtime LoginRestartPort
	// report 记录不含凭证的后续动作错误；为空时忽略诊断。
	report func(string, error)
}

// NewLoginSuccessService 构造登录成功后的关键运行时恢复服务；昵称和头像由独立资料刷新用例处理。
func NewLoginSuccessService(statuses LoginStatusPort, runtime LoginRestartPort, report func(string, error)) *LoginSuccessService {
	return &LoginSuccessService{statuses: statuses, runtime: runtime, report: report}
}

// AfterSuccessfulLogin 在凭证锁释放后立即按账号启用状态重启运行时；不得让可取消的资料请求阻塞恢复 WS。
func (s *LoginSuccessService) AfterSuccessfulLogin(ctx context.Context, userID int64, accountID string) {
	if s == nil {
		return
	}
	// userID 已在凭证持久化用例完成归属校验；这里保留参数以维持登录生命周期端口的身份语义。
	_ = userID
	if s.statuses != nil && s.statuses.GetStatus(ctx, accountID) && s.runtime != nil {
		// restartErr 保存登录成功后运行时重启错误。
		if restartErr := s.runtime.Restart(ctx, accountID); restartErr != nil {
			s.reportError("账号登录后重启账号失败", restartErr)
		}
	}
}

// reportError 统一传递登录成功后续动作的脱敏诊断。
func (s *LoginSuccessService) reportError(message string, err error) {
	if s.report != nil {
		s.report(message, err)
	}
}
