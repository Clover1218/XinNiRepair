-- 018 首页待办看板：工单级已读时间戳
--
-- 背景：首页改为「按状态聚合的待办看板」，未读红点判定规则为
--   order_timeline.created_at > 该用户对该工单的最后查看时间
--
-- 为什么独立建表而不是在 repair_orders 上加一列 user_last_viewed_at：
-- 同一张工单会被报修人 / 单位审核员 / 维修业务员多个角色查看，
-- 单列只能记一个人的时间，后查看的人会覆盖先查看的人，导致红点错乱。
-- 故按 (工单 × 用户) 维度存，互不影响。

CREATE TABLE IF NOT EXISTS order_reads (
    order_id       uuid        NOT NULL,
    user_id        uuid        NOT NULL,
    last_viewed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id, user_id)
);

-- 按用户批量拉取已读时间（看板一次性查当前用户所有可见工单的已读态）
CREATE INDEX IF NOT EXISTS idx_order_reads_user ON order_reads (user_id);

COMMENT ON TABLE order_reads IS '工单级已读时间戳（首页待办看板未读红点）';
COMMENT ON COLUMN order_reads.last_viewed_at IS '该用户最后一次查看该工单详情的时间';
