-- =============================================================================
-- identity_db 初始 8 表（S3-01,v4/03 正本 + 02 §6.2 ADR-08 口径）
--
-- 设计要点：
--   * 主键一律雪花 BIGINT（不自增：多实例撞号、可被遍历）；
--   * mobile/email AES-256-GCM 信封加密存储（拖库不泄露），加密列无法建索引——
--     另设 *_hash = HMAC-SHA256(明文, MICRO_HASH_KEY) CHAR(64) 做唯一约束与等值查询；
--   * 通用字段全表强制：tenant_id / created_at / created_by / updated_at / updated_by / deleted_at
--     （tenant 是租户根表，自身不含 tenant_id）；
--   * 会话/锁定计数在 Redis DB4（sessionx），MySQL 只存"账号是什么"；
--   * user.locked_until 与 sessionx locked:{ident} 同语义：5 次失败锁 30 分钟，到期自动解锁。
-- =============================================================================

-- ----------------------------------------------------------------------------
-- 租户表（tenant_code UK + 配额 JSON）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `tenant`
(
    `tenant_id`   BIGINT       NOT NULL COMMENT '雪花 ID',
    `tenant_code` VARCHAR(64)  NOT NULL COMMENT '租户编码（登录上下文/配置键引用）',
    `name`        VARCHAR(128) NOT NULL COMMENT '租户名称',
    `plan`        VARCHAR(32)  NOT NULL DEFAULT 'STANDARD' COMMENT '套餐：STANDARD/PRO/ENTERPRISE',
    `quota`       JSON         NULL COMMENT '配额 JSON：{"rpm":600,"user_max":100,...}（tenantx 配额源）',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1 正常 / 2 停用',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记（NULL=有效）',
    PRIMARY KEY (`tenant_id`),
    UNIQUE KEY `uk_tenant_code` (`tenant_code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='租户';

-- ----------------------------------------------------------------------------
-- 用户表（mobile/email 加密 + hash 唯一索引；types 单账号多端）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user`
(
    `user_id`       BIGINT       NOT NULL COMMENT '雪花 ID（对外一律字符串渲染，E8）',
    `org_id`        BIGINT       NULL COMMENT '归属部门（逻辑外键 org.org_id，可空=未分配）',
    `party_id`      BIGINT       NULL COMMENT '关联参与方（S4 补充，可空）',
    `nickname`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '展示名（后台用户列表"姓名"列；真实姓名档案归 S4 staff）',
    `mobile`        VARCHAR(512) NULL COMMENT '手机号 AES-256-GCM 信封密文（base64：版本+kid+nonce+GCM 密文）',
    `mobile_hash`   CHAR(64)     NULL COMMENT 'HMAC-SHA256(mobile, MICRO_HASH_KEY)：唯一索引与等值查询用',
    `email`         VARCHAR(512) NULL COMMENT '邮箱 AES-256-GCM 信封密文',
    `email_hash`    CHAR(64)     NULL COMMENT 'HMAC-SHA256(email)：唯一索引用',
    `password_hash` VARCHAR(255) NULL COMMENT 'argon2id PHC 串（含盐含参数；微信-only 用户可空）',
    `types`         JSON         NOT NULL COMMENT '可登录端集合 ["ADMIN_WEB","OPS_APP"]——单账号多端',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1 正常 / 2 禁用（禁用即时踢全部会话）/ 3 锁定',
    `locked_until`  DATETIME     NULL COMMENT '锁定到期时间（5 次失败锁 30 分钟，自动解锁；与 sessionx locked:{ident} 同语义）',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID（登录按 mobile_hash 全局 UK 定位，管理查询显式携带）',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME     NULL COMMENT '软删标记（NULL=有效）',
    PRIMARY KEY (`user_id`),
    UNIQUE KEY `uk_user_mobile_hash` (`mobile_hash`),
    UNIQUE KEY `uk_user_email_hash` (`email_hash`),
    KEY `idx_user_tenant` (`tenant_id`),
    KEY `idx_user_org` (`org_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='用户';

-- ----------------------------------------------------------------------------
-- 组织表（parent_id 树）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `org`
(
    `org_id`     BIGINT       NOT NULL COMMENT '雪花 ID',
    `parent_id`  BIGINT       NOT NULL DEFAULT 0 COMMENT '父组织 ID（0=根）',
    `name`       VARCHAR(128) NOT NULL COMMENT '组织名称',
    `sort`       INT          NOT NULL DEFAULT 0 COMMENT '同级排序',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记',
    PRIMARY KEY (`org_id`),
    KEY `idx_org_tenant_parent` (`tenant_id`, `parent_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='组织（部门树）';

-- ----------------------------------------------------------------------------
-- 角色表（(tenant_id, code) UK + data_scope JSON）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `role`
(
    `role_id`     BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`        VARCHAR(64)  NOT NULL COMMENT '角色码（admin 等；租户内唯一）',
    `name`        VARCHAR(128) NOT NULL COMMENT '角色名',
    `data_scope`  JSON         NULL COMMENT '数据域 JSON：{"type":"ALL"|"SELF"|"ORG","org_ids":[...]}（与 ctxkit.DataScope 同构）',
    `remark`      VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记',
    PRIMARY KEY (`role_id`),
    UNIQUE KEY `uk_role_tenant_code` (`tenant_id`, `code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='角色';

-- ----------------------------------------------------------------------------
-- 菜单表（perm_code (tenant_id,perm_code) UK；目录-菜单-按钮-接口四级）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `menu`
(
    `menu_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `parent_id`  BIGINT       NOT NULL DEFAULT 0 COMMENT '父菜单 ID（0=根）',
    `name`       VARCHAR(64)  NOT NULL COMMENT '菜单/按钮/接口名',
    `type`       TINYINT      NOT NULL COMMENT '1 目录 / 2 菜单 / 3 按钮 / 4 接口（四级）',
    `perm_code`  VARCHAR(128) NULL COMMENT '权限码（system:user:list 等；目录/菜单可空）',
    `path`       VARCHAR(255) NULL COMMENT '前端路由（type=2）或接口路径模式（type=4）',
    `icon`       VARCHAR(64)  NULL COMMENT '图标',
    `sort`       INT          NOT NULL DEFAULT 0 COMMENT '同级排序',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用 / 2 停用',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID（菜单集按租户独立）',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记',
    PRIMARY KEY (`menu_id`),
    UNIQUE KEY `uk_menu_tenant_perm` (`tenant_id`, `perm_code`),
    KEY `idx_menu_tenant_parent` (`tenant_id`, `parent_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='菜单（目录-菜单-按钮-接口四级，perm_code 权限源）';

-- ----------------------------------------------------------------------------
-- 用户-角色（联合主键）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_role`
(
    `user_id`    BIGINT   NOT NULL COMMENT '用户 ID',
    `role_id`    BIGINT   NOT NULL COMMENT '角色 ID',
    `tenant_id`  BIGINT   NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT   NULL COMMENT '创建人 uid',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT   NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME NULL COMMENT '软删标记（绑定解绑走硬删，字段仅为全表字段统一）',
    PRIMARY KEY (`user_id`, `role_id`),
    KEY `idx_user_role_role` (`role_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='用户-角色绑定';

-- ----------------------------------------------------------------------------
-- 角色-菜单（联合主键）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `role_menu`
(
    `role_id`    BIGINT   NOT NULL COMMENT '角色 ID',
    `menu_id`    BIGINT   NOT NULL COMMENT '菜单 ID',
    `tenant_id`  BIGINT   NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT   NULL COMMENT '创建人 uid',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT   NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME NULL COMMENT '软删标记（绑定解绑走硬删，字段仅为全表字段统一）',
    PRIMARY KEY (`role_id`, `menu_id`),
    KEY `idx_role_menu_menu` (`menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='角色-菜单绑定';

-- ----------------------------------------------------------------------------
-- 用户 SSO 绑定（provider + openid UK；微信 code2session 后落此表）
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_sso_binding`
(
    `binding_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `user_id`    BIGINT       NOT NULL COMMENT '用户 ID',
    `provider`   VARCHAR(32)  NOT NULL COMMENT 'SSO 提供方：WECHAT_MA / WECHAT_MP / DINGTALK ...',
    `openid`     VARCHAR(128) NOT NULL COMMENT '提供方侧 openid',
    `unionid`    VARCHAR(128) NULL COMMENT '提供方侧 unionid（可空）',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记',
    PRIMARY KEY (`binding_id`),
    UNIQUE KEY `uk_sso_provider_openid` (`provider`, `openid`),
    KEY `idx_sso_user` (`user_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='用户 SSO 绑定';
