-- order_db 初始 schema（S5-02，v4/03 阶段 5）
-- 订单/订单项（单价快照）/Saga 状态机/发运轨迹/退货单（FR-ORD-001~006）。
-- 状态一律 VARCHAR 枚举（加枚举零 DDL）；金额 DECIMAL；雪花 BIGINT 主键；软删 + 租户。

-- 销售订单（type=DEVICE/STATION/PURCHASE/RETURN；pay_expire_at 为支付超时扫描列，ADR-09）。
CREATE TABLE IF NOT EXISTS `order`
(
    `order_id`       BIGINT        NOT NULL COMMENT '雪花 ID',
    `order_no`       VARCHAR(64)   NOT NULL COMMENT '订单号（幂等键 Saga 步骤1）',
    `type`           VARCHAR(16)   NOT NULL DEFAULT 'DEVICE' COMMENT 'DEVICE 设备/STATION 场站/PURCHASE 采购/RETURN 退货',
    `status`         VARCHAR(16)   NOT NULL DEFAULT 'CREATED' COMMENT 'CREATED/LOCKED/PAYING/PAID/STOCK_OUT/CONTRACTED/DONE/CANCELLED/PAY_TIMEOUT',
    `buyer_party_id` BIGINT        NULL COMMENT '买方参与方',
    `total_amount`   DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '总额（快照价汇总）',
    `pay_expire_at`  DATETIME(3)   NULL COMMENT '支付超时时刻（到期 cron 扫描 → 取消+释放库存，FR-ORD-006）',
    `paid_at`        DATETIME(3)   NULL COMMENT '支付完成时刻（order_paid 事件驱动回写）',
    `cancel_reason`  VARCHAR(512)  NULL COMMENT '取消/超时原因',
    `remark`         VARCHAR(512)  NULL,
    `tenant_id`      BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     BIGINT        NULL COMMENT '创建人 uid',
    `updated_at`     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`     BIGINT        NULL COMMENT '更新人 uid',
    `deleted_at`     DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`order_id`),
    UNIQUE KEY `uk_order_tenant_no` (`tenant_id`, `order_no`),
    -- 支付超时扫描索引（E16：扫描 SQL 必须命中索引，拖库是大忌）
    KEY `idx_order_pay_scan` (`pay_expire_at`, `status`),
    KEY `idx_order_tenant_status` (`tenant_id`, `status`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='销售订单';

-- 订单项（单价快照：下单时取 catalog.ListPrice latest，FR-ORD-001）。
CREATE TABLE IF NOT EXISTS `order_item`
(
    `item_id`      BIGINT        NOT NULL COMMENT '雪花 ID',
    `order_id`     BIGINT        NOT NULL COMMENT '所属订单',
    `sku_id`       BIGINT        NOT NULL COMMENT 'SKU',
    `sku_name`     VARCHAR(255)  NOT NULL DEFAULT '' COMMENT 'SKU 名称快照',
    `sn`           VARCHAR(64)   NULL COMMENT '设备 SN（DEVICE 单）',
    `warehouse_id` BIGINT        NOT NULL COMMENT '出库仓（Reserve/DeductLocked 指定）',
    `qty`          INT           NOT NULL COMMENT '数量',
    `unit_price`   DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '单价快照',
    `amount`       DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '小计',
    `out_qty`      INT           NOT NULL DEFAULT 0 COMMENT '已出库确认数（stock_out 回写，per (order,sku) 幂等）',
    `tenant_id`    BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT        NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT        NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`item_id`),
    KEY `idx_order_item_order` (`order_id`),
    KEY `idx_order_item_sku` (`tenant_id`, `sku_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='订单项';

-- Saga 状态机（02 §9.1：step/status/retry/next_retry_at/context_json；cron 扫描推进，ADR-09）。
CREATE TABLE IF NOT EXISTS `saga`
(
    `saga_id`      BIGINT       NOT NULL COMMENT '全局事务 ID（进日志与审计）',
    `order_id`     BIGINT       NOT NULL COMMENT '关联订单',
    `order_no`     VARCHAR(64)  NOT NULL COMMENT '关联订单号（事件 key/日志关联）',
    `order_type`   VARCHAR(16)  NOT NULL COMMENT '订单类型（决定是否含步骤6）',
    `current_step` TINYINT      NOT NULL DEFAULT 1 COMMENT '当前步骤 1~6（02 §9.1 补偿表）',
    `status`       VARCHAR(16)  NOT NULL DEFAULT 'RUNNING' COMMENT 'RUNNING/COMPENSATING/DONE/CANCELLED/MANUAL',
    `retry_count`  INT          NOT NULL DEFAULT 0 COMMENT '当前步骤已重试次数（退避 1m/5m/30m，超限→MANUAL）',
    `next_retry_at` DATETIME(3) NULL COMMENT '下次推进时刻（重试扫描列）',
    `last_error`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近失败原因（人工队列排查入口）',
    `context_json` JSON         NULL COMMENT '补偿上下文（库存锁键/支付单号/合同号等原始业务标识）',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`saga_id`),
    UNIQUE KEY `uk_saga_tenant_order` (`tenant_id`, `order_id`),
    -- 重试推进扫描索引（E16）
    KEY `idx_saga_retry_scan` (`status`, `next_retry_at`),
    KEY `idx_saga_tenant_status` (`tenant_id`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='Saga 编排状态机';

-- 发货单（FR-ORD-005：轨迹手工录入基线）。
CREATE TABLE IF NOT EXISTS `shipment`
(
    `shipment_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `shipment_no`  VARCHAR(64)  NOT NULL COMMENT '发货单号',
    `order_id`     BIGINT       NOT NULL COMMENT '关联订单',
    `order_no`     VARCHAR(64)  NOT NULL COMMENT '关联订单号',
    `warehouse_id` BIGINT       NOT NULL COMMENT '发货仓',
    `carrier`      VARCHAR(64)  NULL COMMENT '承运方',
    `tracking_no`  VARCHAR(64)  NULL COMMENT '运单号',
    `status`       VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/IN_TRANSIT/SIGNED',
    `signed_by`    VARCHAR(64)  NULL COMMENT '签收人',
    `signed_at`    DATETIME(3)  NULL COMMENT '签收时刻（触发 shipment_signed 事件）',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`shipment_id`),
    UNIQUE KEY `uk_shipment_tenant_no` (`tenant_id`, `shipment_no`),
    KEY `idx_shipment_order` (`order_no`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='发货单';

-- 物流轨迹（手工录入基线，第三方 API 为 L3 可选）。
CREATE TABLE IF NOT EXISTS `shipment_trace`
(
    `trace_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `shipment_id` BIGINT       NOT NULL COMMENT '发货单',
    `node`        VARCHAR(128) NOT NULL COMMENT '轨迹节点',
    `description` VARCHAR(512) NULL COMMENT '描述',
    `trace_time`  DATETIME(3)  NOT NULL COMMENT '轨迹时刻',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`trace_id`),
    KEY `idx_trace_shipment` (`shipment_id`, `trace_time`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='物流轨迹';

-- 发货单明细（部分发货基线；签收事件按 SN 驱动质保起算）。
CREATE TABLE IF NOT EXISTS `shipment_item`
(
    `item_id`     BIGINT      NOT NULL COMMENT '雪花 ID',
    `shipment_id` BIGINT      NOT NULL COMMENT '发货单',
    `sku_id`      BIGINT      NOT NULL COMMENT 'SKU',
    `sn`          VARCHAR(64) NULL COMMENT '设备 SN',
    `qty`         INT         NOT NULL COMMENT '数量',
    `tenant_id`   BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`item_id`),
    KEY `idx_shipment_item` (`shipment_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='发货单明细';

-- 退货单（FR-ORD-004：申请/审核/退款联动；退款事件回写状态）。
CREATE TABLE IF NOT EXISTS `return_order`
(
    `return_id`   BIGINT       NOT NULL COMMENT '雪花 ID',
    `return_no`   VARCHAR(64)  NOT NULL COMMENT '退货单号（order_return_approved 事件 key）',
    `order_id`    BIGINT       NOT NULL COMMENT '原销售单',
    `order_no`    VARCHAR(64)  NOT NULL COMMENT '原销售单号',
    `reason`      VARCHAR(512) NULL COMMENT '退货原因',
    `status`      VARCHAR(16)  NOT NULL DEFAULT 'APPLYING' COMMENT 'APPLYING/APPROVED/REJECTED/REFUNDED',
    `refund_amount` DECIMAL(18, 2) NULL COMMENT '退款金额（按退货明细 × 单价快照计）',
    `refund_no`   VARCHAR(64)  NULL COMMENT '退款单号（payment_refunded 事件回写）',
    `reject_reason` VARCHAR(512) NULL COMMENT '驳回原因',
    `tenant_id`   BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`  BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`  DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`return_id`),
    UNIQUE KEY `uk_return_tenant_no` (`tenant_id`, `return_no`),
    KEY `idx_return_order` (`order_no`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='退货单';

-- 退货明细。
CREATE TABLE IF NOT EXISTS `return_item`
(
    `item_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `return_id`  BIGINT       NOT NULL COMMENT '退货单',
    `sku_id`     BIGINT       NOT NULL COMMENT 'SKU',
    `sn`         VARCHAR(64)  NULL COMMENT '设备 SN',
    `qty`        INT          NOT NULL COMMENT '数量',
    `reason`     VARCHAR(512) NULL COMMENT '明细原因',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`item_id`),
    KEY `idx_return_item` (`return_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='退货明细';

-- Outbox 事件表（eventbus/schema.go SchemaOutbox 同口径；与业务变更同事务写入，02 §9.3）。
CREATE TABLE IF NOT EXISTS `event_outbox`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`       VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    `topic`          VARCHAR(128) NOT NULL COMMENT '目标 topic',
    `event_type`     VARCHAR(128) NOT NULL COMMENT '事件类型（点号命名）',
    `partition_key`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '分区保序键=聚合ID',
    `tenant_id`      BIGINT      NOT NULL DEFAULT 0 COMMENT '租户 ID',
    `trace_id`       VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪',
    `payload`        JSON        NOT NULL COMMENT '事件载荷',
    `status`         VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/RETRY/FAILED/DEAD',
    `retry_count`    INT         NOT NULL DEFAULT 0,
    `next_retry_at`  DATETIME(3) NULL,
    `last_error`     VARCHAR(512) NOT NULL DEFAULT '',
    `created_at`     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`     DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_outbox_dispatch` (`status`, `next_retry_at`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='事件发件箱';
