# -*- coding: utf-8 -*-
"""
新泥报修系统 —— repair_orders 模拟数据生成器
------------------------------------------------------------------
用途: 生成约 1000 条工单模拟数据 (仅主表 repair_orders, 不含时间轴/图片)

约定 (与用户确认):
  - 数据量        : 约 1000 条
  - 状态分布      : 真实比例 (completed 为主, 覆盖全部 7 种状态)
  - 关联表        : 不生成 order_timeline / order_images
  - 项目字典      : 只用现有活跃字典 (大类「电脑」; 属性 更换/维修/保修)
  - 用户          : 不新建用户, 复用现有 5 个账号
  - 时间跨度      : 过去 3 个月

本脚本只生成 SQL 文件, 不直接连库:
  python seed_mock_orders.py            # 生成 seed_mock_orders.sql 与 rollback_mock_orders.sql

执行:  psql -h localhost -U postgres -d xin_ni_repair -f seed_mock_orders.sql
回滚:  psql -h localhost -U postgres -d xin_ni_repair -f rollback_mock_orders.sql
"""

import os
import random
import uuid
from datetime import datetime, timedelta

random.seed(20260910)

# ── 输出路径 ──
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
SEED_SQL = os.path.join(BASE_DIR, "seed_mock_orders.sql")
ROLLBACK_SQL = os.path.join(BASE_DIR, "rollback_mock_orders.sql")

TOTAL = 1000
NOW = datetime.now()

# ── 现有企业 (id, name, 权重) ──
ENTERPRISES = [
    ("224f5f16-d97e-4c53-9dcc-d609d3af2b24", "Cloverstia", 60),
    ("dac1d3ee-41e2-4fac-a339-8d2065a6c3b4", "测试企业2", 20),
    ("3a70ef45-3421-46e6-807b-a582e15fbfc6", "测试企业3", 20),
]

# ── 现有用户 ──
U_CLOVER = "09835d91-cc89-4798-a93c-1e38831f08e4"   # 普通用户(兼 Cloverstia/测试企业3 审核员)
U_YANLI = "10acce90-a725-4755-8eaf-fd4900e3da7a"    # 普通用户(Cloverstia 成员)
U_CS1 = "9d9f0cf7-cb49-439e-8859-4dd067fd90b9"      # 维修业务员(兼 Cloverstia/测试企业2 审核员)
U_BRUNE = "1c5505bb-9944-4ba7-885a-ae6473432899"   # 维修业务员

# 报修人池 (仅该企业的已通过成员; 确保归属一致)
REPORTERS = {
    "224f5f16-d97e-4c53-9dcc-d609d3af2b24": [U_CLOVER, U_YANLI, U_CS1],
    "dac1d3ee-41e2-4fac-a339-8d2065a6c3b4": [U_CS1],
    "3a70ef45-3421-46e6-807b-a582e15fbfc6": [U_CLOVER],
}
# 单位审核员池 (audited_by)
AUDITORS = {
    "224f5f16-d97e-4c53-9dcc-d609d3af2b24": [U_CLOVER, U_CS1],
    "dac1d3ee-41e2-4fac-a339-8d2065a6c3b4": [U_CS1],
    "3a70ef45-3421-46e6-807b-a582e15fbfc6": [U_CLOVER],
}
# 维修业务员池 (repairer_id)
REPAIRERS = [U_CS1, U_BRUNE]

# ── 项目字典 (现有活跃数据) ──
CATEGORY_ID = "967677bf-66d4-42c4-ad9f-92bcb7f506f3"
CATEGORY_NAME = "电脑"
PROPERTIES = [
    ("d4e300b9-9910-4f22-832b-bc541e28e5da", "更换"),
    ("891713ce-6f02-4a37-b754-4808f94591e3", "维修"),
    ("db4cf919-67b8-4a2b-84cc-e416c3d2468a", "保修"),
]

# ── 状态真实比例分布 ──
STATUS_WEIGHTS = [
    ("completed", 60),
    ("processing", 12),
    ("pending_accept", 10),
    ("reported", 8),
    ("draft", 5),
    ("rejected", 3),
    ("cancelled", 2),
]

