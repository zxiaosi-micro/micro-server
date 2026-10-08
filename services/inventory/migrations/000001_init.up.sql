-- inventory_db 初始 schema（S4-03 ★核心，v4/03 阶段 4）
-- 规约（02 §6.1/§6.2）：雪花 BIGINT 主键；全表六通用字段；四态库存 + 流水 + 盘点。

-- 仓库。
CREATE TABLE IF NOT EXISTS `warehouse`
(
    `warehouse_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`         VARCHAR(64)  NOT NULL COMMENT '仓库编码(租户内唯一)',
    `name`         VARCHAR(128) NOT NULL COMMENT '仓库名称',
    `address`      VARCHAR(255) NULL COMMENT '仓库地址',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '1 启用/2 停用',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`warehouse_id`),
    UNIQUE KEY `uk_warehouse_tenant_code` (`tenant_id`, `code`),
    KEY `idx_warehouse_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='仓库';

-- 库存：warehouse_id+sku_id 唯一；四态 = 可用/锁定/在途/残次（01 FR-INV-002）+ 低库存阈值。
CREATE TABLE IF NOT EXISTS `inventory`
(
    `inventory_id`        BIGINT   NOT NULL COMMENT '雪花 ID',
    `warehouse_id`        BIGINT   NOT NULL COMMENT '仓库 ID',
    `sku_id`              BIGINT   NOT NULL COMMENT 'SKU(逻辑引用 catalog.sku)',
    `available`           INT      NOT NULL DEFAULT 0 COMMENT '可用库存',
    `locked`              INT      NOT NULL DEFAULT 0 COMMENT '锁定库存(已预留未出库)',
    `in_transit`          INT      NOT NULL DEFAULT 0 COMMENT '在途库存(调拨/采购在途)',
    `defective`           INT      NOT NULL DEFAULT 0 COMMENT '残次库存',
    `low_stock_threshold` INT      NOT NULL DEFAULT 0 COMMENT '低库存预警阈值',
    `tenant_id`           BIGINT   NOT NULL COMMENT '租户 ID',
    `created_at`          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`          BIGINT   NULL COMMENT '创建人 uid',
    `updated_at`          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`          BIGINT   NULL COMMENT '更新人 uid',
    `deleted_at`          DATETIME NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`inventory_id`),
    UNIQUE KEY `uk_inventory_wh_sku` (`warehouse_id`, `sku_id`),
    KEY `idx_inventory_tenant` (`tenant_id`),
    KEY `idx_inventory_sku` (`sku_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='库存(四态)';

-- 库存流水：biz_type+biz_no 溯源 + 四态 before/after 快照；
-- uk_stock_record_biz = 幂等键（(biz_type,biz_no) 唯一，1062 → ErrTxnReplay，02 §9.2）。
CREATE TABLE IF NOT EXISTS `stock_record`
(
    `record_id`        BIGINT       NOT NULL COMMENT '雪花 ID',
    `inventory_id`     BIGINT       NOT NULL COMMENT '库存行 ID',
    `warehouse_id`     BIGINT       NOT NULL COMMENT '仓库 ID(冗余,溯源)',
    `sku_id`           BIGINT       NOT NULL COMMENT 'SKU(冗余,溯源)',
    `biz_type`         VARCHAR(32)  NOT NULL COMMENT 'RESERVE 预留/RELEASE 释放/DEDUCT 出库/STOCK_IN 入库/SPARE_OUT 备件领用/SPARE_RETURN 备件退库/STOCKTAKE_ADJUST 盘点调整',
    `biz_no`           VARCHAR(64)  NOT NULL COMMENT '业务单号(幂等键组成部分)',
    `qty`              INT          NOT NULL COMMENT '变动数量(可用口径:正增负减)',
    `before_available` INT          NOT NULL DEFAULT 0 COMMENT '变动前可用',
    `after_available`  INT          NOT NULL DEFAULT 0 COMMENT '变动后可用',
    `before_locked`    INT          NOT NULL DEFAULT 0 COMMENT '变动前锁定',
    `after_locked`     INT          NOT NULL DEFAULT 0 COMMENT '变动后锁定',
    `remark`           VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`        BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`       BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`       BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`       DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`record_id`),
    UNIQUE KEY `uk_stock_record_biz` (`tenant_id`, `biz_type`, `biz_no`),
    KEY `idx_stock_record_inv` (`inventory_id`),
    KEY `idx_stock_record_wh` (`warehouse_id`, `created_at`),
    KEY `idx_stock_record_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='库存流水';

-- 盘点单：DRAFT(1)→SUBMITTED(2,实盘提交)→APPROVED(3,账面调整)/REJECTED(4)。
CREATE TABLE IF NOT EXISTS `stocktake`
(
    `stocktake_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `warehouse_id`  BIGINT       NOT NULL COMMENT '盘点仓库',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1 草稿/2 已提交/3 已审批/4 已驳回',
    `remark`        VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`stocktake_id`),
    KEY `idx_stocktake_wh` (`warehouse_id`),
    KEY `idx_stocktake_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='盘点单';

-- 盘点明细：创建时快照账面数(book_qty)；提交实盘数；审批按差异调整账面（有流水）。
CREATE TABLE IF NOT EXISTS `stocktake_item`
(
    `item_id`      BIGINT   NOT NULL COMMENT '雪花 ID',
    `stocktake_id` BIGINT   NOT NULL COMMENT '盘点单 ID',
    `sku_id`       BIGINT   NOT NULL COMMENT 'SKU',
    `book_qty`     INT      NOT NULL DEFAULT 0 COMMENT '账面数(创建时快照可用库存)',
    `counted_qty`  INT      NULL COMMENT '实盘数(提交时填)',
    `diff_qty`     INT      NULL COMMENT '差异=实盘-账面(审批时计算)',
    `tenant_id`    BIGINT   NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT   NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT   NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`item_id`),
    KEY `idx_stocktake_item_st` (`stocktake_id`),
    KEY `idx_stocktake_item_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='盘点明细';

-- event_outbox 事件发件表（DDL 正本 micro-common/eventbus/schema.go；stock_out/stock_low
-- 事件与业务同事务写入，Relay 属 S5 落地）。
CREATE TABLE IF NOT EXISTS `event_outbox`
(
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`      VARCHAR(64)  NOT NULL COMMENT '事件唯一标识(去重/重投/对账键)',
    `event_type`    VARCHAR(128) NOT NULL COMMENT '事件类型(点号命名)',
    `topic`         VARCHAR(128) NOT NULL COMMENT '目标 topic(下划线命名)',
    `partition_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '分区保序键=聚合ID',
    `tenant_id`     BIGINT       NOT NULL DEFAULT 0 COMMENT '租户ID(消费侧恢复)',
    `trace_id`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '生产侧链路ID',
    `payload`       JSON         NOT NULL COMMENT '事件信封全文',
    `status`        VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/FAILED/DEAD',
    `retry_count`   INT          NOT NULL DEFAULT 0 COMMENT '投递重试次数',
    `next_retry_at` DATETIME(3)  NOT NULL COMMENT '下次投递时间(退避)',
    `last_error`    VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近一次投递错误',
    `created_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_dispatch` (`status`, `next_retry_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='事件发件箱(outbox)';
