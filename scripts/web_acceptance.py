# -*- coding: utf-8 -*-
"""Web 端移植验收（修正版）：模拟小程序 V1.7 首页看板 + 工单列表 的 Web 调用。
后端契约要点：
  - 列表响应 DTO 不含 repairer_id 字段（无法据此断言归属，过滤在服务端生效）。
  - repairer_id=me：role=1 维修员服务端严格收敛本人；role=2 超管不受收敛（设计如此）。
  - 列表 display_tag 须用卡片返回的 tag 字段（如 'processing'），不是卡片 key。
依赖：后端已在 localhost:8080 运行（postgres/123456，auto_migrate=false）。
用法：python scripts/web_acceptance.py
"""
import json
import urllib.request
import urllib.error

BASE = 'http://127.0.0.1:8080/api/v1'
_opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


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
        with _opener.open(req, timeout=10) as r:
            return r.status, json.loads(r.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        raw = e.read().decode('utf-8', 'replace')
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, raw


def login(nickname, pwd='admin123'):
    st, res = call('POST', '/auth/admin-login', body={'nickname': nickname, 'password': pwd})
    d = res.get('data') or {}
    tok = d.get('access_token') or d.get('token')
    if st != 200 or not tok:
        return None, (st, res)
    return tok, d


passes, fails = 0, 0


def check(label, cond, detail=''):
    global passes, fails
    if cond:
        passes += 1
        print(f'  [PASS] {label} {detail}')
    else:
        fails += 1
        print(f'  [FAIL] {label} {detail}')


def audit_account(nickname):
    print(f'\n===== 账号: {nickname} =====')
    token, info = login(nickname)
    if not token:
        print('  login failed:', info)
        return
    role = (info or {}).get('user', {}).get('role')
    uid = (info or {}).get('user', {}).get('id')
    print(f'  role = {role} (0普通 / 1维修业务员 / 2超管)')

    # 1) 看板卡片矩阵 + personal 分组 + 单卡 card-detail
    st, res = call('GET', '/dashboard', token=token)
    check('GET /dashboard -> 200', st == 200, f'HTTP {st}')
    data = (res or {}).get('data') or {}
    cards = data.get('cards') or []
    personal_cards = [c for c in cards if c.get('personal')]
    print(f'  卡片数={len(cards)} unread_total={data.get("unread_total")} personal分组={[c["key"] for c in personal_cards]}')
    for c in cards:
        print(f'     - key={c["key"]:<20} label={c.get("label"):<8} tag={c.get("tag"):<18} total={c.get("total"):<3} unread={c.get("unread")} personal={c.get("personal")}')
    if cards:
        key = cards[0].get('key')
        st, res2 = call('GET', f'/dashboard/card-detail?key={key}', token=token)
        check(f'GET /dashboard/card-detail(key={key}) -> 200', st == 200, f'HTTP {st}')
        if st == 200:
            orders = ((res2.get('data') or {}).get('orders')) or []
            check('  单卡返回 orders 列表', isinstance(orders, list))

    if role == 0:
        # 普通用户：my_repair 走用户端 GET /orders
        sample = next((c for c in cards if c.get('tag')), None)
        tag = sample.get('tag') if sample else ''
        st, res = call('GET', f'/orders?page=1&page_size=10&display_tag={tag}&sort=board', token=token)
        check('用户端 GET /orders(display_tag+sort=board) -> 200', st == 200, f'HTTP {st}')
        if st == 200:
            lst = ((res.get('data') or {}).get('list')) or []
            print(f'     返回 {len(lst)} 条')
    else:
        # 审核员 / 维修员 / 超管：review / worker_task 走管理端 GET /admin/orders
        for c in cards:
            tag = c.get('tag') or ''
            st, res = call('GET', f'/admin/orders?page=1&page_size=10&display_tag={tag}&sort_by=board', token=token)
            ok = st == 200
            sc = ((res.get('data') or {}).get('status_counts')) or {}
            cnt = len(((res.get('data') or {}).get('list')) or []) if ok else 0
            check(f'管理端 GET /admin/orders(tag={tag or "空"},sort_by=board) -> 200', ok, f'HTTP {st} cnt={cnt} status_counts={len(sc)}')
        # repairer_id=me（worker_task 我的维修）
        if role == 1:
            # 先取看板 repairer_processing 卡片的总数以交叉验证
            rp = next((c for c in cards if c['key'] == 'repairer_processing'), None)
            st, res = call('GET', '/admin/orders?page=1&page_size=50&sort_by=board&repairer_id=me', token=token)
            check('GET /admin/orders(repairer_id=me) -> 200', st == 200, f'HTTP {st}')
            if st == 200:
                lst = ((res.get('data') or {}).get('list')) or []
                # 服务端已收敛；用 processing 子查询交叉验证数量一致
                st2, res2 = call('GET', '/admin/orders?page=1&page_size=50&sort_by=board&repairer_id=me&display_tag=processing', token=token)
                proc = len(((res2.get('data') or {}).get('list')) or []) if st2 == 200 else -1
                check('  repairer_id=me + display_tag=processing 数量 == 看板 repairer_processing.total',
                      (rp is None and proc == 0) or (rp is not None and proc == rp.get('total')),
                      f'proc={proc} 看板total={rp.get("total") if rp else None}')
        else:
            # 超管：repairer_id=me 不受收敛（设计如此），仅验证 200
            st, res = call('GET', '/admin/orders?page=1&page_size=50&sort_by=board&repairer_id=me', token=token)
            check('GET /admin/orders(repairer_id=me, 超管) -> 200 (不受收敛, 设计如此)', st == 200, f'HTTP {st}')


def main():
    for name in ['Clover', '测试用户1', 'Brune', '管理员']:
        try:
            audit_account(name)
        except Exception as e:
            print(f'  [ERR] {name}: {e}')
    print(f'\n===== 验收汇总: PASS={passes} FAIL={fails} =====')


if __name__ == '__main__':
    main()