# ── 描述语料 (按属性) ──
DESC = {
    "更换": [
        "设备损坏需更换配件", "键盘进水失灵需更换键盘", "硬盘故障需更换固态硬盘",
        "电源模块烧毁需更换", "内存条损坏需更换", "风扇异响需更换散热风扇",
        "显示器背光损坏需更换屏幕", "主板电容鼓包需更换主板", "电池鼓包需更换电池",
    ],
    "维修": [
        "开机无反应需检修", "运行卡顿需重装系统", "频繁蓝屏死机需排查",
        "无法联网需检修", "接口松动需维修", "主板故障需检修",
        "散热异常需清灰保养", "USB接口无响应需维修", "声音异常需排查驱动",
    ],
    "保修": [
        "保内故障申请保修", "保内主板故障需保修", "保修期内屏幕异常",
        "保内无法开机申请检修", "保修期内系统故障处理", "保内风扇异响申请处理",
        "保修期内接口失灵", "保内蓝屏申请保修",
    ],
}
ROOMS = ["1楼前台", "2楼201", "2楼205", "3楼301", "3楼会议室", "财务室",
         "行政办公室", "仓库", "机房", "教师办公室", "阅览室", "档案室"]
SURNAMES = "张李王刘陈杨赵黄周吴徐孙马朱胡郭林何高罗"
GIVEN = "伟娜强洋静帆敏磊涛倩明丽超琳军涛芳勇翔兰"
REPAIR_CONTENT = [
    "更换配件并测试正常", "检修后恢复正常运行", "重装系统并优化",
    "清洁保养后运行正常", "更换硬件并做24小时测试", "升级固件并校准",
    "排查线路并修复", "复位并更换损坏元件",
]
METHODS = ["上门维修", "送修", "远程处理"]
UNIT_PRICES = [30, 50, 80, 120, 150, 200, 280, 350, 450, 600, 800]


def esc(s):
    """SQL 单引号转义"""
    return str(s).replace("'", "''")


def q(s):
    return "'" + esc(s) + "'"


def qn(dt):
    """时间戳字面量 或 NULL"""
    return "NULL" if dt is None else "'" + dt.strftime("%Y-%m-%d %H:%M:%S%z")[:29] + "'"


def phone():
    return random.choice(["138", "139", "150", "151", "158", "186", "188"]) + "".join(
        random.choice("0123456789") for _ in range(8)
    )


def make_contact():
    name = random.choice(SURNAMES) + random.choice(GIVEN)
    return f"{name} {phone()}"


def weighted_status():
    pool = []
    for s, w in STATUS_WEIGHTS:
        pool.extend([s] * w)
    return random.choice(pool)


# order_no 序号计数: {日期: 已用序号}; 2026-09-10 已有 XNB-20260910-001, 故从 2 开始
order_seq = {}
RESERVED = {"20260910": 1}


def next_order_no(dt):
    key = dt.strftime("%Y%m%d")
    cur = order_seq.get(key, RESERVED.get(key, 0))
    cur += 1
    order_seq[key] = cur
    return f"XNB-{key}-{cur:03d}"


def add(base, lo, hi):
    return base + timedelta(minutes=random.randint(lo, hi))


rows = []

