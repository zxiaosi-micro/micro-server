-- party_db 初始 schema（S4-01，v4/03 阶段 4）
-- 规约（02 §6.1/§6.2）：雪花 BIGINT 主键不自增；全表六通用字段；
-- 状态用 VARCHAR 枚举（加枚举零 DDL）；JSON 列存复合语义；utf8mb4/InnoDB。

-- 参与方：type JSON SET 语义（CUSTOMER/DEALER/SUPPLIER/ENTERPRISE/RECYCLER 可并存）；
-- credit_code 企业统一社会信用代码，租户内唯一（跨租户允许同一供应商各自建档）。
CREATE TABLE IF NOT EXISTS `party`
(
    `party_id`    BIGINT       NOT NULL COMMENT '雪花 ID(对外一律字符串渲染,E8)',
    `name`        VARCHAR(128) NOT NULL COMMENT '参与方名称',
    `type`        VARCHAR(255) NOT NULL COMMENT '类型 JSON 数组 SET 语义:["CUSTOMER","DEALER","SUPPLIER","ENTERPRISE","RECYCLER"] 可并存',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1 正常/2 停用',
    `credit_code` VARCHAR(32)  NULL COMMENT '统一社会信用代码(租户内企业唯一)',
    `region`      VARCHAR(64)  NULL COMMENT '所在区域',
    `address`     VARCHAR(255) NULL COMMENT '详细地址',
    `remark`      VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`party_id`),
    UNIQUE KEY `uk_party_tenant_credit` (`tenant_id`, `credit_code`),
    KEY `idx_party_tenant` (`tenant_id`),
    KEY `idx_party_name` (`tenant_id`, `name`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='参与方';

-- 联系人：mobile AES-256-GCM 加密存储 + HMAC-SHA256 哈希索引列（同 identity user.mobile 口径）。
CREATE TABLE IF NOT EXISTS `contact`
(
    `contact_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `party_id`    BIGINT       NOT NULL COMMENT '归属参与方',
    `name`        VARCHAR(64)  NOT NULL COMMENT '联系人姓名',
    `mobile`      VARCHAR(255) NOT NULL COMMENT '手机号(AES-256-GCM 密文)',
    `mobile_hash` CHAR(64)     NOT NULL COMMENT 'HMAC-SHA256 索引哈希',
    `position`    VARCHAR(64)  NULL COMMENT '职务',
    `is_default`  TINYINT      NOT NULL DEFAULT 0 COMMENT '1 默认联系人',
    `notify_pref` VARCHAR(255) NULL COMMENT '通知偏好 JSON {"sms":true,"wechat":false}',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`contact_id`),
    KEY `idx_contact_party` (`party_id`),
    KEY `idx_contact_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='参与方联系人';

-- 员工：user_id 逻辑引用 identity.user（跨库禁止 JOIN，01 §7.2）；
-- skill_tags + work_region 是 S7 派单匹配依据。
CREATE TABLE IF NOT EXISTS `staff`
(
    `staff_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `party_id`    BIGINT       NOT NULL COMMENT '归属参与方(服务商)',
    `user_id`     BIGINT       NULL COMMENT '逻辑引用 identity.user(可空=未开账号)',
    `name`        VARCHAR(64)  NOT NULL COMMENT '员工姓名',
    `staff_type`  VARCHAR(32)  NOT NULL DEFAULT 'ENGINEER' COMMENT 'ENGINEER 工程师/SALES 销售/OPS 运维/ADMIN 管理',
    `skill_tags`  VARCHAR(512) NULL COMMENT '技能标签 JSON ["INSTALL","COMMISSIONING","REPAIR"]',
    `work_region` VARCHAR(128) NULL COMMENT '常驻工作区域(S7 派单匹配)',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1 在职/2 离职',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`staff_id`),
    KEY `idx_staff_party` (`party_id`),
    KEY `idx_staff_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='服务商员工';

-- 经销商扩展信息（party.type 含 DEALER 时维护；1:1，PK 即 party_id）。
CREATE TABLE IF NOT EXISTS `dealer_ext`
(
    `party_id`          BIGINT       NOT NULL COMMENT '参与方 ID(1:1)',
    `dealer_level`      VARCHAR(32)  NOT NULL DEFAULT 'STANDARD' COMMENT 'STANDARD/GOLD/PLATINUM',
    `authorized_region` VARCHAR(255) NULL COMMENT '授权销售区域',
    `rebate_rule`       VARCHAR(512) NULL COMMENT '返利规则 JSON {"rate":0.03,"settle":"QUARTER"}',
    `tenant_id`         BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`        BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`        BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`        DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`party_id`),
    KEY `idx_dealer_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='经销商扩展';

-- CRM 跟进记录（追加式，业务上禁改删）。
CREATE TABLE IF NOT EXISTS `crm_record`
(
    `record_id`      BIGINT         NOT NULL COMMENT '雪花 ID',
    `party_id`       BIGINT         NOT NULL COMMENT '归属参与方',
    `content`        VARCHAR(1024)  NOT NULL COMMENT '跟进内容',
    `next_follow_at` DATETIME       NULL COMMENT '下次跟进时间',
    `tenant_id`      BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`     BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`record_id`),
    KEY `idx_crm_party` (`party_id`),
    KEY `idx_crm_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='CRM 跟进记录';

-- 商机：stage VARCHAR 枚举（NEW/CONTACTING/PROPOSAL/NEGOTIATION/WON/LOST，加枚举零 DDL）。
CREATE TABLE IF NOT EXISTS `opportunity`
(
    `opportunity_id`      BIGINT         NOT NULL COMMENT '雪花 ID',
    `party_id`            BIGINT         NOT NULL COMMENT '关联客户参与方',
    `title`               VARCHAR(128)   NOT NULL COMMENT '商机标题',
    `stage`               VARCHAR(32)    NOT NULL DEFAULT 'NEW' COMMENT 'NEW/CONTACTING/PROPOSAL/NEGOTIATION/WON/LOST',
    `amount`              DECIMAL(14, 2) NULL COMMENT '预估金额(元)',
    `expected_close_date` DATE           NULL COMMENT '预计成交日期',
    `remark`              VARCHAR(255)   NULL COMMENT '备注',
    `tenant_id`           BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`          DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`          BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`          DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`          BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`          DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`opportunity_id`),
    KEY `idx_opp_party` (`party_id`),
    KEY `idx_opp_stage` (`tenant_id`, `stage`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='商机';
