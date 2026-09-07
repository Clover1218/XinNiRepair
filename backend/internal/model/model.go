// Package model 定义核心领域模型。
// 所有模型与数据库表结构一一对应, 使用 nullable types 处理数据库 NULL 值。
//
// 本版本按《数据库字段设计文档 V1.4》对齐:
//   - 状态机: draft → reported → pending_accept(待接单) → processing → completed,
//     completed 可 reopen 回 processing; 退回(非状态)回 draft
//   - 角色双层模型: users.role(0普通/1维修业务员/2超管) + memberships.role(0普通成员/1单位审核员)
//   - 项目字典库表化: project_categories / project_properties / project_problems
//   - repair_orders 项目列: category_id/property_id + 名称快照, audited_at/audited_by
package model

import (
	"time"

	"gorm.io/datatypes"
)

// ────────────────────────────────────────────
// 通用枚举常量
// ────────────────────────────────────────────

// PlatformRole 平台全局角色 (users.role)。
// 决定端与基础权限; 单位内的操作权限见 EnterpriseMemberRole。
const (
	PlatformRoleUser     = 0 // 普通用户（报修人，公众号/小程序端）
	PlatformRoleRepairer = 1 // 维修业务员（原"平台管理员"，店方人员；Web 后台 + 小程序：接单/处理/完工/退回/重新打开/收据与费用登记/个人汇总/创建与管理部门单位）
	PlatformRoleSuper    = 2 // 超级管理员（在维修业务员权限之上，另可：用户管理[查看/搜索/改昵称/改角色/改手机号/重置密码] + 项目字典管理）
)

// Deprecated: 旧名"平台管理员"。与 PlatformRoleRepairer 等价，保留以兼容历史代码引用。
const PlatformRolePlatformAdmin = PlatformRoleRepairer

// Deprecated: 旧名"超级管理员"，等价于 PlatformRoleSuper，保留兼容。
const PlatformRoleSuperAdmin = PlatformRoleSuper

// EnterpriseStatus 企业状态
type EnterpriseStatus int16

const (
	EnterpriseActive  EnterpriseStatus = 1
	EnterpriseDeleted EnterpriseStatus = 0
)

// EnterpriseMemberRole 企业（单位）内成员角色 (memberships.role)。
// 决定用户在本单位内的操作权限；与全局 PlatformRole 相互独立、可叠加。
const (
	EnterpriseRoleMember   = 0 // 普通成员（本单位报修人：提交/查看自己的工单）
	EnterpriseRoleReviewer = 1 // 单位审核员（原"企业管理员"，可多人：成员审批 + 本单位工单审核[reported→pending_accept/退回] + 本单位工单查看与汇总）
)

// Deprecated: 旧名"企业管理员"。与 EnterpriseRoleReviewer 等价，保留兼容。
const EnterpriseRoleAdmin = EnterpriseRoleReviewer

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
	OrderDraft         OrderStatus = "draft"          // 草稿
	OrderReported      OrderStatus = "reported"       // 已上报（等待本单位单位审核员审核）
	OrderPendingAccept OrderStatus = "pending_accept" // 待接单（单位审核员已审核通过，等待维修业务员接单）
	OrderProcessing    OrderStatus = "processing"     // 处理中
	OrderCompleted     OrderStatus = "completed"      // 已处理/完成（终态，可重新打开）
	OrderCancelled     OrderStatus = "cancelled"      // 已取消（终态，列表底部单独展示）
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
	ImageFault   ImageType = "fault"   // 故障图（报修时，≤9 张）
	ImageReceipt ImageType = "receipt" // 收据图（完工凭证，1-3 张）
)

// ImageStatus 图片状态 (temporary=刚上传未确认, active=草稿确认/正式, deleted=替换下架/软删除)
type ImageStatus string

const (
	ImageTemporary ImageStatus = "temporary"
	ImageActive    ImageStatus = "active"
	ImageDeleted   ImageStatus = "deleted"
)

// ActionType 操作类型 (order_timeline.action)
type ActionType string

const (
	ActionCreateDraft   ActionType = "create_draft"
	ActionSubmit        ActionType = "submit"
	ActionAudit         ActionType = "audit"    // 审核通过：reported → pending_accept（单位审核员）
	ActionAccept        ActionType = "accept"   // 接单维修：pending_accept → processing（维修业务员）
	ActionComplete      ActionType = "complete" // 完工：processing → completed
	ActionReopen        ActionType = "reopen"   // 重新打开：completed → processing
	ActionReject        ActionType = "reject"   // 退回：reported/pending_accept/processing → draft
	ActionCancel        ActionType = "cancel"   // 取消：draft/reported/pending_accept → cancelled
	ActionUploadReceipt ActionType = "upload_receipt"
	ActionUpdateFinance ActionType = "update_finance" // 修改对账信息 (5.6.1)
)

// NotifyChannel 通知渠道
type NotifyChannel string

