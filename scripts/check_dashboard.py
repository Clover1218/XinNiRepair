# -*- coding: utf-8 -*-
"""V1.7 首页看板 / 列表排序 接口验收脚本（本地库）。

用法：python scripts/check_dashboard.py
依赖：后端已在 localhost:8080 启动。
"""
import json
import urllib.request
import urllib.error

BASE = 'http://localhost:8080/api/v1'


# 本机直连：绕开可能存在的 http_proxy 环境变量（否则 127.0.0.1 也会走代理拿到 502）
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


def show(label, token, extra=''):
    st, res = call('GET', '/dashboard' + extra, token=token)
    if st != 200:
        print(f'  [{label}] dashboard -> HTTP {st}: {res}')
        return
    data = res.get('data') or {}
    cards = data.get('cards') or []
    print(f'  [{label}] cards={len(cards)} unread_total={data.get("unread_total")}')
    for c in cards:
        orders = c.get('orders') or []
        tags = [(o.get('display_tag'), o.get('status')) for o in orders]
        print(f'     - {c["key"]:<22} label={c["label"]:<6} total={c["total"]:<3} unread={c["unread"]} {tags}')


def main():
    for nickname in ['Clover', '测试用户1', '管理员']:
        token, info = login(nickname)
        print(f'== {nickname} ==')
        if not token:
            print('  login failed:', info)
            continue
        print('  role =', (info or {}).get('user', {}).get('role'))
        show('dashboard', token)

        # 列表排序验收：用户端 /orders?sort=board
        st, res = call('GET', '/orders?page=1&page_size=5&sort=board', token=token)
        if st == 200:
            lst = ((res.get('data') or {}).get('list')) or []
            print('  /orders sort=board ->', [(o.get('status'), o.get('display_tag')) for o in lst])
        else:
            print('  /orders sort=board -> HTTP', st, res)

        # 管理端 /admin/orders?sort_by=board
        st, res = call('GET', '/admin/orders?page=1&page_size=5&sort_by=board', token=token)
        if st == 200:
            lst = ((res.get('data') or {}).get('list')) or []
            print('  /admin/orders sort_by=board ->', [(o.get('status'), o.get('display_tag')) for o in lst])
        else:
            print('  /admin/orders sort_by=board -> HTTP', st, res)

        # 维修员「我的维修」：repairer_id=me
        st, res = call('GET', '/admin/orders?page=1&page_size=5&sort_by=board&repairer_id=me', token=token)
        if st == 200:
            lst = ((res.get('data') or {}).get('list')) or []
            print('  /admin/orders repairer_id=me ->', len(lst), '条')
        else:
            print('  /admin/orders repairer_id=me -> HTTP', st, res)
        print()


if __name__ == '__main__':
    main()
