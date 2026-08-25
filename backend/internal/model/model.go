// Package model 定义核心领域模型。
// 所有模型与数据库表结构一一对应, 使用 nullable types 处理数据库 NULL 值。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// ────────────────────────────────────────────
// 通用枚举常量
// ────────────────────────────────────────────

// User 成员角色
const (
	PlatformRoleUser          = 0 // 普通用户
	PlatformRolePlatformAdmin = 1 // 平台管理员（店主）
	PlatformRoleSuperAdmin    = 2 // 超级管理员
)

// EnterpriseStatus 企业状态
type EnterpriseStatus int16

const (
	EnterpriseActive  EnterpriseStatus = 1
	EnterpriseDeleted EnterpriseStatus = 0
)

// EnterpriseMemberRole 企业内成员角色 (0-普通成员 1-企业管理员)
const (
	EnterpriseRoleMember = 0
	EnterpriseRoleAdmin  = 1
)

// MemberStatus 成员状态
type MemberStatus string

const (
	MemberPending  MemberStatus = "pending"
	MemberApproved MemberStatus = "approved"
	MemberRejected MemberStatus = "rejected"
	MemberRemoved  MemberStatus = "removed"
)

// OrderStatus 工单状态
type OrderStatus string

const (
	OrderDraft      OrderStatus = "draft"      // 草稿
	OrderReported   OrderStatus = "reported"   // 已上报
	OrderReviewed   OrderStatus = "reviewed"   // 已阅
	OrderProcessing OrderStatus = "processing" // 处理中
	OrderCompleted  OrderStatus = "completed"  // 已处理 (终态)
	OrderCancelled  OrderStatus = "cancelled"  // 已取消 (终态)
)

// Urgency 紧急程度
type Urgency string

const (
	UrgencyNormal     Urgency = "normal"
	UrgencyUrgent     Urgency = "urgent"
	UrgencyVeryUrgent Urgency = "very_urgent"
)

// ImageType 图片类型
type ImageType string

const (
	ImageFault   ImageType = "fault"
	ImageReceipt ImageType = "receipt"
)

// ImageStatus 图片状态 (temporary=刚上传未确认, active=草稿确认, deleted=软删除)
type ImageStatus string

const (
	ImageTemporary ImageStatus = "temporary"
	ImageActive    ImageStatus = "active"
	ImageDeleted   ImageStatus = "deleted"
)

// ActionType 操作类型
type ActionType string

const (
	ActionCreateDraft   ActionType = "create_draft"
	ActionSubmit        ActionType = "submit"
	ActionReview        ActionType = "review"
	ActionAccept        ActionType = "accept"
	ActionComplete      ActionType = "complete"
	ActionReject        ActionType = "reject"
	ActionUploadReceipt ActionType = "upload_receipt"
	ActionUpdateFinance ActionType = "update_finance" // 5.6.1 修改对账信息
	ActionCancel        ActionType = "cancel"
)

// NotifyChannel 通知渠道
type NotifyChannel string

const (
	NotifyWechatTpl NotifyChannel = "wechat_template"
	NotifyWebSocket NotifyChannel = "websocket"
)

// NotifySendStatus 通知发送状态
type NotifySendStatus int16

const (
	NotifyPending NotifySendStatus = 1 // 待发送
	NotifySent    NotifySendStatus = 2 // 已发送
	NotifyFailed  NotifySendStatus = 3 // 发送失败
)

// ────────────────────────────────────────────
// 有效状态转换表
// ────────────────────────────────────────────

// ValidTransitions 定义合法状态流转 (from → []to)
var ValidTransitions = map[OrderStatus][]OrderStatus{
	OrderDraft:      {OrderReported, OrderCancelled},
	OrderReported:   {OrderReviewed, OrderDraft},   // 查阅 / 退回
	OrderReviewed:   {OrderProcessing, OrderDraft}, // 接单 / 退回
	OrderProcessing: {OrderCompleted, OrderDraft},  // 完工 / 退回
	OrderCompleted:  {},                            // 终态
	OrderCancelled:  {},                            // 终态
}

