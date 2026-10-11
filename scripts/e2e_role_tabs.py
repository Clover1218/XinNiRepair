"""验收：角色化 Tab / 首页待办所需的后端筛选能力（真实登录 + 真实数据 + 真实调用）

以数据库计数为基准验证：
  1. 各状态单查是否准确
  2. 「处理中」Tab 对维修业务员只显示自己接的单（既有业务规则，回归哨兵）
  3. 「未完成」多状态聚合 status=reported,pending_accept,processing
     = 库中 待审核+待接单+处理中 之和，且排除 cancelled / completed
  4. 首页各待办区块能否用 page_size=5 取到预览 + total 计数
夹具：psycopg2 直接插入指定状态的真实工单，跑完按 ID 精确删除。
"""
import sys
import uuid
from datetime import datetime, timedelta

import psycopg2
import requests

BASE = "http://localhost:8080/api/v1"
NICKNAME = "测试用户1"   # 维修业务员（role=1，非超管）
PASSWORD = "admin123"

DB = dict(host="localhost", port=5432, user="postgres", password="123456", dbname="xin_ni_repair")

ENTERPRISE_ID = "224f5f16-d97e-4c53-9dcc-d609d3af2b24"   # Cloverstia
REPORTER_ID = "9d9f0cf7-cb49-439e-8859-4dd067fd90b9"     # 测试用户1（本次登录的维修业务员）
OTHER_REPAIRER = "1c5505bb-9944-4ba7-885a-ae6473432899"  # 另一位维修业务员，用作回归哨兵
CATEGORY_ID = "967677bf-66d4-42c4-ad9f-92bcb7f506f3"
PROPERTY_ID = "891713ce-6f02-4a37-b754-4808f94591e3"
CATEGORY_NAME = "电脑"
PROPERTY_NAME = "维修"

results: list[tuple[str, bool, str]] = []
fixture_ids: list[str] = []


def check(name: str, ok: bool, detail: str = "") -> None:
    results.append((name, ok, detail))
    print(f"{'PASS' if ok else 'FAIL'} | {name} | {detail}")


def login() -> str:
    r = requests.post(f"{BASE}/auth/admin-login", json={"nickname": NICKNAME, "password": PASSWORD}, timeout=10)
    if r.status_code != 200:
        print("登录失败:", r.status_code, r.text[:300])
        sys.exit(1)
    data = r.json().get("data") or {}
    token = data.get("access_token") or data.get("token")
    if not token:
        print("登录响应缺 token:", r.json())
        sys.exit(1)
    print(f"登录成功: {NICKNAME} role={data.get('user', {}).get('role')}")
    return token


def count(token: str, status: str) -> int:
    params = {"page": 1, "page_size": 1}
    if status:
        params["status"] = status
    r = requests.get(f"{BASE}/admin/orders", params=params,
                     headers={"Authorization": f"Bearer {token}"}, timeout=10)
    body = r.json()
    if r.status_code != 200 or body.get("code") not in (0, None):
        print(f"  请求异常 status={status!r} HTTP={r.status_code} 响应={r.text[:300]}")
        sys.exit(1)
    return (body.get("data") or {}).get("total", 0)


def prepared(status: str, ts: datetime, repairer: str | None = None) -> dict:
    return dict(
        id=str(uuid.uuid4()), order_no=None, enterprise_id=ENTERPRISE_ID, reporter_id=REPORTER_ID,
        category_id=CATEGORY_ID, category_name=CATEGORY_NAME, property_id=PROPERTY_ID, property_name=PROPERTY_NAME,
        description=f"E2E角色Tab-{status}", urgency="normal", status=status, reject_reason=None,
        room="E2E测试地点", contact="张三 13800000000",
        submitted_at=ts, audited_at=None, audited_by=None,
        accepted_at=ts if status in ("processing", "completed") else None,
        repairer_id=repairer, completed_at=ts if status == "completed" else None,
        created_at=ts, updated_at=ts, repair_content="", quantity=1, unit_price=0,
        metadata='{"repair_result":"","repair_method":"","warranty_period":"","repair_duration":0,"extra_remark":""}',
    )


COLS = ["id", "order_no", "enterprise_id", "reporter_id", "category_id", "category_name",
        "property_id", "property_name", "description", "urgency", "status", "reject_reason",
        "room", "contact", "submitted_at", "audited_at", "audited_by", "accepted_at",
        "repairer_id", "completed_at", "created_at", "updated_at",
        "repair_content", "quantity", "unit_price", "metadata"]
UUID_COLS = {"id", "enterprise_id", "reporter_id", "category_id", "property_id"}


