# -*- coding: utf-8 -*-
"""退单回池 (V1.9) 端到端验收脚本。

完整闭环：
  造单(草稿→上报→审核通过) → 接单 → 上传收据 → 完工(激活收据) → 重新打开
  → 退单回池 → 断言财务/收据图清零/audited保留/时间轴留痕
  → 越权校验(普通用户 403) → 下一任接单 → 重新完工(新收据/新财务，旧数据不回流)
  → 附加：超管可退他人接单的工单

运行前置：
  - 后端已在 localhost:8080 编译运行（V1.9，含 /admin/orders/:id/return）
  - postgres/123456，auto_migrate=false
  - 测试账号：Brune(维修业务员 role=1) / Clover(普通 role=0) / 管理员(超管 role=2)
用法：python scripts/return_to_pool_acceptance.py
"""
import json
import zlib
import struct
import sys
import urllib.request
import urllib.error

BASE = 'http://127.0.0.1:8080/api/v1'
_opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

PNG_PATH = 'C:/Windows/Temp/xinni_r2p.png'

passes, fails = 0, 0


def check(label, cond, detail=''):
    global passes, fails
    if cond:
        passes += 1
        print('  [PASS] ' + label + ((' ' + detail) if detail else ''))
    else:
        fails += 1
        print('  [FAIL] ' + label + ((' ' + detail) if detail else ''))


def call(method, path, token=None, body=None):
    url = BASE + path
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode('utf-8')
        headers['Content-Type'] = 'application/json'
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with _opener.open(req, timeout=15) as r:
            return r.status, json.loads(r.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        raw = e.read().decode('utf-8', 'replace')
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {'raw': raw}


def upload_file(method, path, token, file_path):
    boundary = '----xinniBoundaryR2P'
    with open(file_path, 'rb') as f:
        blob = f.read()
    body = b''
    body += ('--' + boundary + '\r\n').encode()
    body += b'Content-Disposition: form-data; name="file"; filename="r.png"\r\n'
    body += b'Content-Type: image/png\r\n\r\n'
    body += blob + b'\r\n'
    body += ('--' + boundary + '--\r\n').encode()
    headers = {'Content-Type': 'multipart/form-data; boundary=' + boundary}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(BASE + path, data=body, headers=headers, method=method)
    try:
        with _opener.open(req, timeout=30) as r:
            return r.status, json.loads(r.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        raw = e.read().decode('utf-8', 'replace')
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {'raw': raw}


def login(nickname, pwd='admin123'):
    st, res = call('POST', '/auth/admin-login', body={'nickname': nickname, 'password': pwd})
    d = (res or {}).get('data') or {}
    tok = d.get('access_token') or d.get('token')
    if st != 200 or not tok:
        print('  login failed for', nickname, '->', (st, res))
        return None, None
    return tok, (d.get('user') or {}).get('id'), (d.get('user') or {}).get('role')


def detail(token, oid):
    st, res = call('GET', f'/admin/orders/{oid}', token=token)
    if st != 200:
        return None
    return res.get('data') or {}


def make_pending_accept(t_user, t_admin, ent_id, cat_id, prop_id):
    """草稿 → 上报 → 审核通过 → 返回 pending_accept 工单 id，失败返回 None。"""
    st, res = call('POST', '/orders', token=t_user, body={})
    oid = ((res.get('data') or {}).get('id')) or ((res.get('data') or {}).get('order_id'))
    if st not in (200, 201) or not oid:
        return None
    st, _ = call('PUT', f'/orders/{oid}', token=t_user, body={
        'enterprise_id': ent_id, 'category_id': cat_id, 'property_id': prop_id,
        'room': '验收测试房间A', 'contact': '13800000000',
        'description': '验收脚本-退单回池测试工单', 'urgency': 'normal'
    })
    if st != 200:
        return None
    st, _ = call('POST', f'/orders/{oid}/submit', token=t_user, body={})
    if st != 200:
        return None
    st, _ = call('POST', f'/admin/orders/{oid}/audit', token=t_admin, body={'remark': '验收脚本自动审核'})
    if st != 200:
        return None
    return oid


def make_png(path):
    w = h = 64
    raw = b''
    for y in range(h):
        raw += b'\x00'
        for x in range(w):
            raw += bytes([(x * 4) % 256, (y * 4) % 256, 200, 255])
    def chunk(typ, data):
        c = typ + data
        return struct.pack('>I', len(data)) + c + struct.pack('>I', zlib.crc32(c) & 0xFFFFFFFF)
    png = b'\x89PNG\r\n\x1a\n'
    png += chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 6, 0, 0, 0))
    png += chunk(b'IDAT', zlib.compress(raw))
    png += chunk(b'IEND', b'')
    with open(path, 'wb') as f:
        f.write(png)