for i in range(TOTAL):
    # 企业 (加权抽样)
    ent_id, ent_name, _ = random.choices(ENTERPRISES, weights=[e[2] for e in ENTERPRISES])[0]

    prop_id, prop_name = random.choice(PROPERTIES)
    status = weighted_status()

    # 时间链: 起点落在过去 3 个月内, 生成后整体回移以避免超过当前时刻
    created = NOW - timedelta(seconds=random.randint(0, 90 * 24 * 3600))
    submitted = audited = accepted = completed = None

    needs_submit = status != "draft"
    if needs_submit:
        submitted = add(created, 5, 720)
    if status in ("pending_accept", "processing", "completed"):
        audited = add(submitted, 30, 2880)
    elif status == "rejected":
        if random.random() < 0.5:  # 审核后(待接单阶段)被退回
            audited = add(submitted, 30, 2880)
    elif status == "cancelled":
        if random.random() < 0.4:
            audited = add(submitted, 30, 900)
    if status in ("processing", "completed"):
        accepted = add(audited, 30, 4320)
    if status == "completed":
        completed = add(accepted, 60, 5760)

    # 保证末事件不晚于当前时刻: 若越界则整条时间链前移 (margin 留 5 分钟~3 天余量)
    events = [t for t in (created, submitted, audited, accepted, completed) if t]
    latest = max(events)
    margin = NOW - timedelta(minutes=random.randint(5, 4320))
    if latest > margin:
        shift = latest - margin
        created = created - shift
        submitted = submitted - shift if submitted else None
        audited = audited - shift if audited else None
        accepted = accepted - shift if accepted else None
        completed = completed - shift if completed else None

    # 各关键时间
    events = [t for t in (created, submitted, audited, accepted, completed) if t]
    updated = max(events)

    order_no = next_order_no(submitted) if needs_submit else None

    # 审核人
    audited_by = random.choice(AUDITORS[ent_id]) if audited else None
    # 维修员
    repairer_id = random.choice(REPAIRERS) if accepted else None

    # 退回原因 (>=10 字)
    reject_reason = None
    if status == "rejected":
        reject_reason = random.choice([
            "报修信息不完整，请补充设备型号与故障现象后重新提交",
            "该设备不在保修范围内，请确认后重新提交工单",
            "重复报修，已有同设备在途工单，请勿重复提交",
            "故障描述与现场情况不符，请核实后重新提交",
        ])

    # 对账字段 (仅 completed 填写)
    repair_content = ""
    quantity = 1
    unit_price = 0
    metadata = "{}"
    if status == "completed":
        repair_content = random.choice(REPAIR_CONTENT)
        quantity = random.randint(1, 3)
        unit_price = random.choice(UNIT_PRICES)
        method = random.choice(METHODS)
        warranty = random.choice(["1个月", "3个月", "6个月", "12个月", "无"])
        duration = random.randint(20, 240)
        metadata = (
            '{"repair_result":"已修复","repair_method":"' + method + '",'
            '"warranty_period":"' + warranty + '","repair_duration":' + str(duration) +
            ',"extra_remark":""}'
        )

    row_id = str(uuid.uuid4())
    rows.append(dict(
        id=row_id, order_no=order_no, enterprise_id=ent_id,
        reporter_id=random.choice(REPORTERS[ent_id]),
        category_id=CATEGORY_ID, category_name=CATEGORY_NAME,
        property_id=prop_id, property_name=prop_name,
        description=random.choice(DESC[prop_name]),
        urgency=random.choices(
            ["normal", "urgent", "very_urgent"], weights=[70, 22, 8])[0],
        status=status, reject_reason=reject_reason,
        room=random.choice(ROOMS), contact=make_contact(),
        submitted_at=submitted, audited_at=audited, audited_by=audited_by,
        accepted_at=accepted, repairer_id=repairer_id, completed_at=completed,
        created_at=created, updated_at=updated,
        repair_content=repair_content, quantity=quantity, unit_price=unit_price,
        metadata=metadata,
    ))

COLS = [
    "id", "order_no", "enterprise_id", "reporter_id",
    "category_id", "category_name", "property_id", "property_name",
    "description", "urgency", "status", "reject_reason", "room", "contact",
    "submitted_at", "audited_at", "audited_by", "accepted_at", "repairer_id",
    "completed_at", "created_at", "updated_at",
    "repair_content", "quantity", "unit_price", "metadata",
]


def val(r, c):
    v = r[c]
    if v is None:
        return "NULL"
    if c in ("submitted_at", "audited_at", "accepted_at", "completed_at",
             "created_at", "updated_at"):
        return qn(v)
    if c == "metadata":
        return q(v) + "::jsonb"
    if c in ("quantity",):
        return str(v)
    if c in ("unit_price",):
        return str(v)
    if c == "order_no":
        return q(v)
    if c in ("id", "enterprise_id", "reporter_id", "category_id", "property_id",
             "audited_by", "repairer_id"):
        return q(v) + "::uuid"
    return q(v)


with open(SEED_SQL, "w", encoding="utf-8") as f:
    f.write("-- 新泥报修系统 —— repair_orders 模拟数据 (自动生成, 请勿手工修改)\n")
    f.write(f"-- 生成时间: {NOW:%Y-%m-%d %H:%M:%S}  条数: {TOTAL}\n")
    f.write("BEGIN;\n\n")
    for r in rows:
        cols = ", ".join(COLS)
        vals = ", ".join(val(r, c) for c in COLS)
        f.write(f"INSERT INTO repair_orders ({cols}) VALUES ({vals});\n")
    f.write("\nCOMMIT;\n")

ids = ",\n".join("  " + q(r["id"]) + "::uuid" for r in rows)
with open(ROLLBACK_SQL, "w", encoding="utf-8") as f:
    f.write("-- 回滚本次生成的模拟工单 (按 ID 精确删除)\n")
    f.write(f"-- 生成时间: {NOW:%Y-%m-%d %H:%M:%S}\n")
    f.write("BEGIN;\n")
    f.write("DELETE FROM repair_orders WHERE id IN (\n" + ids + "\n);\n")
    f.write("COMMIT;\n")

# 控制台统计
from collections import Counter
print("生成完成:")
print("  条数:", len(rows))
print("  状态:", dict(Counter(r["status"] for r in rows)))
print("  企业:", dict(Counter(r["enterprise_id"][:8] for r in rows)))
print("  文件:", SEED_SQL)
print("  文件:", ROLLBACK_SQL)