// AllowedTransition 检查状态流转是否合法
func AllowedTransition(from, to OrderStatus) bool {
	if from == "" {
		from = OrderDraft
	}
	targets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// IsFinalStatus 判断是否为终态
func IsFinalStatus(s OrderStatus) bool {
	return s == OrderCompleted || s == OrderCancelled
}

type Enterprise struct {
	ID                  string        `gorm:"primaryKey;type:uuid"`
	Name                string        `gorm:"type:varchar(50);not null;uniqueIndex"`
	InviteCode          string        `gorm:"type:varchar(8);not null;uniqueIndex"`
	AutoApprove         bool          `gorm:"not null;default:false"`
	Status              int           `gorm:"not null;default:1;index"`
	CreatedAt           time.Time     `gorm:"autoCreateTime"`
	UpdatedAt           time.Time     `gorm:"autoUpdateTime"`
	InviteCodeExpiresAt *time.Time    `gorm:"type:timestamptz"`
	Memberships         []Membership  `gorm:"foreignKey:EnterpriseID"`
	RepairOrders        []RepairOrder `gorm:"foreignKey:EnterpriseID"`
}

type User struct {
	ID           string        `gorm:"primaryKey;type:uuid"`
	Openid       string        `gorm:"type:varchar(64);not null;uniqueIndex"`
	Unionid      string        `gorm:"type:varchar(64);index"`
	Nickname     string        `gorm:"type:varchar(32);not null"`
	Password     string        `gorm:"type:varchar(100)"` // 登录密码, bcrypt 哈希; 平台管理员必填, 微信用户可为空
	AvatarUrl    string        `gorm:"type:varchar(512)"`
	Phone        string        `gorm:"type:varchar(20);index"`
	CreatedAt    time.Time     `gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `gorm:"autoUpdateTime"`
	Role         int           `gorm:"type:smallint;not null;default:0;index"`
	Memberships  []Membership  `gorm:"foreignKey:UserID"`
	RepairOrders []RepairOrder `gorm:"foreignKey:ReporterID"`
}

type Membership struct {
	ID           string     `gorm:"primaryKey;type:uuid"`
	EnterpriseID string     `gorm:"type:uuid;not null;index:idx_memberships_enterprise"`
	UserID       string     `gorm:"type:uuid;not null;index:idx_memberships_user"`
	Role         int        `gorm:"type:smallint;not null;default:0;index"`
	Status       string     `gorm:"type:varchar(20);not null;default:'pending'"`
	JoinedAt     *time.Time `gorm:"type:timestamptz"`
	RemovedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`

	Enterprise Enterprise `gorm:"foreignKey:EnterpriseID"`
	User       User       `gorm:"foreignKey:UserID"`
}

// RepairOrder 报修工单
type RepairOrder struct {
	ID           string     `gorm:"primaryKey;type:uuid"`
	OrderNo      *string    `gorm:"type:varchar(20);"`                     // 提交上报时生成, 草稿为空
	EnterpriseID *string    `gorm:"type:uuid;index:idx_orders_enterprise"` // 空草稿时为 NULL, 4.3 设置
	ReporterID   string     `gorm:"type:uuid;not null;index:idx_orders_reporter"`
	ProjectName  string     `gorm:"type:varchar(20);not null"`
	Description  string     `gorm:"type:varchar(500);not null"`
	Urgency      string     `gorm:"type:varchar(20);not null;default:'normal'"`
	Status       string     `gorm:"type:varchar(20);not null;default:'draft'"`
	Category     string     `gorm:"type:varchar(20);not null;default:'other'"`
	Property     string     `gorm:"type:varchar(20);not null;default:'repair'"`
	Room         string     `gorm:"type:varchar(20);not null;default:''"`
	Contact      string     `gorm:"type:varchar(40);not null;default:''"`
	RejectReason string     `gorm:"type:varchar(200)"`
	SubmittedAt  *time.Time `gorm:"type:timestamptz"`
	ReviewedAt   *time.Time `gorm:"type:timestamptz"`
	AcceptedAt   *time.Time `gorm:"type:timestamptz"`
	CompletedAt  *time.Time `gorm:"type:timestamptz"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`

	RepairContent string         `gorm:"type:varchar(500)"`
	Quantity      int            `gorm:"type:integer;not null;default:1"`
	UnitPrice     float64        `gorm:"type:decimal(10,2);not null;default:0"`
	Amount        float64        `gorm:"type:decimal(10,2) GENERATED ALWAYS AS (quantity * unit_price) STORED;->"` // 只读生成列: 数据库自动计算 amount=quantity*unit_price, GORM 不写入
	Metadata      datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	RepairerID    *string        `gorm:"type:uuid;index:idx_orders_repairer"` // 维修员(业务员), accept 时写入
	Repairer      User           `gorm:"foreignKey:RepairerID"`

	Enterprise Enterprise      `gorm:"foreignKey:EnterpriseID"`
	Reporter   User            `gorm:"foreignKey:ReporterID"`
	Images     []OrderImage    `gorm:"foreignKey:OrderID"`
	Timelines  []OrderTimeline `gorm:"foreignKey:OrderID"`
}

// OrderImage 工单图片
type OrderImage struct {
	ID        string    `gorm:"primaryKey;type:uuid"`
	OrderID   string    `gorm:"type:uuid;not null;index:idx_order_images_order"`
	ImageUrl  string    `gorm:"type:varchar(512);not null"`
	ImageType string    `gorm:"type:varchar(20);not null;default:'fault'"`
	SortOrder int       `gorm:"type:smallint;not null;default:0"`
	FileSize  int       `gorm:"type:integer"`
	Status    string    `gorm:"type:varchar(20);not null;default:'temporary'"`
	CreatedAt time.Time `gorm:"autoCreateTime"`

	Order RepairOrder `gorm:"foreignKey:OrderID"`
}

// RepairMetadata 维修附加元数据 (repair_orders.metadata JSONB)
type RepairMetadata struct {
	RepairResult   string `json:"repair_result"`   // 维修结果
	RepairMethod   string `json:"repair_method"`   // 维修方式
	WarrantyPeriod string `json:"warranty_period"` // 保修期
	ExtraRemark    string `json:"extra_remark"`    // 额外备注
	RepairDuration int    `json:"repair_duration"` // 维修时长（分钟）
}

// OrderTimeline 工单操作时间轴
type OrderTimeline struct {
	ID         string    `gorm:"primaryKey;type:uuid"`
	OrderID    string    `gorm:"type:uuid;not null;index:idx_timeline_order"`
	OrderNo    *string   `gorm:"type:varchar(20)"` // 冗余工单号, 提交上报时生成
	OperatorID string    `gorm:"type:uuid;not null;index:idx_timeline_operator"`
	Action     string    `gorm:"type:varchar(30);not null"`
	FromStatus string    `gorm:"type:varchar(20)"`
	ToStatus   string    `gorm:"type:varchar(20)"`
	Remark     string    `gorm:"type:varchar(500)"`
	IpAddress  string    `gorm:"type:varchar(45)"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`

	Order    RepairOrder `gorm:"foreignKey:OrderID"`
	Operator User        `gorm:"foreignKey:OperatorID"`
}

// TableName 指定表名 (数据库表为单数 order_timeline, GORM 默认复数化会拼成 order_timelines)
func (OrderTimeline) TableName() string {
	return "order_timeline"
}
