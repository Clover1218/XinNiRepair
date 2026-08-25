// OrderNotifier 工单订阅消息推送
//
// 对应《小程序订阅消息模板信息.md》三类模板:
//   - 工单状态变更 (reported/reviewed/processing)
//   - 工单退回 (reject)
//   - 工单完结 (completed)
//
// 通知失败仅记录日志, 不阻塞业务流程 (订阅消息授权配额耗尽属正常情况)。
package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

// 订阅消息模板 ID (见 docs/小程序订阅消息模板信息.md)
//
// 每个状态一个独立模板, 与 uni-app 端一次性授权的三个模板一一对应:
//   - 处理中 (processing): 工单处理提醒
//   - 退回 (reject): 工单状态提醒
//   - 完结 (completed): 报修工单完结通知
const (
	tplOrderProcessing = "GzsQVCeBG4ObOgoYuYkeZ4VZh711fmH9D3T9taI4TJE" // 工单处理提醒(处理中)
	tplOrderReject     = "3Gw9MOYxZN9sC8ka02RyrZK1y6guc3wE1H2wcWjNy0w" // 工单状态提醒(退回)
	tplOrderComplete   = "zj71qQ57GcxS6zzkqc2a4PI9ufJftolzmB-f0ed4f5I" // 报修工单完结通知
)

// OrderNotifier 订阅消息推送服务
type OrderNotifier struct {
	wechat *WechatService
	users  *repository.AuthRepository
	logger *zap.Logger
}

// NewOrderNotifier 创建 OrderNotifier
func NewOrderNotifier(wechat *WechatService, users *repository.AuthRepository, logger *zap.Logger) *OrderNotifier {
	return &OrderNotifier{wechat: wechat, users: users, logger: logger}
}

// NotifyOrderProcessing 工单处理中通知 (processing)
//
// 使用模板: 工单处理提醒 (tplOrderProcessing)
// 字段: thing1=项目名称 character_string2=工单编号 phrase17=工单状态 time3=开始时间
func (n *OrderNotifier) NotifyOrderProcessing(ctx context.Context, order *model.RepairOrder) {
	if order == nil {
		return
	}
	openid, ok := n.reporterOpenid(ctx, order.ReporterID)
	if !ok {
		return
	}
	data := map[string]SubscribeMessageData{
		"thing1":            {Value: truncateUTF8(order.ProjectName, 20)},
		"character_string2": {Value: orderNo(order)},
		"phrase17":          {Value: "处理中"},
		"time3":             {Value: time.Now().Format("2006-01-02 15:04:05")},
	}
	n.send(ctx, openid, tplOrderProcessing, data, order.ID)
}

// NotifyOrderReject 工单退回通知
//
// 使用模板: 工单状态提醒 (tplOrderReject)
// 字段: thing34=工单标题 character_string10=工单编号 phrase7=工单状态 thing17=退回原因 time16=操作时间
func (n *OrderNotifier) NotifyOrderReject(ctx context.Context, order *model.RepairOrder, reason string) {
	if order == nil {
		return
	}
	openid, ok := n.reporterOpenid(ctx, order.ReporterID)
	if !ok {
		return
	}
	data := map[string]SubscribeMessageData{
		"thing34":            {Value: truncateUTF8(order.ProjectName, 20)},
		"character_string10": {Value: orderNo(order)},
		"phrase7":            {Value: "已退回"},
		"thing17":            {Value: truncateUTF8(reason, 20)},
		"time16":             {Value: time.Now().Format("2006-01-02 15:04:05")},
	}
	n.send(ctx, openid, tplOrderReject, data, order.ID)
}

// NotifyOrderComplete 工单完结通知
//
// 使用模板: 报修工单完结通知 (tplOrderComplete)
// 字段: thing5=工单名称 number1=工单编号(纯数字) phrase4=处理结果 thing3=处理人 time2=维修时间
func (n *OrderNotifier) NotifyOrderComplete(ctx context.Context, order *model.RepairOrder, operatorID string) {
	if order == nil {
		return
	}
	openid, ok := n.reporterOpenid(ctx, order.ReporterID)
	if !ok {
		return
	}
	// 处理人昵称 (操作的管理员)
	operatorName := ""
	if u, err := n.users.FindUserByID(ctx, operatorID); err == nil && u != nil {
		operatorName = u.Nickname
	}
	completedAt := time.Now().Format("2006-01-02 15:04:05")
	if order.CompletedAt != nil {
		completedAt = order.CompletedAt.Format("2006-01-02 15:04:05")
	}
	data := map[string]SubscribeMessageData{
		"thing5":  {Value: truncateUTF8(order.ProjectName, 20)},
		"number1": {Value: digitsOnly(orderNo(order))},
		"phrase4": {Value: "已完成"},
		"thing3":  {Value: truncateUTF8(operatorName, 20)},
		"time2":   {Value: completedAt},
	}
	n.send(ctx, openid, tplOrderComplete, data, order.ID)
}

// reporterOpenid 查询报修人 openid, 失败返回 false (已记录日志)
func (n *OrderNotifier) reporterOpenid(ctx context.Context, reporterID string) (string, bool) {
	if reporterID == "" {
		return "", false
	}
	user, err := n.users.FindUserByID(ctx, reporterID)
	if err != nil {
		n.logger.Warn("notifier: find reporter failed", zap.String("reporter_id", reporterID), zap.Error(err))
		return "", false
	}
	if user == nil || user.Openid == "" {
		return "", false
	}
	return user.Openid, true
}

// send 调用微信接口发送, 通知失败仅记录日志
func (n *OrderNotifier) send(ctx context.Context, openid, templateID string, data map[string]SubscribeMessageData, orderID string) {
	page := "pages/order/detail?id=" + orderID
	if err := n.wechat.SendSubscribeMessage(ctx, openid, templateID, data, page); err != nil {
		n.logger.Warn("notifier: send subscribe message failed",
			zap.String("order_id", orderID), zap.String("template_id", templateID), zap.Error(err))
	}
}

// orderNo 安全取工单号, 为空时回退占位符
func orderNo(order *model.RepairOrder) string {
	if order.OrderNo != nil && *order.OrderNo != "" {
		return *order.OrderNo
	}
	return "未生成"
}

// truncateUTF8 按 rune 截断到 maxLen, 防止超出微信字段长度限制
func truncateUTF8(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	rs := []rune(s)
	return string(rs[:maxLen])
}

// digitsOnly 提取数字字符 (number 类型字段仅允许纯数字)
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "0"
	}
	return out
}
