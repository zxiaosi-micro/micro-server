-- notification_db 初始 schema（S4-04，v4/03 阶段 4）
-- 业务服务只发 notification_request 事件，不直连渠道（FR-NTF-001）；渠道 Provider 化。

-- 站内信（消息中心收件箱，FR-NTF-004）。
CREATE TABLE IF NOT EXISTS `message`
(
    `message_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `user_id`    BIGINT       NOT NULL COMMENT '收件人 uid',
    `title`      VARCHAR(255) NOT NULL COMMENT '标题(模板渲染)',
    `content`    VARCHAR(2048) NOT NULL COMMENT '内容(模板渲染)',
    `is_read`    TINYINT      NOT NULL DEFAULT 0 COMMENT '0 未读/1 已读',
    `biz_type`   VARCHAR(64)  NULL COMMENT '来源业务类型(order/alert/...)',
    `biz_id`     VARCHAR(64)  NULL COMMENT '来源业务 ID',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`message_id`),
    KEY `idx_message_user` (`user_id`, `is_read`, `created_at`),
    KEY `idx_message_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='站内信';

-- 通知模板（code UK；渲染占位 {{key}}）。
CREATE TABLE IF NOT EXISTS `notify_template`
(
    `template_id`      BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`             VARCHAR(64)  NOT NULL COMMENT '模板编码(租户内唯一,事件 template_code 对齐)',
    `title_template`   VARCHAR(255) NOT NULL COMMENT '标题模板',
    `content_template` VARCHAR(2048) NOT NULL COMMENT '内容模板',
    `channel`          VARCHAR(32)  NOT NULL DEFAULT 'INBOX' COMMENT 'INBOX 站内信/SMS 短信(Provider 留位)',
    `status`           TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用/2 停用',
    `tenant_id`        BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`       BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`       BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`       DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`template_id`),
    UNIQUE KEY `uk_template_tenant_code` (`tenant_id`, `code`),
    KEY `idx_template_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='通知模板';

-- 渠道配置（Provider 化：dev=log 桩；sms 上线前必切云 Provider，04 §2）。
CREATE TABLE IF NOT EXISTS `notify_channel`
(
    `channel_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`       VARCHAR(32)  NOT NULL COMMENT '渠道编码(INBOX/SMS/EMAIL/WECHAT)',
    `provider`   VARCHAR(64)  NOT NULL DEFAULT 'log' COMMENT 'Provider 实现(log 桩/aliyun-sms/...)',
    `config`     VARCHAR(1024) NULL COMMENT 'Provider 配置 JSON(密钥经 KMS 预留)',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用/2 停用',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`channel_id`),
    UNIQUE KEY `uk_channel_tenant_code` (`tenant_id`, `code`),
    KEY `idx_channel_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='通知渠道';

-- 外发流水（渠道侧投递结果；站内信不走外发，SMS/EMAIL 记录）。
CREATE TABLE IF NOT EXISTS `outbound_log`
(
    `outbound_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `message_id`  BIGINT       NULL COMMENT '关联站内信(同模板同参时)',
    `channel`     VARCHAR(32)  NOT NULL COMMENT '渠道(SMS/EMAIL/WECHAT)',
    `provider`    VARCHAR(64)  NOT NULL COMMENT 'Provider 实现',
    `target`      VARCHAR(255) NOT NULL COMMENT '收件目标(手机号/邮箱/openid)',
    `status`      VARCHAR(16)  NOT NULL DEFAULT 'SENT' COMMENT 'SENT/FAILED',
    `error`       VARCHAR(512) NULL COMMENT '失败原因',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`outbound_id`),
    KEY `idx_outbound_message` (`message_id`),
    KEY `idx_outbound_tenant` (`tenant_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='外发流水';

-- 用户通知设置（免打扰/频控偏好，FR-NTF-002/004）。
CREATE TABLE IF NOT EXISTS `user_notify_setting`
(
    `setting_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `user_id`       BIGINT       NOT NULL COMMENT '用户 uid',
    `template_code` VARCHAR(64)  NOT NULL DEFAULT '*' COMMENT '模板编码(*=全局默认)',
    `enabled`       TINYINT      NOT NULL DEFAULT 1 COMMENT '1 接收/0 静默',
    `quiet_hours`   VARCHAR(128) NULL COMMENT '免打扰时段 JSON {"start":"22:00","end":"08:00"}',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`setting_id`),
    UNIQUE KEY `uk_setting_user_code` (`tenant_id`, `user_id`, `template_code`),
    KEY `idx_setting_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='用户通知设置';
