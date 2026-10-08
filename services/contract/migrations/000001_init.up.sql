-- contract_db 初始 schema（S5-04，v4/03 阶段 5）
-- 合同模板/合同/归档件/质保（双层）/延保/SLA 策略/索赔（FR-CTR-001~008）。
-- 合同签署基线 = 线下签署 + 上传归档；文件本体在 file 服务合同桶，本库只存归档元数据（平台不解析内容）。

-- 合同模板（变量 + body；版本管理 FR-CTR-001，L2 打印稿辅助数据源）。
CREATE TABLE IF NOT EXISTS `contract_template`
(
    `template_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`           VARCHAR(64)  NOT NULL COMMENT '模板编码（租户内唯一）',
    `name`           VARCHAR(255) NOT NULL COMMENT '模板名称',
    `contract_type`  VARCHAR(16)  NOT NULL COMMENT 'SALES/STATION/SERVICE/FRAMEWORK/EXTENDED',
    `variables_json` JSON         NULL COMMENT '变量定义 [{key,label,required}]',
    `body`           TEXT         NOT NULL COMMENT '模板正文（{{key}} 占位）',
    `version`        INT          NOT NULL DEFAULT 1 COMMENT '模板版本（更新递增）',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE/DISABLED',
    `tenant_id`      BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`     BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`template_id`),
    UNIQUE KEY `uk_template_tenant_code` (`tenant_id`, `code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='合同模板';

-- 合同（草稿→生效→归档；类型 SALES/STATION/SERVICE/FRAMEWORK/EXTENDED）。
CREATE TABLE IF NOT EXISTS `contract`
(
    `contract_id`   BIGINT         NOT NULL COMMENT '雪花 ID',
    `contract_no`   VARCHAR(64)    NOT NULL COMMENT '合同号',
    `type`          VARCHAR(16)    NOT NULL COMMENT 'SALES/STATION/SERVICE/FRAMEWORK/EXTENDED',
    `status`        VARCHAR(16)    NOT NULL DEFAULT 'DRAFT' COMMENT 'DRAFT/ACTIVE/ARCHIVED',
    `name`          VARCHAR(255)   NOT NULL DEFAULT '' COMMENT '合同名称',
    `template_id`   BIGINT         NULL COMMENT '来源模板（0=无模板）',
    `amount`        DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '合同金额',
    `buyer_party_id` BIGINT        NULL COMMENT '买方参与方',
    `order_no`      VARCHAR(64)    NULL COMMENT '来源订单（Saga 步骤5 建立时回填）',
    `remark`        VARCHAR(512)   NULL,
    `effective_at`  DATETIME(3)    NULL COMMENT '生效时刻',
    `archived_at`   DATETIME(3)    NULL COMMENT '归档时刻',
    `tenant_id`     BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`contract_id`),
    UNIQUE KEY `uk_contract_tenant_no` (`tenant_id`, `contract_no`),
    KEY `idx_contract_order` (`order_no`),
    KEY `idx_contract_status` (`tenant_id`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='合同';

-- 合同关联（订单/客户/场站，归档五要素之「关联」）。
CREATE TABLE IF NOT EXISTS `contract_target`
(
    `target_rec_id` BIGINT      NOT NULL COMMENT '雪花 ID',
    `contract_id`   BIGINT      NOT NULL COMMENT '合同',
    `target_type`   VARCHAR(16) NOT NULL COMMENT 'ORDER/PARTY/STATION',
    `target_pk`     BIGINT      NOT NULL COMMENT '目标 ID',
    `target_no`     VARCHAR(64) NULL COMMENT '目标单号（展示/追溯）',
    `tenant_id`     BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`target_rec_id`),
    KEY `idx_ctarget_contract` (`contract_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='合同关联';

-- 合同归档件（归档五要素：上传人/版本/关联/状态/受控下载；文件本体在 file 合同桶）。
CREATE TABLE IF NOT EXISTS `contract_file`
(
    `file_rec_id`      BIGINT       NOT NULL COMMENT '雪花 ID',
    `contract_id`      BIGINT       NOT NULL COMMENT '所属合同',
    `file_id`          VARCHAR(64)  NOT NULL COMMENT 'file 服务对象 ID（合同桶，受控下载）',
    `file_name`        VARCHAR(255) NOT NULL COMMENT '文件名',
    `version`          INT          NOT NULL COMMENT '合同内版本号（重签递增）',
    `sign_party_name`  VARCHAR(255) NULL COMMENT '签署方名称',
    `sign_party_type`  VARCHAR(16)  NOT NULL DEFAULT 'BUYER' COMMENT 'BUYER/SELLER/OTHER',
    `status`           VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE/REPLACED',
    `uploaded_by`      BIGINT       NOT NULL COMMENT '上传人（审计要素）',
    `uploader_name`    VARCHAR(64)  NULL COMMENT '上传人姓名（展示）',
    `remark`           VARCHAR(512) NULL,
    `tenant_id`        BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`       BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`       BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`       DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`file_rec_id`),
    UNIQUE KEY `uk_cfile_contract_version` (`contract_id`, `version`),
    KEY `idx_cfile_contract` (`contract_id`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='合同归档件';

-- 质保（STATION/DEVICE 双层 + start_rule 快照；起算由事件驱动，FR-CTR-005）。
CREATE TABLE IF NOT EXISTS `warranty`
(
    `warranty_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `warranty_no`  VARCHAR(64)  NOT NULL COMMENT '质保编号',
    `level`        VARCHAR(16)  NOT NULL COMMENT 'STATION/DEVICE 双层',
    `target_type`  VARCHAR(16)  NOT NULL COMMENT 'STATION/DEVICE',
    `target_id`    BIGINT       NOT NULL DEFAULT 0 COMMENT '目标 ID（SN 起算时 device_id S6 回填）',
    `target_key`   VARCHAR(64)  NOT NULL COMMENT '目标键（SN / 场站编号）',
    `status`       VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/ACTIVE/EXPIRED/REFUNDED',
    `start_at`     DATETIME(3)  NULL COMMENT '起算时刻（事件驱动写入）',
    `end_at`       DATETIME(3)  NULL COMMENT '到期时刻（start_at + months）',
    `months`       INT          NOT NULL DEFAULT 0 COMMENT '质保月数',
    `start_rule`   JSON         NULL COMMENT '起算规则快照 {"event":"shipment_signed","months":24}',
    `source_type`  VARCHAR(16)  NOT NULL DEFAULT 'ORDER' COMMENT 'ORDER/EXTENSION',
    `source_no`    VARCHAR(64)  NULL COMMENT '来源单号',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`warranty_id`),
    UNIQUE KEY `uk_warranty_tenant_no` (`tenant_id`, `warranty_no`),
    KEY `idx_warranty_target` (`target_type`, `target_id`, `target_key`),
    -- 到期扫描索引（E16：质保到期 cron 扫描）
    KEY `idx_warranty_expiry_scan` (`status`, `end_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='质保';

-- 延保（销售/衔接/转移/退款，FR-CTR-006）。
CREATE TABLE IF NOT EXISTS `warranty_extension`
(
    `extension_id`    BIGINT         NOT NULL COMMENT '雪花 ID',
    `extension_no`    VARCHAR(64)    NOT NULL COMMENT '延保单号',
    `base_warranty_id` BIGINT        NOT NULL COMMENT '衔接的原质保',
    `months`          INT            NOT NULL COMMENT '延长月数',
    `order_no`        VARCHAR(64)    NULL COMMENT '延保销售订单',
    `amount`          DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '销售金额',
    `status`          VARCHAR(16)    NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE/TRANSFERRED/REFUNDED',
    `to_target_type`  VARCHAR(16)    NULL COMMENT '转移目标类型',
    `to_target_id`    BIGINT         NULL,
    `to_target_key`   VARCHAR(64)    NULL,
    `refund_no`       VARCHAR(64)    NULL COMMENT '退款单号（退款联动 finance）',
    `tenant_id`       BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`      BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`      BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`      DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`extension_id`),
    UNIQUE KEY `uk_ext_tenant_no` (`tenant_id`, `extension_no`),
    KEY `idx_ext_base` (`base_warranty_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='延保';

-- SLA 策略（响应/解决时限分级；计时由 ops 执行，FR-CTR-007）。
CREATE TABLE IF NOT EXISTS `sla_strategy`
(
    `strategy_id`      BIGINT      NOT NULL COMMENT '雪花 ID',
    `code`             VARCHAR(64) NOT NULL COMMENT '策略编码（租户内唯一）',
    `name`             VARCHAR(255) NOT NULL COMMENT '策略名称',
    `level`            VARCHAR(16) NOT NULL COMMENT 'P1/P2/P3 分级',
    `response_minutes` INT         NOT NULL COMMENT '响应时限（分钟）',
    `resolve_minutes`  INT         NOT NULL COMMENT '解决时限（分钟）',
    `tenant_id`        BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`       BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`       BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`       DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`strategy_id`),
    UNIQUE KEY `uk_sla_tenant_code` (`tenant_id`, `code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='SLA 策略';

-- 合同 SLA 绑定（绑定即快照冻结，ops 按快照计时）。
CREATE TABLE IF NOT EXISTS `contract_sla`
(
    `bind_id`     BIGINT      NOT NULL COMMENT '雪花 ID',
    `contract_id` BIGINT      NOT NULL COMMENT '合同',
    `strategy_id` BIGINT      NOT NULL COMMENT '策略',
    `snapshot`    JSON        NOT NULL COMMENT '策略快照（冻结）',
    `tenant_id`   BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`bind_id`),
    UNIQUE KEY `uk_csla_contract` (`contract_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='合同 SLA 绑定';

-- 索赔（申请/审核/结算；结算联动 finance，FR-CTR-008）。
CREATE TABLE IF NOT EXISTS `claim`
(
    `claim_id`    BIGINT         NOT NULL COMMENT '雪花 ID',
    `claim_no`    VARCHAR(64)    NOT NULL COMMENT '索赔单号',
    `warranty_id` BIGINT         NOT NULL COMMENT '关联质保',
    `warranty_no` VARCHAR(64)    NOT NULL COMMENT '质保编号',
    `type`        VARCHAR(16)    NOT NULL COMMENT 'QUALITY/TRANSPORT/INSTALL',
    `description` VARCHAR(1024)  NULL COMMENT '描述',
    `status`      VARCHAR(16)    NOT NULL DEFAULT 'APPLYING' COMMENT 'APPLYING/APPROVED/REJECTED/SETTLED',
    `settle_type` VARCHAR(16)    NULL COMMENT 'REPAIR/REPLACE/REFUND/PAID',
    `amount`      DECIMAL(18, 2) NULL COMMENT '结算金额（PAID/REFUND）',
    `reject_remark` VARCHAR(512) NULL,
    `settle_at`   DATETIME(3)    NULL,
    `tenant_id`   BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`claim_id`),
    UNIQUE KEY `uk_claim_tenant_no` (`tenant_id`, `claim_no`),
    KEY `idx_claim_warranty` (`warranty_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='索赔';

-- Outbox 事件表（eventbus/schema.go SchemaOutbox 同口径）。
CREATE TABLE IF NOT EXISTS `event_outbox`
(
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`      VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    `topic`         VARCHAR(128) NOT NULL COMMENT '目标 topic',
    `event_type`    VARCHAR(128) NOT NULL COMMENT '事件类型（点号命名）',
    `partition_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '分区保序键=聚合ID',
    `tenant_id`     BIGINT      NOT NULL DEFAULT 0 COMMENT '租户 ID',
    `trace_id`      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪',
    `payload`       JSON        NOT NULL COMMENT '事件载荷',
    `status`        VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/RETRY/FAILED/DEAD',
    `retry_count`   INT         NOT NULL DEFAULT 0,
    `next_retry_at` DATETIME(3) NULL,
    `last_error`    VARCHAR(512) NOT NULL DEFAULT '',
    `created_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_outbox_dispatch` (`status`, `next_retry_at`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='事件发件箱';
