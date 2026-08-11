// Package errors 定义分层业务错误码与错误类型。
// 错误码分段:
//
//	0          成功
//	1000-1999  认证错误 (未登录/Token过期)
//	2000-2999  权限错误 (无权限操作)
//	3000-3999  参数校验错误
//	4000-4999  资源不存在
//	5000-5999  服务端内部错误
//	6000-6999  第三方服务错误 (微信API/OSS)
package errors

import (
	"fmt"
)

// BizError 业务错误
type BizError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	err     error  // 底层原始错误
}

func (e *BizError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap
func (e *BizError) Unwrap() error {
	return e.err
}

// WithError 附加底层错误 (用于链式传递)
func (e *BizError) WithError(err error) *BizError {
	return &BizError{Code: e.Code, Message: e.Message, err: err}
}

// WithMessage 覆盖错误消息
func (e *BizError) WithMessage(msg string) *BizError {
	return &BizError{Code: e.Code, Message: msg, err: e.err}
}

// Is 实现 errors.Is 判定, 按 Code 比较
func (e *BizError) Is(target error) bool {
	t, ok := target.(*BizError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ────────────────────────────────────────────
// 预定义错误
// ────────────────────────────────────────────

// 认证错误 (1000-1999)
var (
	ErrUnauthorized     = &BizError{Code: 1000, Message: "未登录或登录已过期"}
	ErrTokenInvalid     = &BizError{Code: 1001, Message: "Token无效"}
	ErrTokenExpired     = &BizError{Code: 1002, Message: "Token已过期"}
	ErrWechatAuthFailed   = &BizError{Code: 1010, Message: "微信授权失败"}
	ErrInvalidCredentials = &BizError{Code: 1011, Message: "账号或密码错误"}
)

// 权限错误 (2000-2999)
var (
	ErrForbidden       = &BizError{Code: 2000, Message: "无权限操作"}
	ErrNotAdmin        = &BizError{Code: 2001, Message: "仅管理员可执行此操作"}
	ErrWrongEnterprise = &BizError{Code: 2002, Message: "无权操作该企业数据"}
)

// 参数校验 (3000-3999)
var (
	ErrInvalidParam = &BizError{Code: 3000, Message: "参数校验失败"}
	ErrInvalidPage  = &BizError{Code: 3001, Message: "分页参数无效"}
	ErrImageInvalid = &BizError{Code: 3010, Message: "图片格式不支持或大小超限"}
	ErrImageTooMany = &BizError{Code: 3011, Message: "图片数量超出限制"}
)

// 资源不存在 (4000-4999)
var (
	ErrNotFound           = &BizError{Code: 4000, Message: "资源不存在"}
	ErrOrderNotFound      = &BizError{Code: 4001, Message: "工单不存在"}
	ErrUserNotFound       = &BizError{Code: 4002, Message: "用户不存在"}
	ErrEnterpriseNotFound = &BizError{Code: 4003, Message: "企业不存在"}
	ErrMemberNotFound     = &BizError{Code: 4004, Message: "成员不存在"}
)

// 业务规则冲突 (4500-4599)
var (
	ErrOrderCannotEdit      = &BizError{Code: 4500, Message: "工单当前状态不可修改"}
	ErrInvalidTransition    = &BizError{Code: 4501, Message: "状态流转不合法"}
	ErrDraftNotSubmittable  = &BizError{Code: 4502, Message: "草稿必填项未完成，无法提交"}
	ErrRejectReasonTooShort = &BizError{Code: 4503, Message: "退回原因不少于10字"}
	ErrAlreadyProcessed     = &BizError{Code: 4504, Message: "该操作已处理，请勿重复操作"}
	ErrMemberStatusInvalid  = &BizError{Code: 4505, Message: "成员状态不允许该操作"}
	ErrPhoneAlreadyBound    = &BizError{Code: 4510, Message: "手机号已绑定，不可修改"}
	ErrAlreadyJoined        = &BizError{Code: 4520, Message: "已在该企业中，请勿重复加入"}
	ErrInviteCodeInvalid    = &BizError{Code: 4521, Message: "邀请码无效"}
	ErrEnterpriseNameExists = &BizError{Code: 4522, Message: "企业名称已存在"}
	ErrInviteCodeExpired    = &BizError{Code: 4523, Message: "邀请码已过期"}
	ErrDraftLimit           = &BizError{Code: 4524, Message: "草稿数量已达上限（最多5个）"}
	ErrNoExportData         = &BizError{Code: 4507, Message: "当前筛选条件下没有可导出的数据"}
)

// 服务端错误 (5000-5999)
var (
	ErrInternal      = &BizError{Code: 5000, Message: "服务器内部错误"}
	ErrDatabaseError = &BizError{Code: 5001, Message: "数据库操作失败"}
	ErrGenerateCode  = &BizError{Code: 5002, Message: "生成邀请码失败"}
)

// 第三方服务 (6000-6999)
var (
	ErrWechatAPI    = &BizError{Code: 6000, Message: "微信接口调用失败"}
	ErrOSSUpload    = &BizError{Code: 6001, Message: "文件上传失败"}
	ErrExportFailed = &BizError{Code: 6002, Message: "导出失败"}
)