def insert_fixtures() -> None:
    """插入夹具：覆盖各 Tab；特意加一条「别人接的处理中」与一条「已完成」做回归哨兵"""
    now = datetime.now()
    rows = [
        prepared("reported", now - timedelta(minutes=40)),
        prepared("reported", now - timedelta(minutes=30)),
        prepared("pending_accept", now - timedelta(minutes=20)),
        prepared("processing", now - timedelta(minutes=10), repairer=REPORTER_ID),   # 自己接
        prepared("processing", now - timedelta(minutes=8), repairer=OTHER_REPAIRER),  # 别人接
        prepared("cancelled", now - timedelta(minutes=5)),
        prepared("completed", now - timedelta(minutes=3), repairer=REPORTER_ID),
    ]
    fixture_ids.extend(r["id"] for r in rows)
    cols = ", ".join(COLS)
    placeholders = ", ".join(("%s::uuid" if c in UUID_COLS else "%s") for c in COLS)
    with psycopg2.connect(**DB) as conn:
        with conn.cursor() as cur:
            for r in rows:
                cur.execute(f"INSERT INTO repair_orders ({cols}) VALUES ({placeholders})", [r[c] for c in COLS])
    print(f"已插入 {len(rows)} 条夹具工单")


def db_counts() -> dict[str, int]:
    """夹具层面的真实分布，作为断言基准"""
    with psycopg2.connect(**DB) as conn:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT status, COUNT(*) FROM repair_orders WHERE id = ANY(%s::uuid[]) GROUP BY status",
                (fixture_ids,),
            )
            rows = cur.fetchall()
    return {s: n for s, n in rows}


def cleanup_fixtures() -> None:
    if not fixture_ids:
        return
    with psycopg2.connect(**DB) as conn:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM repair_orders WHERE id = ANY(%s::uuid[])", (fixture_ids,))
    print(f"已清理 {len(fixture_ids)} 条夹具工单")


def main() -> None:
    token = login()
    insert_fixtures()
    try:
        base = db_counts()
        print("夹具状态分布:", base)

        reported, pending, processing = (base.get(s, 0) for s in ("reported", "pending_accept", "processing"))
        unfinished_expected = reported + pending + processing

        # ── 1. 单状态 Tab 与库一致 ──
        for status, label in (("reported", "待审核"), ("pending_accept", "待接单")):
            got = count(token, status)
            check(f"单状态 Tab 准确 {label}", got == base.get(status, 0), f"接口={got} 库={base.get(status, 0)}")

        # ── 2. 回归：「处理中」只对业务员展示自己接的单 ──
        got_processing = count(token, "processing")
        check("处理中 Tab 仅自己的单", got_processing < processing,
              f"接口={got_processing} 库中处理中共 {processing} 条（含他人接单）")

        # ── 3. 未完成聚合 = 待审核+待接单+处理中（全部，不受 repairer 收敛影响）──
        unfinished = count(token, "reported,pending_accept,processing")
        check("未完成 = 待审核+待接单+处理中", unfinished == unfinished_expected,
              f"接口={unfinished} 库={unfinished_expected}")

        # ── 4. 未完成排除 cancelled / completed ──
        check("未完成排除已取消", base.get("cancelled", 0) > 0 and unfinished == unfinished_expected,
              f"库中有 {base.get('cancelled', 0)} 条已取消且未被计入")
        check("未完成排除已完成", base.get("completed", 0) > 0 and unfinished == unfinished_expected,
              f"库中有 {base.get('completed', 0)} 条已完成且未被计入")

        # ── 5. 首页各区块：预览 ≤5 条 + total 计数 ──
        for status, label in (("reported", "待审核"), ("pending_accept", "待接单"), ("processing", "处理中")):
            r = requests.get(f"{BASE}/admin/orders",
                             params={"page": 1, "page_size": 5, "status": status,
                                     "sort_by": "submitted_at", "sort_order": "desc"},
                             headers={"Authorization": f"Bearer {token}"}, timeout=10)
            d = r.json().get("data") or {}
            n = len(d.get("list") or [])
            expected_total = got_processing if status == "processing" else base.get(status, 0)
            check(f"首页区块取数 {label}", n <= 5 and d.get("total") == expected_total,
                  f"预览 {n} 条 / total={d.get('total')} 期望={expected_total}")
    finally:
        cleanup_fixtures()

    passed = sum(1 for _, ok, _ in results if ok)
    print(f"\nRESULT: {passed}/{len(results)} PASS")
    sys.exit(0 if passed == len(results) else 1)


if __name__ == "__main__":
    main()
