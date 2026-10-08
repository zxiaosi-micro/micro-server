-- catalog_db 初始 schema（S4-02，v4/03 阶段 4）
-- 规约（02 §6.1/§6.2）：雪花 BIGINT 主键；全表六通用字段；状态/枚举 VARCHAR；金额 DECIMAL。

-- 商品（SPU）。
CREATE TABLE IF NOT EXISTS `product`
(
    `product_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `name`       VARCHAR(128) NOT NULL COMMENT '商品名称',
    `category`   VARCHAR(64)  NULL COMMENT '分类',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1 上架/2 下架',
    `remark`     VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`product_id`),
    KEY `idx_product_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='商品';

-- SKU：type=STANDARD/EXT_WARRANTY——延保也是 SKU，下单链路统一（01 FR-CTL-002/006）。
CREATE TABLE IF NOT EXISTS `sku`
(
    `sku_id`     BIGINT       NOT NULL COMMENT '雪花 ID',
    `product_id` BIGINT       NOT NULL COMMENT '归属商品',
    `code`       VARCHAR(64)  NOT NULL COMMENT 'SKU 编码(租户内唯一)',
    `name`       VARCHAR(128) NOT NULL COMMENT 'SKU 名称',
    `type`       VARCHAR(32)  NOT NULL DEFAULT 'STANDARD' COMMENT 'STANDARD 标准品/EXT_WARRANTY 延保',
    `spec`       VARCHAR(512) NULL COMMENT '规格参数 JSON',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1 上架/2 下架',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`sku_id`),
    UNIQUE KEY `uk_sku_tenant_code` (`tenant_id`, `code`),
    KEY `idx_sku_product` (`product_id`),
    KEY `idx_sku_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='SKU';

-- 价格：RETAIL/DEALER/TIER + tier_qty 阶梯量；version 版本化留痕——SetPrice 插入新版本行，
-- 历史不删改（01 FR-CTL-004"版本化留痕"）；生效版本 = 同键内 version 最大。
CREATE TABLE IF NOT EXISTS `price`
(
    `price_id`   BIGINT         NOT NULL COMMENT '雪花 ID',
    `sku_id`     BIGINT         NOT NULL COMMENT '归属 SKU',
    `price_type` VARCHAR(32)    NOT NULL COMMENT 'RETAIL 零售价/DEALER 经销商价/TIER 阶梯价',
    `tier_qty`   INT            NOT NULL DEFAULT 0 COMMENT '阶梯起量(仅 TIER 有意义)',
    `amount`     DECIMAL(14, 2) NOT NULL COMMENT '价格(元)',
    `version`    INT            NOT NULL DEFAULT 1 COMMENT '版本号(同 sku+type+tier_qty 内自增)',
    `tenant_id`  BIGINT         NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT         NULL COMMENT '创建人 uid',
    `updated_at` DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT         NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME       NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`price_id`),
    KEY `idx_price_sku` (`sku_id`, `price_type`, `tier_qty`, `version`),
    KEY `idx_price_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='价格(版本化)';

-- 质保策略：按 SKU 配置；start_rule 起算规则由合同域执行时快照（01 FR-CTL-005）。
CREATE TABLE IF NOT EXISTS `warranty_policy`
(
    `policy_id`     BIGINT      NOT NULL COMMENT '雪花 ID',
    `sku_id`        BIGINT      NOT NULL COMMENT '归属 SKU',
    `period_months` INT         NOT NULL COMMENT '质保期(月)',
    `start_rule`    VARCHAR(32) NOT NULL DEFAULT 'ACTIVATION' COMMENT 'ACTIVATION 激活日/RECEIPT 签收日',
    `tenant_id`     BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`policy_id`),
    UNIQUE KEY `uk_policy_sku` (`sku_id`),
    KEY `idx_policy_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='质保策略';

-- 场站模板（BOM 头）。
CREATE TABLE IF NOT EXISTS `station_product`
(
    `station_product_id` BIGINT       NOT NULL COMMENT '雪花 ID',
    `name`               VARCHAR(128) NOT NULL COMMENT '场站模板名称',
    `remark`             VARCHAR(255) NULL COMMENT '备注',
    `tenant_id`          BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`         BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`         BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`         DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`station_product_id`),
    KEY `idx_station_product_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站模板';

-- 场站模板 BOM 明细。
CREATE TABLE IF NOT EXISTS `station_product_item`
(
    `item_id`            BIGINT      NOT NULL COMMENT '雪花 ID',
    `station_product_id` BIGINT      NOT NULL COMMENT '归属场站模板',
    `sku_id`             BIGINT      NOT NULL COMMENT 'SKU(逻辑引用 catalog.sku)',
    `qty`                INT         NOT NULL DEFAULT 1 COMMENT '数量',
    `tenant_id`          BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`         BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`         BIGINT      NULL COMMENT '更新人 uid',
    `deleted_at`         DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`item_id`),
    KEY `idx_sp_item_station` (`station_product_id`),
    KEY `idx_sp_item_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站模板 BOM 明细';
