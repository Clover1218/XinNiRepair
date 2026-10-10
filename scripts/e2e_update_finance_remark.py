# -*- coding: utf-8 -*-
"""
E2E: 验证「修改对账信息弹窗」新增 维修备注(remark) 的真实持久化与回显逻辑。
- 直接 DB 插入一条 completed 工单（含真实企业/用户 FK）
- 伪造超级管理员 JWT（HS256, 空密钥, issuer=xin-ni-repair, role=2）
- 仅传 remark 调用 /admin/orders/:id/finance
- 校验 order.remark 被更新，且时间轴生成「维修备注: '' -> 'xxx'」diff
"""
import base64
import hashlib
import hmac
import json
import time
import urllib.request
import uuid

import psycopg2

DB = dict(host="localhost", port=5432, dbname="xin_ni_repair",
          user="postgres", password="123456")

ENT = "224f5f16-d97e-4c53-9dcc-d609d3af2b24"   # Cloverstia
REPORTER = "9d9f0cf7-cb49-439e-8859-4dd067fd90b9"  # 维修业务员(兼审核员) 真实 uuid
CATEGORY = "967677bf-66d4-42c4-ad9f-92bcb7f506f3"
CATEGORY_NAME = "电脑"
PROP = "d4e300b9-9910-4f22-832b-bc541e28e5da"
PROP_NAME = "更换"

BASE = "http://localhost:8080/api/v1"
SECRET = b""          # 配置 jwt.secret 为空
ISSUER = "xin-ni-repair"
SUPER_UID = REPORTER  # 用作 token uid（真实 uuid，避免 timeline operator_id 报错）


def b64url(b):
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode("ascii")


def make_token(uid, role):
    header = b64url(json.dumps({"alg": "HS256", "typ": "JWT"}).encode())
    now = int(time.time())
    payload = b64url(json.dumps({
        "uid": uid, "role": role,
        "iss": ISSUER, "sub": uid,
        "iat": now, "exp": now + 7200,
    }).encode())
    signing = f"{header}.{payload}".encode()
    sig = hmac.new(SECRET, signing, hashlib.sha256).digest()
    return f"{header}.{payload}.{b64url(sig)}"


TOKEN = make_token(SUPER_UID, 2)


def http_post(path, body):
    req = urllib.request.Request(
        BASE + path, data=json.dumps(body).encode(),
        headers={"Authorization": f"Bearer {TOKEN}",
                 "Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode())


def http_get(path):
    req = urllib.request.Request(
        BASE + path, headers={"Authorization": f"Bearer {TOKEN}"}, method="GET")
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.status, json.loads(r.read().decode())


def main():
    conn = psycopg2.connect(**DB)
    conn.autocommit = True
    cur = conn.cursor()
    oid = str(uuid.uuid4())
    order_no = "XNB-E2E-%s" % uuid.uuid4().hex[:6]
    now = "NOW()"
    cur.execute("""
        INSERT INTO repair_orders (
            id, order_no, enterprise_id, reporter_id,
            category_id, category_name, property_id, property_name,
            description, urgency, status, room, contact,
            submitted_at, audited_at, audited_by, accepted_at, repairer_id,
            completed_at, created_at, updated_at,
            repair_content, quantity, unit_price, metadata, signature_status
        ) VALUES (
            %s::uuid, %s, %s::uuid, %s::uuid,
            %s::uuid, %s, %s::uuid, %s,
            'E2E 测试工单', 'normal', 'completed', 'E2E机房', '测试员 13800000000',
            %s, %s, %s::uuid, %s, %s::uuid,
            %s, %s, %s,
            '初始维修内容', 1, 50, '{}'::jsonb, 'signed'
        )
    """, (oid, order_no, ENT, REPORTER, CATEGORY, CATEGORY_NAME, PROP,
          PROP_NAME, now, now, REPORTER, now, REPORTER, now, now, now))

    print("插入工单:", oid)
    try:
        # 1) 仅传 remark
        st, body = http_post(f"/admin/orders/{oid}/finance",
                             {"remark": "E2E 修改备注：已当面交付并签字确认"})
        print("finance(仅 remark) ->", st, body)

        # 2) DB 校验 remark 已落库
        cur.execute("SELECT remark FROM repair_orders WHERE id=%s::uuid", (oid,))
        db_remark = cur.fetchone()[0]
        print("DB remark =", repr(db_remark))

        # 3) 时间轴校验 diff 含「维修备注」
        st2, detail = http_get(f"/admin/orders/{oid}")
        timeline = detail.get("data", {}).get("timeline", [])
        last = timeline[-1] if timeline else {}
        print("时间轴末条 action=", last.get("action"), "remark=", repr(last.get("remark")))

        # 4) 再传一次：把备注清空，验证可清空
        st3, body3 = http_post(f"/admin/orders/{oid}/finance", {"remark": ""})
        print("finance(清空 remark) ->", st3, body3)
        cur.execute("SELECT remark FROM repair_orders WHERE id=%s::uuid", (oid,))
        db_remark2 = cur.fetchone()[0]
        print("DB remark(after clear) =", repr(db_remark2))

        # 判定
        ok = (st == 200 and db_remark == "E2E 修改备注：已当面交付并签字确认"
              and "维修备注" in (last.get("remark") or "")
              and st3 == 200 and db_remark2 == "")
        print("RESULT:", "PASS" if ok else "FAIL")
    finally:
        cur.execute("DELETE FROM repair_orders WHERE id=%s::uuid", (oid,))
        print("已清理测试工单")
    conn.close()


if __name__ == "__main__":
    main()
