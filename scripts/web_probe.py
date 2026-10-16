"""Web 端问题复现：按角色调用关键接口，检查返回码与耗时。"""
import json
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = 'http://127.0.0.1:8080/api/v1'
op = urllib.request.ProxyHandler({})


class C:
    def __init__(self, nick, pw):
        self.nick = nick
        self.cl = urllib.request.build_opener(op)
        req = urllib.request.Request(
            BASE + '/auth/admin-login',
            data=json.dumps({'nickname': nick, 'password': pw}).encode(),
            headers={'Content-Type': 'application/json'})
        d = json.loads(self.cl.open(req, timeout=10).read().decode())
        self.token = d['data']['access_token']
        self.user = d['data']['user']

    def get(self, path, params=None):
        url = BASE + path
        if params:
            url += '?' + urllib.parse.urlencode(params)
        req = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + self.token})
        t0 = time.time()
        try:
            r = self.cl.open(req, timeout=15)
            body = json.loads(r.read().decode())
            return r.status, body, round((time.time() - t0) * 1000)
        except urllib.error.HTTPError as e:
            body = e.read().decode()
            try:
                body = json.loads(body)
            except Exception:
                pass
            return e.code, body, round((time.time() - t0) * 1000)


def brief(body, keys=('total', 'list', 'cards', 'status_counts')):
    if not isinstance(body, dict):
        return body
    out = {}
    data = body.get('data', body)
    for k in keys:
        if isinstance(data, dict) and k in data:
            v = data[k]
            if isinstance(v, list):
                out[k] = f'list(len={len(v)})'
            elif isinstance(v, dict):
                out[k] = json.dumps(v, ensure_ascii=False)[:200]
            else:
                out[k] = v
    return {'code': body.get('code'), 'message': body.get('message'), **out}


accounts = [('Clover', 'admin123'), ('Brune', 'admin123'), ('管理员', 'admin123')]
for nick, pw in accounts:
    try:
        c = C(nick, pw)
    except Exception as e:
        print('LOGIN FAIL', nick, e)
        continue
    ents = c.user.get('enterprises') or []
    print('=' * 70)
    print(f'[{nick}] role={c.user.get("role")} ents={[(e["enterprise_name"], e["role"], e["status"]) for e in ents]}')

    paths = [
        ('/dashboard', None),
        ('/orders', {'page': 1, 'page_size': 20}),
        ('/admin/orders', {'page': 1, 'page_size': 20}),
        ('/admin/orders', {'page': 1, 'page_size': 20, 'display_tag': 'pending_review', 'sort_by': 'board', 'sort_order': 'asc'}),
        ('/admin/orders', {'page': 1, 'page_size': 20, 'display_tag': 'pending_accept', 'sort_by': 'board', 'sort_order': 'asc'}),
        ('/admin/orders', {'page': 1, 'page_size': 20, 'display_tag': 'processing', 'repairer_id': 'me', 'sort_by': 'board', 'sort_order': 'asc'}),
        ('/admin/enterprises', {'page': 1, 'page_size': 100}),
        ('/admin/repairers', None),
        ('/orders/options', None),
    ]
    if ents:
        paths.insert(1, ('/enterprises/' + ents[0]['enterprise_id'], None))
    for p, q in paths:
        st, body, ms = c.get(p, q)
        print(f'  {st} {ms}ms {p} {json.dumps(q or {}, ensure_ascii=False)}')
        print(f'      {json.dumps(brief(body), ensure_ascii=False)[:260]}')
