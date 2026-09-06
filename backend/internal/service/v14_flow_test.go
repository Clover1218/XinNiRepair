// Integration 测试 (V1.4 核心链路)。
//
// 针对本地开发库运行 (需先执行 migrations/008_v1_4_state_dict.sql):
//
//	RUN_DB_TESTS=1 go test ./internal/service/ -run V14 -count=1 -v
//
// 覆盖: 项目字典 options / 草稿→上报(WO 工单号)→审核(audit)→接单(accept, 写 repairer_id)
// →完工(complete, 金额生成列)→重新打开(reopen)→退回/取消; 单位审核员权限判定;
// 字典 CRUD 与唯一性。测试数据带 flow_ 前缀, 结束时清理。
package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"xin-ni-repair/internal/config"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/pkg/imagebed"
)

// v14TestEnv 承载被测服务与测试数据 ID
type v14TestEnv struct {
	ctx context.Context
	db  *gorm.DB

	entID, reporterID, reviewerID, repairerID, catID, propID string

	orders    *repository.OrderRepository
	images    *repository.OrderImageRepository
	timelines *repository.OrderTimelineRepository
	mems      *repository.MembershipRepository
	projects  *repository.ProjectRepository
	users     *repository.AuthRepository
	ents      *repository.EnterpriseRepository

	orderSvc  *OrderService
	adminSvc  *AdminOrderService
	accessSvc *AccessService
	entSvc    *EnterpriseService
	projSvc   *ProjectService
}

func newV14Env(t *testing.T) *v14TestEnv {
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Skipf("config load failed, skip: %v", err)
	}
	dbWrap, err := repository.New(context.Background(), cfg.Database)
	if err != nil {
		t.Skipf("db unavailable, skip: %v", err)
	}
	db := dbWrap.DB

	env := &v14TestEnv{
		ctx:       context.Background(),
		db:        db,
		orders:    repository.NewOrderRepository(db),
		images:    repository.NewOrderImageRepository(db),
		timelines: repository.NewOrderTimelineRepository(db),
		mems:      repository.NewMembershipRepository(db),
		projects:  repository.NewProjectRepository(db),
		users:     repository.NewAuthRepository(db),
		ents:      repository.NewEnterpriseRepository(db),
	}
	env.accessSvc = NewAccessService(env.mems)
	env.entSvc = NewEnterpriseService(env.ents, env.mems, zapNop())
	img := imagebed.New(imagebed.Config{Endpoint: "http://127.0.0.1:9/none"})
	env.orderSvc = NewOrderService(env.orders, env.images, env.timelines, env.mems, env.projects, img, nil, zapNop())
	env.adminSvc = NewAdminOrderService(env.orders, env.images, env.timelines, env.accessSvc, img, nil, zapNop())
	env.projSvc = NewProjectService(env.projects, zapNop())

	env.seed(t)
	return env
}

