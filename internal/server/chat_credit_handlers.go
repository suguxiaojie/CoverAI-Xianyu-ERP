package server

import (
	"errors"
	"net/http"
	"strings"

	chatapp "xianyu-go/internal/application/chat"
	"xianyu-go/internal/auth"
)

// getChatUserCredit 只为当前已打开会话按需查询一个买家的公开信用。
func (s *Server) getChatUserCredit(writer http.ResponseWriter, request *http.Request) {
	// session 是已经通过认证中间件的 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// accountID 和 buyerID 是完成空白清理的店铺账号与当前会话买家标识。
	accountID, buyerID := strings.TrimSpace(request.URL.Query().Get("account_id")), strings.TrimSpace(request.URL.Query().Get("buyer_id"))
	if accountID == "" || buyerID == "" {
		writeErr(writer, http.StatusBadRequest, "缺少账号或买家标识")
		return
	}
	// profile、creditErr 是应用层完成归属、缓存、限速和平台查询后的结果。
	profile, creditErr := s.chatApplication().GetUserCredit(request.Context(), session.UserID, accountID, buyerID)
	if creditErr != nil {
		switch {
		case errors.Is(creditErr, chatapp.ErrInvalidInput):
			writeErr(writer, http.StatusBadRequest, "信用查询参数无效")
		case errors.Is(creditErr, chatapp.ErrSessionForbidden):
			writeErr(writer, http.StatusForbidden, "无权访问该账号")
		case errors.Is(creditErr, chatapp.ErrCreditCoolingDown):
			writeErrDetails(writer, http.StatusTooManyRequests, "chat_credit_cooling_down", "信用信息暂缓更新", "", nil)
		case errors.Is(creditErr, chatapp.ErrCreditUnavailable), errors.Is(creditErr, chatapp.ErrSessionUnavailable):
			writeErr(writer, http.StatusServiceUnavailable, "信用查询服务未启用")
		default:
			writeErrDetails(writer, http.StatusBadGateway, "chat_credit_fetch_failed", "信用信息暂未获取", "", nil)
		}
		return
	}
	writeJSON(writer, http.StatusOK, newChatCreditProfileDTO(profile))
}
