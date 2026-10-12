// Integration 测试：工单列表「待补签」筛选（V1.5）。
//
// 针对本地开发库运行：
//
//	RUN_DB_TESTS=1 go test ./internal/service/ -run PendingSign -count=1 -v
//
// 覆盖：上报→审核→接单→完工(稍后补签) 后，
//  1. 工单保持 processing + signature_status=pending_sign；
//  2. 不传 status 时 status_counts["pending_sign"] 反映真实数量；
//  3. status=pending_sign 能被正常放行（此前被 status 白名单误拦 400）并查询到该工单。
// 测试数据带 flow_ 前缀, 结束时清理。
package service

import (
	"os"
	"testing"

	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
)

func TestV15PendingSignFilter(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run db integration tests")
	}
	env := newV14Env(t)
	defer env.cleanup(t)

	reviewerOp := Operator{UserID: env.reviewerID, Role: model.PlatformRoleUser}
	repairerOp := Operator{UserID: env.repairerID, Role: model.PlatformRoleRepairer}
	superOp := Operator{UserID: env.repairerID, Role: model.PlatformRoleSuper}

	// 1) 造一条「待补签」工单：上报 → 审核 → 接单 → 完工(稍后补签)
	//    维修员经营范围已由 env.seed 授予
	order, _ := env.newOrderViaReport(t, "flow_待补签筛选测试")
	if err := env.adminSvc.Audit(env.ctx, reviewerOp, order.ID, "通过", "127.0.0.1"); err != nil {
		t.Fatalf("audit: %v", err)
	}
	if err := env.adminSvc.Accept(env.ctx, repairerOp, order.ID, "收到", "127.0.0.1"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	in := CompleteOrderInput{
		SignMode:      model.SignModePendingSign,
		Quantity:      1,
		UnitPrice:     100,
		RepairContent: "暂无法签字，稍后补签",
		Metadata:      model.RepairMetadata{RepairResult: "完全修复", RepairMethod: "上门维修"},
	}
	if err := env.adminSvc.Complete(env.ctx, repairerOp, order.ID, "127.0.0.1", in); err != nil {
		t.Fatalf("complete(pending_sign): %v", err)
	}
	got, err := env.orders.FindByID(env.ctx, order.ID)
	if err != nil || got == nil {
		t.Fatalf("reload order: %v", err)
	}
	if got.Status != string(model.OrderProcessing) || got.SignatureStatus != string(model.SignaturePendingSign) {
		t.Fatalf("完工(稍后补签) 后应为 processing/pending_sign, 实际 %s/%s", got.Status, got.SignatureStatus)
	}

	// 2) 不传 status：status_counts 应含 pending_sign = 1
	res, err := env.adminSvc.ListOrders(env.ctx, superOp, repository.OrderAdminFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if res.StatusCounts["pending_sign"] != 1 {
		t.Fatalf("status_counts[pending_sign] = %d, 期望 1 (counts=%v)",
			res.StatusCounts["pending_sign"], res.StatusCounts)
	}

	// 3) status=pending_sign 必须能查询到该工单（此前校验顺序错误会直接 400）
	res2, err := env.adminSvc.ListOrders(env.ctx, superOp,
		repository.OrderAdminFilter{Status: []string{"pending_sign"}}, 1, 20)
	if err != nil {
		t.Fatalf("status=pending_sign 筛选被拒绝(校验未放行): %v", err)
	}
	if res2.Total < 1 {
		t.Fatalf("status=pending_sign 应至少返回 1 条, 实际 total=%d", res2.Total)
	}
	found := false
	for _, o := range res2.List {
		if o.ID == order.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("status=pending_sign 结果未包含刚创建的待补签工单 %s", order.ID)
	}

	// 4) 非「待补签」的其它筛选组合应正常放行（不误伤）
	if _, err := env.adminSvc.ListOrders(env.ctx, superOp,
		repository.OrderAdminFilter{Status: []string{"processing", "completed"}}, 1, 20); err != nil {
		t.Fatalf("常规多状态筛选应放行: %v", err)
	}
	// 5) 真正非法的状态仍应被拒绝（校验顺序调整不能放走非法值）
	if _, err := env.adminSvc.ListOrders(env.ctx, superOp,
		repository.OrderAdminFilter{Status: []string{"not_a_status"}}, 1, 20); err == nil {
		t.Fatal("非法状态 not_a_status 应被拒绝, 实际通过了")
	}

	t.Logf("PASS: 待补签筛选链路正常, pending_sign 角标=%d, 筛选命中 total=%d",
		res.StatusCounts["pending_sign"], res2.Total)
}