const (
	NotifyWechatTpl NotifyChannel = "wechat_template" // 公众号模板消息 / 小程序订阅消息
	NotifyWebSocket NotifyChannel = "websocket"       // 管理端实时推送
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

// ValidTransitions 定义合法状态流转 (from → []to)。
// 退回不是独立状态：退回操作将状态置回 draft（见 OrderReject 业务规则）。
var ValidTransitions = map[OrderStatus][]OrderStatus{
	OrderDraft:         {OrderReported, OrderCancelled},                  // 提交 / 取消
	OrderReported:      {OrderPendingAccept, OrderDraft, OrderCancelled}, // 审核通过 / 退回 / 取消
	OrderPendingAccept: {OrderProcessing, OrderDraft, OrderCancelled},    // 接单 / 退回 / 取消
	OrderProcessing:    {OrderCompleted, OrderDraft},                     // 完工 / 退回
	OrderCompleted:     {OrderProcessing},                                // 重新打开（维修业务员）
	OrderCancelled:     {},                                               // 终态
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

// IsFinalStatus 判断是否为终态 (终态不可再流转, completed 除外可 reopen, 由调用方按动作判断)
func IsFinalStatus(s OrderStatus) bool {
	return s == OrderCompleted || s == OrderCancelled
}

// IsCancelableStatus 判断是否可取消 (报修人取消范围)
func IsCancelableStatus(s OrderStatus) bool {
	switch s {
	case OrderDraft, OrderReported, OrderPendingAccept:
		return true
	default:
		return false
	}
}

// ────────────────────────────────────────────
// 企业 / 用户 / 成员
// ────────────────────────────────────────────

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
	Password     string        `gorm:"type:varchar(100)"` // 登录密码, bcrypt 哈希; 维修业务员/超级管理员必填, 微信用户可为空
	AvatarUrl    string        `gorm:"type:varchar(512)"`
	Phone        string        `gorm:"type:varchar(20);index"`
	CreatedAt    time.Time     `gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `gorm:"autoUpdateTime"`
	Role         int           `gorm:"type:smallint;not null;default:0;index"` // 0=普通用户 1=维修业务员 2=超级管理员
	Memberships  []Membership  `gorm:"foreignKey:UserID"`
	RepairOrders []RepairOrder `gorm:"foreignKey:ReporterID"`
}

type Membership struct {
	ID           string     `gorm:"primaryKey;type:uuid"`
	EnterpriseID string     `gorm:"type:uuid;not null;index:idx_memberships_enterprise"`
	UserID       string     `gorm:"type:uuid;not null;index:idx_memberships_user"`
	Role         int        `gorm:"type:smallint;not null;default:0;index"` // 0=普通成员 1=单位审核员
	Status       string     `gorm:"type:varchar(20);not null;default:'pending'"`
	JoinedAt     *time.Time `gorm:"type:timestamptz"`
	RemovedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`

	Enterprise Enterprise `gorm:"foreignKey:EnterpriseID"`
	User       User       `gorm:"foreignKey:UserID"`
}

// ────────────────────────────────────────────
// 项目字典 (project_categories / project_properties / project_problems)
// 替代原 /data/project_categories.json; 软删除(deleted_at)+排序(sort_order)
// ────────────────────────────────────────────

// ProjectCategory 项目大类
type ProjectCategory struct {
	ID          string     `gorm:"primaryKey;type:uuid"`
	Name        string     `gorm:"type:varchar(50);not null"`
	Description string     `gorm:"type:varchar(200);not null;default:''"`
	SortOrder   int        `gorm:"type:smallint;not null;default:0"`
	DeletedAt   *time.Time `gorm:"type:timestamptz;index"` // NULL=有效; 软删除仅置此列, ID 与行保留
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`

	Properties []ProjectProperty `gorm:"foreignKey:CategoryID"`
}

// TableName 指定表名 (GORM 默认复数化 project_categories 一致, 显式声明防歧义)
func (ProjectCategory) TableName() string { return "project_categories" }

// ProjectProperty 项目属性 (隶属项目大类, 一对多; 属性下可挂常见问题)
type ProjectProperty struct {
	ID          string     `gorm:"primaryKey;type:uuid"`
	CategoryID  string     `gorm:"type:uuid;not null;index:idx_project_properties_category"`
	Name        string     `gorm:"type:varchar(50);not null"`
	Description string     `gorm:"type:varchar(200);not null;default:''"`
	SortOrder   int        `gorm:"type:smallint;not null;default:0"`
	DeletedAt   *time.Time `gorm:"type:timestamptz"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`

	Category ProjectCategory  `gorm:"foreignKey:CategoryID"`
	Problems []ProjectProblem `gorm:"foreignKey:PropertyID"`
}

// TableName 指定表名
func (ProjectProperty) TableName() string { return "project_properties" }

// ProjectProblem 常见问题 (隶属项目属性, 一对多; 仅用于快捷填充报修描述, 不落工单表)
// V1.4 修订: 由隶属大类改为直接隶属项目属性
type ProjectProblem struct {
	ID              string         `gorm:"primaryKey;type:uuid"`
	PropertyID      string         `gorm:"type:uuid;not null;index:idx_project_problems_property"`
	Name            string         `gorm:"type:varchar(100);not null"`
	Description     string         `gorm:"type:varchar(200);not null;default:''"`
	CommonSolutions datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'"` // ["更换屏幕","检查排线"]
	SortOrder       int            `gorm:"type:smallint;not null;default:0"`
	DeletedAt       *time.Time     `gorm:"type:timestamptz"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime"`

	Property ProjectProperty `gorm:"foreignKey:PropertyID"`
}

// TableName 指定表名
func (ProjectProblem) TableName() string { return "project_problems" }

// ────────────────────────────────────────────
// 报修工单
// ────────────────────────────────────────────

// RepairOrder 报修工单 (V1.4)
type RepairOrder struct {
	ID string `gorm:"primaryKey;type:uuid"`

	// ── 身份/归属 ──
	OrderNo      *string `gorm:"type:varchar(20);"`                     // 工单号 WO{YYYYMMDD}{4位序号}, 提交上报时生成; 草稿为空
	EnterpriseID *string `gorm:"type:uuid;index:idx_orders_enterprise"` // 所属单位; 草稿阶段可为空, 提交时必填
	ReporterID   string  `gorm:"type:uuid;not null;index:idx_orders_reporter"`

	// ── 项目大类/属性 (外键 + 名称快照; 替代旧 project_name/category/property 枚举) ──
	CategoryID   *string `gorm:"type:uuid;index:idx_orders_category"`  // → project_categories.id
	CategoryName string  `gorm:"type:varchar(50);not null;default:''"` // 大类名称快照 (字典改名/删除不影响历史单展示)
	PropertyID   *string `gorm:"type:uuid"`                            // → project_properties.id (须属于 category_id)
	PropertyName string  `gorm:"type:varchar(50);not null;default:''"` // 属性名称快照

	// ── 报修内容 ──
	Description  string `gorm:"type:varchar(500);not null"`                 // 报修描述 (常见问题预填于此, 1-500 字)
	Urgency      string `gorm:"type:varchar(20);not null;default:'normal'"` // normal/urgent/very_urgent
	Status       string `gorm:"type:varchar(20);not null;default:'draft'"`  // draft/reported/pending_accept/processing/completed/cancelled
	RejectReason string `gorm:"type:varchar(200)"`                          // 退回原因 (≥10字); 退回后保留供"已退回"派生展示
	Room         string `gorm:"type:varchar(20);not null;default:''"`       // 报修位置/房间号
	Contact      string `gorm:"type:varchar(40);not null;default:''"`       // 联系人及电话

	// ── 关键时间戳 / 责任人 ──
	SubmittedAt *time.Time `gorm:"type:timestamptz"`                    // 提交上报时间
	AuditedAt   *time.Time `gorm:"type:timestamptz"`                    // 单位审核员审核通过时间 (原 reviewed_at 更名)
	AuditedBy   *string    `gorm:"type:uuid;index:idx_orders_audited"`  // 审核人=单位审核员用户 ID
	AcceptedAt  *time.Time `gorm:"type:timestamptz"`                    // 接单时间
	RepairerID  *string    `gorm:"type:uuid;index:idx_orders_repairer"` // 维修业务员 (接单人, accept 时写入)
	CompletedAt *time.Time `gorm:"type:timestamptz"`                    // 完工时间 (reopen 后清空)
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`

	// ── 完工/对账 (V1.3 保留字段, 财务对账在范围内) ──
	RepairContent string         `gorm:"type:varchar(500)"`                                                        // 具体维修内容
	Quantity      int            `gorm:"type:integer;not null;default:1"`                                          // 数量
	UnitPrice     float64        `gorm:"type:decimal(10,2);not null;default:0"`                                    // 单价（元）
	Amount        float64        `gorm:"type:decimal(10,2) GENERATED ALWAYS AS (quantity * unit_price) STORED;->"` // 金额生成列, 只读
	Metadata      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`                                         // 维修结果/方式/保修期/时长/额外备注

	// ── 关联 ──
	Enterprise Enterprise      `gorm:"foreignKey:EnterpriseID"`
	Reporter   User            `gorm:"foreignKey:ReporterID"`
	Auditor    User            `gorm:"foreignKey:AuditedBy"`
	Repairer   User            `gorm:"foreignKey:RepairerID"`
	Category   ProjectCategory `gorm:"foreignKey:CategoryID"`
	Property   ProjectProperty `gorm:"foreignKey:PropertyID"`
	Images     []OrderImage    `gorm:"foreignKey:OrderID"`
	Timelines  []OrderTimeline `gorm:"foreignKey:OrderID"`
}

// RepairMetadata 维修附加元数据 (repair_orders.metadata JSONB)
type RepairMetadata struct {
	RepairResult   string `json:"repair_result"`   // 维修结果
	RepairMethod   string `json:"repair_method"`   // 维修方式
	WarrantyPeriod string `json:"warranty_period"` // 保修期
	ExtraRemark    string `json:"extra_remark"`    // 额外备注
	RepairDuration int    `json:"repair_duration"` // 维修时长（分钟）
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
