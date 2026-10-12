#!/usr/bin/env python3
# 图片即时上传 + 提交 diff 关联 的 API 集成验收脚本
# 覆盖: 故障图 free-upload(temporary) -> 绑定 order_id 经 image_ids 翻 active -> 移除变 deleted
import json, zlib, struct, os, sys, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"
NICK = "测试用户1"
PWD = "admin123"

def make_png(path):
    w = h = 200
    raw = b""
    for y in range(h):
        raw += b"\x00"
        for x in range(w):
            raw += bytes([(x * 80) % 256, (y * 80) % 256, 128, 255])
    def chunk(typ, data):
        c = typ + data
        return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c) & 0xFFFFFFFF)
    png = b"\x89PNG\r\n\x1a\n"
    png += chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0))
    png += chunk(b"IDAT", zlib.compress(raw))
    png += chunk(b"IEND", b"")
    with open(path, "wb") as f:
        f.write(png)

def req(method, url, token=None, payload=None, file=None):
    data = None
    headers = {}
    if file is not None:
        boundary = "----xinniBoundary7Q"
        with open(file, "rb") as f:
            blob = f.read()
        body = b""
        body += ("--" + boundary + "\r\n").encode()
        body += b'Content-Disposition: form-data; name="file"; filename="t.png"\r\n'
        body += b"Content-Type: image/png\r\n\r\n"
        body += blob + b"\r\n"
        body += ("--" + boundary + "--\r\n").encode()
        data = body
        headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
    elif payload is not None:
        data = json.dumps(payload).encode()
        headers["Content-Type"] = "application/json"
    r = urllib.request.Request(url, data=data, method=method)
    for k, v in headers.items():
        r.add_header(k, v)
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, {"raw": body}

def check(cond, msg):
    print(("PASS " if cond else "FAIL ") + msg)
    if not cond:
        print("  -> ABORT")
        sys.exit(1)

tmp = "C:/Windows/Temp/xinni_test.png"
make_png(tmp)

# 1) 登录
st, body = req("POST", BASE + "/auth/admin-login", payload={"nickname": NICK, "password": PWD})
check(st == 200 and body.get("code") in (0, None), f"admin-login 200 (got {st})")
data = body.get("data", body)
token = data.get("access_token") or data.get("token")
check(bool(token), f"拿到 token (keys={list(data.keys())})")
print("  login data keys:", list(data.keys()))

# 2) 故障图 free-upload（无 order_id -> temporary + 哨兵 order_id）
st, body = req("POST", BASE + "/orders/images", token=token, file=tmp)
check(st == 200 and body.get("data", {}).get("id"), f"free-upload 故障图1 200 (got {st})")
id1 = body["data"]["id"]
print("  img1 id:", id1, "url:", body["data"].get("url"))
st, body = req("POST", BASE + "/orders/images", token=token, file=tmp)
check(st == 200 and body.get("data", {}).get("id"), f"free-upload 故障图2 200 (got {st})")
id2 = body["data"]["id"]
print("  img2 id:", id2)

# 3) 创建空草稿
st, body = req("POST", BASE + "/orders", token=token, payload={})
check(st in (200, 201), f"create 空草稿 200/201 (got {st})")
dd = body.get("data", {})
oid = dd.get("id") or dd.get("order_id")
check(bool(oid), f"create 返回 order id (data keys={list(dd.keys())})")
print("  order id:", oid)

# 4) 绑定两张图（PUT update image_ids -> AssociateImages: temporary->active）
st, body = req("PUT", BASE + f"/orders/{oid}", token=token, payload={"image_ids": [id1, id2]})
check(st == 200, f"update image_ids=[id1,id2] 200 (got {st})")
st, body = req("GET", BASE + f"/orders/{oid}", token=token)
imgs = body.get("data", {}).get("images", [])
ids = {i.get("id") for i in imgs}
check(len(imgs) == 2 and ids == {id1, id2}, f"详情含 2 张 active 故障图 (got {len(imgs)}, {ids})")

# 5) 移除 id2（PUT update image_ids=[id1] -> SoftDeleteActiveExcept: id2 -> deleted）
st, body = req("PUT", BASE + f"/orders/{oid}", token=token, payload={"image_ids": [id1]})
check(st == 200, f"update image_ids=[id1] 200 (got {st})")
st, body = req("GET", BASE + f"/orders/{oid}", token=token)
imgs = body.get("data", {}).get("images", [])
ids = {i.get("id") for i in imgs}
check(len(imgs) == 1 and ids == {id1}, f"移除后仅剩 1 张 active 故障图 (got {len(imgs)}, {ids})")

print("\nALL FAULT-IMAGE DIFF CHECKS PASSED")

# ---- 收据上传端点运行期验证（best-effort）：找一个本维修员名下的 processing 工单，上传收据确认返回 id ----
me = data.get("user", {}).get("id")
st, body = req("GET", BASE + "/admin/orders?status=processing&page=1&page_size=50", token=token)
orders = (body.get("data", {}) or {}).get("list", []) if isinstance(body.get("data"), dict) else body.get("data", [])
target = None
for o in orders:
    if o.get("repairer_id") == me and o.get("status") == "processing":
        target = o
        break
if target:
    st, body = req("POST", BASE + f"/admin/orders/{target['id']}/receipts", token=token, file=tmp)
    rid = (body.get("data", {}) or {}).get("id")
    print(f"RECEIPT-UPLOAD best-effort: status={st}, receipt_id={rid} (order {target['id']})")
    if st == 200 and rid:
        print("PASS 收据即时上传端点返回 id（temporary 记录已建，待完工/补签时经 receipt_ids diff 激活）")
    else:
        print("WARN 收据上传未返回 id（可能图床/权限），但不影响故障图 diff 结论")
else:
    print("WARN 未找到本维修员名下的 processing 工单，跳过收据上传运行期验证（收据 diff 逻辑由 v14 服务层测试覆盖）")

