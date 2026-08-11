-- =============================================================================
-- 电脑维修店报修系统 —— 迁移脚本 007
-- 目标: users 表新增 password 字段 + phone 唯一索引 (密码登录 / 手机号唯一)
-- 版本: V1.4
-- =============================================================================

-- 1. password: bcrypt 哈希(60字符), 微信用户可为空, 平台管理员必填
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password VARCHAR(100);

COMMENT ON COLUMN users.password IS '登录密码, bcrypt 哈希; 平台管理员(role=1)必填, 微信小程序用户可为空';

-- 2. 清理重复手机号: 同一手机号只保留最早创建的一条, 其余置空
UPDATE users u
SET phone = NULL
WHERE phone IS NOT NULL AND phone <> ''
  AND u.id NOT IN (
      SELECT MIN(id) FROM users WHERE phone IS NOT NULL AND phone <> '' GROUP BY phone
  );

-- 3. phone 部分唯一索引 (排除空串, NULL 天然不冲突)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_uniq
    ON users (phone) WHERE phone IS NOT NULL AND phone <> '';

-- 4. 初始店主账号示例 (bcrypt 生成, 密码: 123456):
--    INSERT INTO users (id, openid, nickname, password, role)
--    VALUES (gen_random_uuid(), 'admin-bootstrap', 'admin',
--            '$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx', 1);
--    生成方式: python3 -c "import bcrypt; print(bcrypt.hashpw(b'123456', bcrypt.gensalt()).decode())"