// seed 准备基础数据 (企业/三种用户/成员关系 + 项目大类引用)
func (e *v14TestEnv) seed(t *testing.T) {
	now := time.Now()
	// 借用迁移种子中的大类/属性, 保证后续引用有效
	cat, err := e.projects.FindActiveCategoryByID(e.ctx, "10000000-0000-4000-8000-000000000001")
	if err != nil || cat == nil {
		t.Fatalf("seed category missing (run migration 008 first): %v", err)
	}
	e.catID = cat.ID
	props, err := e.projects.ListProperties(e.ctx, cat.ID)
	if err != nil || len(props) == 0 {
		t.Fatalf("seed property missing: %v", err)
	}
	e.propID = props[0].ID

	ent := &model.Enterprise{ID: "ffffffff-0000-4000-8000-000000000001", Name: fmt.Sprintf("flow_企业_%d", now.UnixNano()), InviteCode: "FLOW01"}
	if err := e.ents.Create(e.ctx, ent); err != nil {
		t.Fatalf("create enterprise: %v", err)
	}
	e.entID = ent.ID

	users := []*model.User{
		{ID: "ffffffff-0000-4000-8000-000000000101", Openid: "flow_reporter", Nickname: "flow_报修人"},
		{ID: "ffffffff-0000-4000-8000-000000000102", Openid: "flow_reviewer", Nickname: "flow_审核员", Role: model.PlatformRoleUser},
		{ID: "ffffffff-0000-4000-8000-000000000103", Openid: "flow_repairer", Nickname: "flow_维修员", Role: model.PlatformRoleRepairer},
	}
	for _, u := range users {
		if err := e.users.CreateUser(e.ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	e.reporterID = users[0].ID
	e.reviewerID = users[1].ID
	e.repairerID = users[2].ID

	memberships := []*model.Membership{
		{ID: "ffffffff-0000-4000-8000-000000000201", EnterpriseID: e.entID, UserID: e.reporterID, Role: model.EnterpriseRoleMember, Status: string(model.MemberApproved), JoinedAt: &now},
		{ID: "ffffffff-0000-4000-8000-000000000202", EnterpriseID: e.entID, UserID: e.reviewerID, Role: model.EnterpriseRoleReviewer, Status: string(model.MemberApproved), JoinedAt: &now},
	}
	for _, m := range memberships {
		if err := e.mems.Create(e.ctx, m); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}
}

// cleanup 清理本次测试数据 (按外键顺序)
func (e *v14TestEnv) cleanup(t *testing.T) {
	db := e.db
	// 用户创建的 flow_ 字典 (先删子表再删大类)
	_ = db.Exec(`DELETE FROM project_problems WHERE category_id IN (SELECT id FROM project_categories WHERE name LIKE 'flow_%')`).Error
	_ = db.Exec(`DELETE FROM project_properties WHERE category_id IN (SELECT id FROM project_categories WHERE name LIKE 'flow_%')`).Error
	_ = db.Exec(`DELETE FROM project_categories WHERE name LIKE 'flow_%'`).Error
	// 工单链路
	_ = db.Exec(`DELETE FROM order_timeline WHERE order_id IN (SELECT id FROM repair_orders WHERE reporter_id = ?)`, e.reporterID).Error
	_ = db.Exec(`DELETE FROM order_images WHERE order_id IN (SELECT id FROM repair_orders WHERE reporter_id = ?)`, e.reporterID).Error
	_ = db.Exec(`DELETE FROM repair_orders WHERE reporter_id = ?`, e.reporterID).Error
	_ = db.Exec(`DELETE FROM memberships WHERE enterprise_id = ?`, e.entID).Error
	_ = db.Exec(`DELETE FROM enterprises WHERE id = ?`, e.entID).Error
	_ = db.Exec(`DELETE FROM users WHERE id IN (?,?,?)`, e.reporterID, e.reviewerID, e.repairerID).Error
}

func (e *v14TestEnv) newOrderViaReport(t *testing.T, description string) (*model.RepairOrder, string) {
	draft, err := e.orderSvc.Create(e.ctx, e.reporterID)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	desc := description
	if desc == "" {
		desc = "flow_测试工单: 台式机无法开机"
	}
	_, err = e.orderSvc.Update(e.ctx, e.reporterID, draft.OrderID, UpdateOrderInput{
		EnterpriseID: strPtr2(e.entID),
		CategoryID:   strPtr2(e.catID),
		PropertyID:   strPtr2(e.propID),
		Description:  &desc,
		Urgency:      strPtr2(string(model.UrgencyUrgent)),
		Room:         strPtr2("101"),
		Contact:      strPtr2("张三 13800000000"),
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	res, err := e.orderSvc.Submit(e.ctx, e.reporterID, draft.OrderID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if res.Status != string(model.OrderReported) {
		t.Fatalf("submit status = %s, want reported", res.Status)
	}
	if res.OrderNo == nil || !strings.HasPrefix(*res.OrderNo, "WO") {
		t.Fatalf("order_no should be WO-prefixed, got %v", res.OrderNo)
	}
	order, err := e.orders.FindByID(e.ctx, draft.OrderID)
	if err != nil || order == nil {
		t.Fatalf("load submitted order: %v", err)
	}
	// 快照校验
	if order.CategoryName == "" || order.PropertyName == "" || order.Description != desc {
		t.Fatalf("snapshot fields wrong: cat=%q prop=%q desc=%q", order.CategoryName, order.PropertyName, order.Description)
	}
	return order, desc
}

func TestV14OrderLifecycleAndAccess(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run db integration tests")
	}
	env := newV14Env(t)
	defer env.cleanup(t)

	reporterOp := Operator{UserID: env.reporterID, Role: model.PlatformRoleUser}
	reviewerOp := Operator{UserID: env.reviewerID, Role: model.PlatformRoleUser}
	repairerOp := Operator{UserID: env.repairerID, Role: model.PlatformRoleRepairer}

	// 1) 普通成员无权执行单位管理
	if err := env.accessSvc.CanManageEnterprise(env.ctx, env.entID, env.reporterID, 0); err == nil {
		t.Fatal("普通成员不应能管理单位")
	}
	// 2) 单位审核员可管理
	if err := env.accessSvc.CanManageEnterprise(env.ctx, env.entID, env.reviewerID, 0); err != nil {
		t.Fatalf("单位审核员应能管理本单位: %v", err)
	}

	// 3) options 字典树 (单位审核员也是成员, 可见其单位)
	opts, err := env.orderSvc.Options(env.ctx, env.reviewerID)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if len(opts.Categories) == 0 || len(opts.Enterprises) == 0 {
		t.Fatal("options 应包含字典与单位")
	}

	// 4) 生命周期: 上报→审核→接单→完工
	order, _ := env.newOrderViaReport(t, "")
	if err := env.adminSvc.Audit(env.ctx, reviewerOp, order.ID, "通过", "127.0.0.1"); err != nil {
		t.Fatalf("reviewer audit: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.Status != string(model.OrderPendingAccept) || order.AuditedBy == nil || *order.AuditedBy != env.reviewerID {
		t.Fatalf("after audit: status=%s audited_by=%v", order.Status, nstr(order.AuditedBy))
	}

	if err := env.adminSvc.Accept(env.ctx, repairerOp, order.ID, "收到", "127.0.0.1"); err != nil {
		t.Fatalf("repairer accept: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.Status != string(model.OrderProcessing) || order.RepairerID == nil || *order.RepairerID != env.repairerID {
		t.Fatalf("after accept: status=%s repairer=%v", order.Status, nstr(order.RepairerID))
	}

	in := CompleteOrderInput{
		Remark:        "已更换电源, 测试通过",
		Receipts:      []string{"https://img.local/receipt1.jpg"},
		Quantity:      1,
		UnitPrice:     150.00,
		RepairContent: "更换台式机电源模块",
		Metadata:      model.RepairMetadata{RepairResult: "完全修复", RepairMethod: "上门维修", WarrantyPeriod: "3个月", RepairDuration: 30},
	}
	if err := env.adminSvc.Complete(env.ctx, repairerOp, order.ID, "127.0.0.1", in); err != nil {
		t.Fatalf("complete: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.Status != string(model.OrderCompleted) || order.CompletedAt == nil {
		t.Fatalf("after complete: status=%s", order.Status)
	}
	if order.Amount != 150.00 {
		t.Fatalf("amount should be 150.00 (generated), got %v", order.Amount)
	}

	// 5) 重新打开 → 再次完工
	if err := env.adminSvc.Reopen(env.ctx, repairerOp, order.ID, "客户复检有问题", "127.0.0.1"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.Status != string(model.OrderProcessing) || order.CompletedAt != nil {
		t.Fatalf("after reopen: status=%s completed_at=%v", order.Status, order.CompletedAt)
	}
	if err := env.adminSvc.Complete(env.ctx, repairerOp, order.ID, "127.0.0.1", in); err != nil {
		t.Fatalf("re-complete: %v", err)
	}

	// 6) 修改对账信息
	np := 160.00
	if err := env.adminSvc.UpdateFinance(env.ctx, repairerOp, order.ID, "127.0.0.1", UpdateFinanceInput{UnitPrice: &np}); err != nil {
		t.Fatalf("update finance: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.UnitPrice != 160.00 || order.Amount != 160.00 {
		t.Fatalf("finance not updated: unit=%v amount=%v", order.UnitPrice, order.Amount)
	}

	// 7) 越权: 普通成员(报修人)不能接单/审核其他流程以外动作
	if err := env.adminSvc.Accept(env.ctx, reporterOp, order.ID, "", "127.0.0.1"); err == nil {
		t.Fatal("普通用户接单应被拒绝")
	}
}

func TestV14RejectAndCancel(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run db integration tests")
	}
	env := newV14Env(t)
	defer env.cleanup(t)

	reviewerOp := Operator{UserID: env.reviewerID, Role: model.PlatformRoleUser}
	reporterOp := Operator{UserID: env.reporterID, Role: model.PlatformRoleUser}

	// 上报后由单位审核员退回 (需 ≥10 字原因)
	order, _ := env.newOrderViaReport(t, "")
	if err := env.adminSvc.Reject(env.ctx, reviewerOp, order.ID, "报修描述不够清晰，请补充具体的故障现象后再提交", "127.0.0.1"); err != nil {
		t.Fatalf("reviewer reject: %v", err)
	}
	order, _ = env.orders.FindByID(env.ctx, order.ID)
	if order.Status != string(model.OrderDraft) || order.RejectReason == "" {
		t.Fatalf("after reject: status=%s reason=%q", order.Status, order.RejectReason)
	}
	if order.SubmittedAt != nil {
		t.Fatalf("after reject submitted_at 应被清空 (order_no 保留用于历史追溯)")
	}

	// 原因过短
	order2, _ := env.newOrderViaReport(t, "")
	if err := env.adminSvc.Reject(env.ctx, reviewerOp, order2.ID, "太短", "127.0.0.1"); err == nil {
		t.Fatal("过短原因应被拒绝")
	}

	// 取消 (仅 draft/reported/pending_accept; 上报后可取消)
	order3, _ := env.newOrderViaReport(t, "")
	if err := env.orderSvc.Cancel(env.ctx, env.reporterID, order3.ID, "问题自行解决"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	order3, _ = env.orders.FindByID(env.ctx, order3.ID)
	if order3.Status != string(model.OrderCancelled) {
		t.Fatalf("after cancel: status=%s", order3.Status)
	}

	// 普通用户不能以店方动作处理 (例如 accept)
	if err := env.adminSvc.Accept(env.ctx, reporterOp, order.ID, "", "127.0.0.1"); err == nil {
		t.Fatal("普通成员不应能接单")
	}
}

func TestV14DictionaryCRUD(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run db integration tests")
	}
	env := newV14Env(t)
	defer env.cleanup(t)

	name := fmt.Sprintf("flow_大类_%d", time.Now().UnixNano())
	cat, err := env.projSvc.CreateCategory(env.ctx, CategoryInput{Name: strPtr2(name), Description: strPtr2("测试")})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	// 重名冲突
	if _, err := env.projSvc.CreateCategory(env.ctx, CategoryInput{Name: strPtr2(name)}); err == nil {
		t.Fatal("重名大类应报错")
	}
	prop, err := env.projSvc.CreateProperty(env.ctx, PropertyInput{CategoryID: strPtr2(cat.ID), Name: strPtr2("测试属性")})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	prob, err := env.projSvc.CreateProblem(env.ctx, ProblemInput{CategoryID: strPtr2(cat.ID), Name: strPtr2("测试问题"), CommonSolutions: []string{"方案A", "方案B"}})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	// 软删除后不再出现, 但仍可按 id 找回
	if err := env.projSvc.DeleteCategory(env.ctx, cat.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	list, err := env.projSvc.ListCategories(env.ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range list {
		if c.ID == cat.ID {
			t.Fatal("软删除大类不应出现在列表")
		}
	}
	// 大类软删除后, 其下属性应不可再用 (级联软删除)
	p2, err := env.projects.FindActivePropertyByID(env.ctx, prop.ID)
	if err != nil {
		t.Fatalf("find property after delete: %v", err)
	}
	if p2 != nil {
		t.Fatal("大类软删除后其属性应不可见")
	}
	_ = prob
}

func TestV14ReviewerWebAccessAndMemberRole(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run db integration tests")
	}
	env := newV14Env(t)
	defer env.cleanup(t)

	// 1) 提升: 普通成员 → 单位审核员; 幂等; 非法值报错; 降级恢复
	if err := env.accessSvc.CanManageEnterprise(env.ctx, env.entID, env.reporterID, 0); err == nil {
		t.Fatal("提升前 reporter 不应具备单位管理权限")
	}
	if err := env.entSvc.SetMemberRole(env.ctx, env.entID, env.reporterID, "reviewer"); err != nil {
		t.Fatalf("promote reviewer: %v", err)
	}
	if err := env.accessSvc.CanManageEnterprise(env.ctx, env.entID, env.reporterID, 0); err != nil {
		t.Fatalf("提升后 reporter 应具备单位管理权限: %v", err)
	}
	if err := env.entSvc.SetMemberRole(env.ctx, env.entID, env.reporterID, "reviewer"); err != nil {
		t.Fatalf("重复提升应幂等: %v", err)
	}
	if err := env.entSvc.SetMemberRole(env.ctx, env.entID, env.reporterID, "boss"); err == nil {
		t.Fatal("非法 role 应报错")
	}
	if err := env.entSvc.SetMemberRole(env.ctx, env.entID, env.reporterID, "member"); err != nil {
		t.Fatalf("demote member: %v", err)
	}
	if err := env.accessSvc.CanManageEnterprise(env.ctx, env.entID, env.reporterID, 0); err == nil {
		t.Fatal("降级后 reporter 不应再具备单位管理权限")
	}

	// 2) 最后一名单位审核员不可降级 (seed 中仅 reviewer 一名审核员)
	if err := env.entSvc.SetMemberRole(env.ctx, env.entID, env.reviewerID, "member"); err == nil {
		t.Fatal("最后一名单位审核员应禁止降级")
	}

	// 3) 成员列表 role 筛选 reviewer
	list, _, err := env.mems.ListByEnterprise(env.ctx, env.entID, "", "reviewer", "", 0, 50)
	if err != nil {
		t.Fatalf("list reviewer members: %v", err)
	}
	if len(list) != 1 || list[0].UserID != env.reviewerID {
		t.Fatalf("应仅返回一名审核员: %+v", list)
	}

	// 4) 审核员账号设置密码后可登录 Web
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("review123"), bcrypt.DefaultCost)
	reviewer, err := env.users.FindUserByID(env.ctx, env.reviewerID)
	if err != nil || reviewer == nil {
		t.Fatalf("load reviewer: %v", err)
	}
	reviewer.Password = string(hash)
	if err := env.users.UpdateUser(env.ctx, reviewer); err != nil {
		t.Fatalf("set reviewer password: %v", err)
	}

	tokenSvc := NewTokenService(cfg.JWT)
	authSvc := NewAuthService(env.users, env.mems, tokenSvc, nil, zapNop())
	login, err := authSvc.AdminLogin(env.ctx, "flow_审核员", "review123")
	if err != nil {
		t.Fatalf("审核员应可登录 Web 后台: %v", err)
	}
	if login.User.Role != model.PlatformRoleUser {
		t.Fatalf("审核员登录后 role 应为 0: %v", login.User.Role)
	}
	if login.AccessToken == "" {
		t.Fatal("应签发 token")
	}

	// 5) 非审核员普通成员即使有密码也不能登录
	hash2, _ := bcrypt.GenerateFromPassword([]byte("mem12345"), bcrypt.DefaultCost)
	mem, err := env.users.FindUserByID(env.ctx, env.reporterID)
	if err != nil || mem == nil {
		t.Fatalf("load reporter: %v", err)
	}
	mem.Password = string(hash2)
	if err := env.users.UpdateUser(env.ctx, mem); err != nil {
		t.Fatalf("set reporter password: %v", err)
	}
	if _, err := authSvc.AdminLogin(env.ctx, "flow_报修人", "mem12345"); err == nil {
		t.Fatal("普通成员不应能登录管理后台")
	}

	// 6) 超管重置密码: role=0 单位审核员放行, 纯微信成员不放行
	userAdmin := NewUserAdminService(env.users, env.mems, zapNop())
	if err := userAdmin.ResetPassword(env.ctx, env.reporterID, "abc123456"); err == nil {
		t.Fatal("纯微信成员不应允许重置密码")
	}
	if err := userAdmin.ResetPassword(env.ctx, env.reviewerID, "abc123456"); err != nil {
		t.Fatalf("单位审核员应允许设置密码: %v", err)
	}
}

// strPtr2 便捷 string 指针
func strPtr2(s string) *string { return &s }

// zapNop 空日志器 (测试用)
func zapNop() *zap.Logger { return zap.NewNop() }
