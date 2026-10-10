-- 015: 系统设置表（超管可调参数持久化；当前承载「完工后二次修改时限」闸门阈值）
CREATE TABLE IF NOT EXISTS system_settings (
    key         varchar(50)   PRIMARY KEY,
    value       text          NOT NULL DEFAULT '',
    description varchar(200)  NOT NULL DEFAULT '',
    updated_at  timestamptz   NOT NULL DEFAULT now(),
    updated_by  uuid
);

INSERT INTO system_settings (key, value, description) VALUES
('completed_edit_window_hours', '24', '完工后允许二次修改/重开的时限（小时），超时后仅超级管理员可操作')
ON CONFLICT (key) DO NOTHING;