def cleanup_drafts(t_user):
    """删除当前用户的全部草稿，避免触发『草稿数量上限』限制。"""
    st, res = call('GET', '/orders?page_size=50', token=t_user)
    if st != 200:
        return
    lst = res.get('data') or []
    if not isinstance(lst, list):
        lst = ((res.get('data') or {}).get('list')) or []
    for o in lst:
        if isinstance(o, dict) and o.get('status') == 'draft' and o.get('id'):
            call('DELETE', f'/orders/{o.get("id")}', token=t_user)


def main():
    make_png(PNG_PATH)

    print('\n===== 登录三角色 =====')
    t_brune, brune_uid, _ = login('Brune')
    t_clover, _, _ = login('Clover')
    t_admin, _, _ = login('管理员')
    if not t_brune or not t_clover or not t_admin:
        print('[ABORT] 登录失败')
        sys.exit(1)
    print(f'  Brune uid={brune_uid}')

    # 清理 Clover 历史草稿，避免触发草稿数量上限
    cleanup_drafts(t_clover)

    # 取一个已有工单的 enterprise_id / category_id / property_id 用于造单
    st, res = call('GET', '/admin/orders?display_tag=processing&page_size=10', token=t_admin)
    seed = (((res.get('data') or {}).get('list')) or [])[0] if st == 200 else None
    if not seed:
        st, res = call('GET', '/admin/orders?page_size=10', token=t_admin)
        seed = (((res.get('data') or {}).get('list')) or [])[0] if st == 200 else None
    ent_id = (seed or {}).get('enterprise_id')
    cat_id = (seed or {}).get('category_id')
    prop_id = None
    if seed and seed.get('id'):
        sd = detail(t_admin, seed['id'])
        prop_id = (sd or {}).get('property_id')
    if not ent_id or not cat_id or not prop_id:
        print('[ABORT] 无法取得 enterprise_id/category_id/property_id 用于造单')
        sys.exit(1)
    print(f'  造单参考 enterprise_id={ent_id} category_id={cat_id} property_id={prop_id}')

    # ---------- 主流程 ----------
    print('\n===== 主流程：造单→接单→上传收据→完工→重开→退单回池→清场断言 =====')
    oid = make_pending_accept(t_clover, t_admin, ent_id, cat_id, prop_id)
    if not oid:
        print('[ABORT] 造 pending_accept 工单失败')
        sys.exit(1)
    print(f'  造单 O={oid} (pending_accept)')

    # 1) Brune 接单
    st, _ = call('POST', f'/admin/orders/{oid}/accept', token=t_brune, body={'remark': ''})
    check('接单 accept -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    check('接单后 status=processing', d and d.get('status') == 'processing')
    check('接单后 repairer_id=Brune', d and d.get('repairer_id') == brune_uid)

    # 2) 上传收据（temporary）
    st, res = upload_file('POST', f'/admin/orders/{oid}/receipts', t_brune, PNG_PATH)
    rid = (((res or {}).get('data') or {}).get('id')) if st == 200 else None
    check('上传收据 -> 200 且返回 id', st == 200 and bool(rid), f'HTTP {st} rid={rid}')

    # 3) 完工（带收据 + 财务），激活收据
    st, _ = call('POST', f'/admin/orders/{oid}/complete', token=t_brune, body={
        'receipt_ids': [rid], 'quantity': 2, 'unit_price': 50.0,
        'repair_content': '更换电源模块并测试', 'result': 'complete', 'sign_mode': 'signed', 'remark': ''
    })
    check('完工 complete -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    check('完工后 status=completed', d and d.get('status') == 'completed')
    rcv = [r.get('id') for r in (d or {}).get('receipts') or []]
    check('完工后收据图进入 active（出现在 receipts）', rid in rcv, f'receipts={rcv}')
    check('完工后财务已写入(quantity=2, amount=100)', d and d.get('quantity') == 2 and d.get('amount') == 100.0)

    # 4) 重新打开 → processing（收据仍 active）
    st, _ = call('POST', f'/admin/orders/{oid}/reopen', token=t_brune, body={'remark': '验收脚本重开'})
    check('重开 reopen -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    check('重开后 status=processing', d and d.get('status') == 'processing')
    rcv2 = [r.get('id') for r in (d or {}).get('receipts') or []]
    check('重开后旧收据仍 active（退单前状态）', rid in rcv2, f'receipts={rcv2}')

    # 5) 退单回池
    st, _ = call('POST', f'/admin/orders/{oid}/return', token=t_brune, body={'reason': '现场无配件需返厂'})
    check('退单 return -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    check('退单后 status=pending_accept', d and d.get('status') == 'pending_accept')
    check('退单后 repairer_id 清空', d and (d.get('repairer_id') in (None, '')))
    check('退单后 AcceptedAt 清空', d and (d.get('accepted_at') in (None, '')))
    check('退单后财务清零(repair_content空/quantity=0/unit_price=0/amount=0)',
          d and d.get('repair_content') == '' and d.get('quantity') == 0 and d.get('unit_price') == 0 and d.get('amount') == 0.0,
          f'repair_content={d.get("repair_content")!r} qty={d.get("quantity")} price={d.get("unit_price")} amount={d.get("amount")}')
    meta = d.get('metadata') if d else None
    check('退单后 metadata 清空', meta in (None, {}), f'metadata={meta}')
    check('退单后 audited_at 保留（仍是审核通过待接单）', d and d.get('audited_at') not in (None, ''))
    rcv3 = [r.get('id') for r in (d or {}).get('receipts') or []]
    check('退单后旧收据图已清（receipts 为空）', rid not in rcv3 and len(rcv3) == 0, f'receipts={rcv3}')
    actions = [t.get('action') for t in (d or {}).get('timeline') or []]
    check('时间轴含 return_to_pool 记录', 'return_to_pool' in actions)
    check('时间轴含 cleanup_receipts 记录(已清理上一任收据图)', 'cleanup_receipts' in actions, f'actions={actions}')

    # 6) 越权校验：普通用户 Clover 调退单应 403
    st, _ = call('POST', f'/admin/orders/{oid}/return', token=t_clover, body={'reason': '越权尝试'})
    check('普通用户退单 -> 403', st == 403, f'HTTP {st}')

    # 7) 下一任接单 → 重新完工（新收据/新财务，旧数据不回流）
    st, _ = call('POST', f'/admin/orders/{oid}/accept', token=t_brune, body={'remark': ''})
    check('下一任接单 accept -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    check('重新接单后 status=processing', d and d.get('status') == 'processing')
    check('重新接单后 repairer_id=Brune', d and d.get('repairer_id') == brune_uid)

    st, res = upload_file('POST', f'/admin/orders/{oid}/receipts', t_brune, PNG_PATH)
    rid2 = (((res or {}).get('data') or {}).get('id')) if st == 200 else None
    check('下一任上传新收据 -> 200', st == 200 and bool(rid2), f'HTTP {st}')
    st, _ = call('POST', f'/admin/orders/{oid}/complete', token=t_brune, body={
        'receipt_ids': [rid2], 'quantity': 3, 'unit_price': 80.0,
        'repair_content': '重做并更换主板', 'result': 'complete', 'sign_mode': 'signed', 'remark': ''
    })
    check('下一任重新完工 -> 200', st == 200, f'HTTP {st}')
    d = detail(t_brune, oid)
    rcv4 = [r.get('id') for r in (d or {}).get('receipts') or []]
    check('重新完工后仅含新收据(旧 rid 不回流)', d and rid2 in rcv4 and rid not in rcv4, f'receipts={rcv4}')
    check('重新完工后新财务写入(quantity=3, amount=240)', d and d.get('quantity') == 3 and d.get('amount') == 240.0)

    # ---------- 附加：超管可退他人接单的工单 ----------
    print('\n===== 附加：超管可退他人接单的工单 =====')
    oid2 = make_pending_accept(t_clover, t_admin, ent_id, cat_id, prop_id)
    if not oid2:
        print('  [SKIP] 造第二个 pending_accept 失败，跳过超管越权用例')
    else:
        st, _ = call('POST', f'/admin/orders/{oid2}/accept', token=t_brune, body={'remark': ''})
        check('O2 接单 -> 200', st == 200, f'HTTP {st}')
        st, _ = call('POST', f'/admin/orders/{oid2}/return', token=t_admin, body={'reason': '超管调度退单'})
        check('超管退他人工单 -> 200', st == 200, f'HTTP {st}')
        d = detail(t_admin, oid2)
        check('O2 退单后 status=pending_accept', d and d.get('status') == 'pending_accept')

    print(f'\n===== 验收汇总: PASS={passes} FAIL={fails} =====')
    sys.exit(1 if fails else 0)


if __name__ == '__main__':
    main()
