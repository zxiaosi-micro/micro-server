-- finance_db 初始 schema（S5-03，v4/03 阶段 5）
-- 支付单（三渠道 + REVIEWING 双人复核中间态）/退款/发票/对账任务/返利预留（FR-FIN-001~006）。

-- 支付单（payment_no UK + channel_txn_id UK；对公复核职责分离：created_by ≠ approved_by）。
CREATE TABLE IF NOT EXISTS `payment`
(
    `payment_id`      BIGINT         NOT NULL COMMENT '雪花 ID',
    `payment_no`      VARCHAR(64)    NOT NULL COMMENT '支付单号',
    `order_no`        VARCHAR(64)    NOT NULL COMMENT '业务订单号（order_paid 事件 key）',
    `channel`         VARCHAR(16)    NOT NULL COMMENT 'WECHAT/ALIPAY/BANK_OFFLINE',
    `status`          VARCHAR(16)    NOT NULL DEFAULT 'PAYING' COMMENT 'PAYING/REVIEWING/PAID/REJECTED/SETTLED/CLOSED',
    `amount`          DECIMAL(18, 2) NOT NULL COMMENT '应收金额',
    `paid_amount`     DECIMAL(18, 2) NULL COMMENT '实收金额（不一致 → 差异对账任务）',
    `channel_txn_id`  VARCHAR(128)   NULL COMMENT '渠道流水号（UK；对公录入为空，回调验签后回填）',
    `payer_party_id`  BIGINT         NULL COMMENT '付款方',
    `created_by`      BIGINT         NULL COMMENT '录入人（对公）',
    `approved_by`     BIGINT         NULL COMMENT '复核人（第二人，职责分离）',
    `approve_remark`  VARCHAR(512)   NULL COMMENT '复核意见',
    `pay_params`      VARCHAR(1024)  NULL COMMENT '渠道透传参数 JSON（dev mock URL）',
    `remark`          VARCHAR(512)   NULL,
    `paid_at`         DATETIME(3)    NULL COMMENT '支付完成时刻',
    `tenant_id`       BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`      DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`payment_id`),
    UNIQUE KEY `uk_payment_tenant_no` (`tenant_id`, `payment_no`),
    UNIQUE KEY `uk_payment_channel_txn` (`channel_txn_id`),
    KEY `idx_payment_order` (`order_no`),
    KEY `idx_payment_status` (`tenant_id`, `status`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='支付单';

-- 退款单（原路退回；幂等键 payment_no+return_no，FR-FIN-002）。
CREATE TABLE IF NOT EXISTS `refund`
(
    `refund_id`         BIGINT         NOT NULL COMMENT '雪花 ID',
    `refund_no`         VARCHAR(64)    NOT NULL COMMENT '退款单号',
    `payment_no`        VARCHAR(64)    NOT NULL COMMENT '原支付单（原路退回锚点）',
    `order_no`          VARCHAR(64)    NOT NULL COMMENT '业务订单号',
    `return_no`         VARCHAR(64)    NULL COMMENT '触发退货单（退货审批联动必传）',
    `amount`            DECIMAL(18, 2) NOT NULL COMMENT '退款金额',
    `channel`           VARCHAR(16)    NOT NULL COMMENT '原支付渠道（原路退回）',
    `status`            VARCHAR(16)    NOT NULL DEFAULT 'PROCESSING' COMMENT 'PROCESSING/SUCCESS/FAILED',
    `channel_refund_id` VARCHAR(128)   NULL COMMENT '渠道退款流水',
    `reason`            VARCHAR(512)   NULL,
    `tenant_id`         BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`        DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`        BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`        DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`        BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`        DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`refund_id`),
    UNIQUE KEY `uk_refund_tenant_no` (`tenant_id`, `refund_no`),
    UNIQUE KEY `uk_refund_idem` (`payment_no`, `return_no`),
    KEY `idx_refund_status` (`tenant_id`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='退款单';

-- 电子发票（ISSUED/REVERSED 状态机桩，FR-FIN-004；生产对接税控服务商）。
CREATE TABLE IF NOT EXISTS `invoice`
(
    `invoice_id`     BIGINT         NOT NULL COMMENT '雪花 ID',
    `invoice_no`     VARCHAR(64)    NOT NULL COMMENT '发票号',
    `payment_no`     VARCHAR(64)    NOT NULL COMMENT '关联支付单',
    `order_no`       VARCHAR(64)    NOT NULL COMMENT '关联订单',
    `title`          VARCHAR(255)   NOT NULL COMMENT '抬头',
    `tax_no`         VARCHAR(64)    NULL COMMENT '税号',
    `amount`         DECIMAL(18, 2) NOT NULL COMMENT '开票金额',
    `status`         VARCHAR(16)    NOT NULL DEFAULT 'ISSUED' COMMENT 'ISSUED/REVERSED',
    `reverse_reason` VARCHAR(512)   NULL COMMENT '红冲原因',
    `reversed_at`    DATETIME(3)    NULL,
    `tenant_id`      BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`     BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`invoice_id`),
    UNIQUE KEY `uk_invoice_tenant_no` (`tenant_id`, `invoice_no`),
    KEY `idx_invoice_payment` (`payment_no`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='电子发票';

-- 对账任务（FR-FIN-003：差异自动标记；修复只允许补投递重放，E10）。
CREATE TABLE IF NOT EXISTS `reconcile_task`
(
    `task_id`        BIGINT       NOT NULL COMMENT '雪花 ID',
    `task_no`        VARCHAR(64)  NOT NULL COMMENT '任务号',
    `type`           VARCHAR(16)  NOT NULL COMMENT 'PAYMENT/REFUND（资金日结）',
    `biz_date`       VARCHAR(10)  NOT NULL COMMENT '业务日期 yyyy-MM-dd',
    `diff_report`    JSON         NULL COMMENT '差异报告（渠道账单 vs 系统流水）',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'OPEN' COMMENT 'OPEN/RESOLVED',
    `resolution`     VARCHAR(16)  NULL COMMENT 'REPLAY（补投递重放）/IGNORE',
    `resolve_remark` VARCHAR(512) NULL,
    `resolved_by`    BIGINT       NULL,
    `resolved_at`    DATETIME(3)  NULL,
    `tenant_id`      BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`     BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`task_id`),
    UNIQUE KEY `uk_recon_tenant_no` (`tenant_id`, `task_no`),
    UNIQUE KEY `uk_recon_biz` (`tenant_id`, `type`, `biz_date`),
    KEY `idx_recon_status` (`status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='对账任务';

-- 经销商返利结算（L3 预留表，FR-FIN-006：规则快照可追溯）。
CREATE TABLE IF NOT EXISTS `rebate_settlement`
(
    `settlement_id` BIGINT         NOT NULL COMMENT '雪花 ID',
    `period`        VARCHAR(7)     NOT NULL COMMENT '结算周期 yyyy-MM',
    `party_id`      BIGINT         NOT NULL COMMENT '经销商',
    `rule_snapshot` JSON           NULL COMMENT '返利规则快照（结算金额可追溯）',
    `amount`        DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '返利金额',
    `status`        VARCHAR(16)    NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/CONFIRMED/PAID',
    `tenant_id`     BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT         NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`settlement_id`),
    UNIQUE KEY `uk_rebate_period_party` (`tenant_id`, `period`, `party_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='返利结算(L3 预留)';

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
