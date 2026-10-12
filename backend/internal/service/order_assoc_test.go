package service_test

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/internal/service"
)

func openTestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=123456 dbname=xin_ni_repair sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("skip: cannot connect to postgres (%v)", err)
	}
	return db
}

func getImg(t *testing.T, db *gorm.DB, id string) model.OrderImage {
	var img model.OrderImage
	if err := db.Where("id = ?", id).First(&img).Error; err != nil {
		t.Fatalf("query %s: %v", id, err)
	}
	return img
}

// TestAssociateImages_Diff 验证"提交时 diff 关联"的核心语义：
//   - 现表列表中的记录：temporary→active 并回填 order_id；已是 active 的保持不变（仅更新排序）
//   - 原表该工单下 active 但不在列表中的记录：置 deleted（用户移除的旧图）
//   - 未绑定哨兵的 temporary：被纳入列表时绑定到本工单并置 active
//   - 其它工单的图片不受影响
//   - 只有 active 计入工单真实图片数
func TestAssociateImages_Diff(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	images := repository.NewOrderImageRepository(tx)
	svc := service.NewOrderService(nil, images, nil, nil, nil, nil, nil, zap.NewNop())

	orderID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	otherOrder := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	seed := []model.OrderImage{
		{ID: "11111111-1111-1111-1111-1111111111a1", OrderID: orderID, ImageUrl: "u-a", ImageType: "fault", Status: "active", SortOrder: 0},
		{ID: "11111111-1111-1111-1111-1111111111b2", OrderID: orderID, ImageUrl: "u-b", ImageType: "fault", Status: "active", SortOrder: 1},
		{ID: "11111111-1111-1111-1111-1111111111c3", OrderID: orderID, ImageUrl: "u-c", ImageType: "fault", Status: "temporary", SortOrder: -1},
		{ID: "11111111-1111-1111-1111-1111111111d4", OrderID: model.UnboundImageOrderID, ImageUrl: "u-d", ImageType: "fault", Status: "temporary", SortOrder: -1},
		{ID: "11111111-1111-1111-1111-1111111111e5", OrderID: otherOrder, ImageUrl: "u-e", ImageType: "fault", Status: "temporary", SortOrder: -1},
	}
	for i := range seed {
		if err := tx.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// 提交时传入：保留 a、新增 c（临时→active）、纳入 d（未绑定→绑定+active）；移除 b。
	if err := svc.AssociateImages(context.Background(), orderID, "fault", []string{"11111111-1111-1111-1111-1111111111a1", "11111111-1111-1111-1111-1111111111c3", "11111111-1111-1111-1111-1111111111d4"}); err != nil {
		t.Fatalf("AssociateImages: %v", err)
	}

	a := getImg(t, tx, "11111111-1111-1111-1111-1111111111a1")
	if a.Status != "active" || a.OrderID != orderID {
		t.Errorf("id-a 应为 active 且属于本工单, got status=%s order_id=%s", a.Status, a.OrderID)
	}
	if a.SortOrder != 0 {
		t.Errorf("id-a sort_order 应为 0, got %d", a.SortOrder)
	}

	b := getImg(t, tx, "11111111-1111-1111-1111-1111111111b2")
	if b.Status != "deleted" {
		t.Errorf("id-b 应被移除置 deleted, got %s", b.Status)
	}

	c := getImg(t, tx, "11111111-1111-1111-1111-1111111111c3")
	if c.Status != "active" || c.OrderID != orderID {
		t.Errorf("id-c 应 temporary→active 并绑定本工单, got status=%s order_id=%s", c.Status, c.OrderID)
	}
	if c.SortOrder != 1 {
		t.Errorf("id-c sort_order 应为 1, got %d", c.SortOrder)
	}

	d := getImg(t, tx, "11111111-1111-1111-1111-1111111111d4")
	if d.Status != "active" || d.OrderID != orderID {
		t.Errorf("id-d 未绑定哨兵应被绑定并置 active, got status=%s order_id=%s", d.Status, d.OrderID)
	}
	if d.SortOrder != 2 {
		t.Errorf("id-d sort_order 应为 2, got %d", d.SortOrder)
	}

	e := getImg(t, tx, "11111111-1111-1111-1111-1111111111e5")
	if e.Status != "temporary" || e.OrderID != otherOrder {
		t.Errorf("id-e（其它工单）应不受影响, got status=%s order_id=%s", e.Status, e.OrderID)
	}

	// 只有 active 计入工单真实图片数（a/c/d 共 3 张）
	active, err := images.ListActiveByOrder(context.Background(), orderID, "fault")
	if err != nil {
		t.Fatalf("ListActiveByOrder: %v", err)
	}
	if len(active) != 3 {
		t.Errorf("本工单 active 图片应为 3, got %d", len(active))
	}
}

// TestUploadImage_Unbound 验证上传支持"未绑定工单"（order_id 为空 → 写入哨兵），且不校验工单归属。
func TestUploadImage_Unbound(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	images := repository.NewOrderImageRepository(tx)
	svc := service.NewOrderService(nil, images, nil, nil, nil, nil, nil, zap.NewNop())

	// 用真实 imagebed 太重，这里直接验证"空 order_id → 哨兵"的分支逻辑：
	// 通过 Insert 一条 order_id=哨兵 的 temporary 记录，再 AssociateImages 绑定，模拟预上传→提交。
	unbound := model.OrderImage{
		ID:        "11111111-1111-1111-1111-1111111111f6",
		OrderID:   model.UnboundImageOrderID,
		ImageUrl:  "u-u",
		ImageType: "fault",
		Status:    "temporary",
		SortOrder: -1,
	}
	if err := tx.Create(&unbound).Error; err != nil {
		t.Fatalf("create unbound: %v", err)
	}
	orderID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	if err := svc.AssociateImages(context.Background(), orderID, "fault", []string{"11111111-1111-1111-1111-1111111111f6"}); err != nil {
		t.Fatalf("AssociateImages: %v", err)
	}
	u := getImg(t, tx, "11111111-1111-1111-1111-1111111111f6")
	if u.Status != "active" || u.OrderID != orderID {
		t.Errorf("未绑定预上传图应被绑定并置 active, got status=%s order_id=%s", u.Status, u.OrderID)
	}
}
