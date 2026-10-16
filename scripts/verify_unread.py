"""验收：管理端 /admin/orders 与用户端 /orders 是否返回 unread_counts（未读红点聚合）。

直连 127.0.0.1:8080，绕过系统代理。
"""
import json
import time
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def call(method, path, token=None, body=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    with opener.open(req, timeout=10) as resp:
        return json.loads(resp.read().decode())


def login(nickname, password):
    d = call("POST", "/auth/admin-login", body={"nickname": nickname, "password": password})
    return d["data"]["access_token"]


def wait_ready():
    for _ in range(30):
        try:
            call("GET", "/auth/me")  # 401 也算服务起来了
            return True
        except urllib.error.HTTPError:
            return True
        except Exception:
            time.sleep(0.5)
    return False


def main():
    if not wait_ready():
        print("SERVER NOT READY")
        return

    admin = login("管理员", "admin123")
    res = call("GET", "/admin/orders?page=1&page_size=5", admin)
    data = res["data"]
    print("[ADMIN] /admin/orders keys:", sorted(data.keys()))
    print("        unread_counts:", data.get("unread_counts"))
    print("        status_counts:", data.get("status_counts"))

    brune = login("Brune", "admin123")
    res2 = call("GET", "/orders?page=1&page_size=5", brune)
    data2 = res2["data"]
    print("[USER]  /orders keys:", sorted(data2.keys()))
    print("        unread_counts:", data2.get("unread_counts"))
    print("        pagination.total:", data2.get("pagination", {}).get("total"))


if __name__ == "__main__":
    main()
