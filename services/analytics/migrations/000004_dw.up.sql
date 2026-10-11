-- analytics_db S10-02 BI 自助 + 数仓：星型模型（2 事实 + 3 维度）+ ETL 批次台账。
-- 平台级聚合面：tenant_id 是分析维度而非隔离条件（BI 面向 admin 管理域）；
-- ETL INSERT..SELECT 跨库直读业务库（A1 应用账号有 micro_% 授权，对账 recon 同口径）。
-- 幂等口径：维 INSERT IGNORE / ODKU 快照，事实 PK(单号) ODKU——重跑同值 affected=0。

-- 日历维（ETL 按需补行，无租户概念）。
CREATE TABLE IF NOT EXISTS `dw_dim_date`
(
    `date_key`     DATE         NOT NULL COMMENT '日期键 yyyy-MM-dd',
    `year`         SMALLINT     NOT NULL COMMENT '年',
    `quarter`      TINYINT      NOT NULL COMMENT '季 1~4',
    `month`        TINYINT      NOT NULL COMMENT '月 1~12',
    `day_of_month` TINYINT      NOT NULL COMMENT '日 1~31',
    `week_of_year` TINYINT      NOT NULL COMMENT '年内周序',
    `weekday`      TINYINT      NOT NULL COMMENT '星期 1=周一…7=周日',
    `is_weekend`   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否周末',
    PRIMARY KEY (`date_key`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓日历维';

-- 租户维（事实源 DISTINCT 提取；first_seen 口径）。
CREATE TABLE IF NOT EXISTS `dw_dim_tenant`
(
    `tenant_id`     BIGINT   NOT NULL COMMENT '租户 ID',
    `first_seen_at` DATETIME NOT NULL COMMENT '首次出现在事实源的时刻',
    PRIMARY KEY (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓租户维';

-- 参与方维（party 快照 ODKU，重跑幂等刷新）。
CREATE TABLE IF NOT EXISTS `dw_dim_party`
(
    `party_id`   BIGINT       NOT NULL COMMENT '参与方',
    `tenant_id`  BIGINT       NOT NULL COMMENT '归属租户',
    `name`       VARCHAR(255) NOT NULL COMMENT '名称快照',
    `types`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '类型 JSON 快照',
    `region`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '区域快照',
    `updated_at` DATETIME     NOT NULL COMMENT '源更新时刻',
    PRIMARY KEY (`party_id`),
    KEY `idx_dwdp_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓参与方维';

-- 订单事实（PK=order_no：ODKU 重跑幂等；date_key=下单日；状态/金额随源刷新）。
CREATE TABLE IF NOT EXISTS `dw_fact_order`
(
    `order_no`         VARCHAR(64)    NOT NULL COMMENT '业务订单号',
    `date_key`         DATE           NOT NULL COMMENT '下单日期',
    `tenant_id`        BIGINT         NOT NULL COMMENT '租户（分析维度）',
    `buyer_party_id`   BIGINT         NOT NULL DEFAULT 0 COMMENT '买方参与方（0=散单）',
    `status`           VARCHAR(16)    NOT NULL COMMENT '订单状态快照',
    `total_amount`     DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '订单总额快照',
    `source_created_at` DATETIME      NOT NULL COMMENT '源创建时刻',
    `source_updated_at` DATETIME      NOT NULL COMMENT '源更新时刻',
    `etl_at`           DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'ETL 刷新时刻',
    PRIMARY KEY (`order_no`),
    KEY `idx_dwfo_date` (`date_key`),
    KEY `idx_dwfo_tenant_date` (`tenant_id`, `date_key`),
    KEY `idx_dwfo_party` (`buyer_party_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓订单事实';

-- 支付事实（PK=payment_no；date_key=实收日（未支付=创建日）；窗口含 created_at 或 paid_at）。
CREATE TABLE IF NOT EXISTS `dw_fact_payment`
(
    `payment_no`  VARCHAR(64)    NOT NULL COMMENT '支付单号',
    `date_key`    DATE           NOT NULL COMMENT '业务日期（实收日/创建日）',
    `tenant_id`   BIGINT         NOT NULL COMMENT '租户（分析维度）',
    `order_no`    VARCHAR(64)    NOT NULL COMMENT '关联订单',
    `channel`     VARCHAR(16)    NOT NULL COMMENT 'WECHAT/ALIPAY/BANK_OFFLINE',
    `status`      VARCHAR(16)    NOT NULL COMMENT '支付状态快照',
    `amount`      DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '应收金额',
    `paid_amount` DECIMAL(18, 2) NOT NULL DEFAULT 0 COMMENT '实收金额',
    `paid_at`     DATETIME(3)    NULL COMMENT '实收时刻',
    `etl_at`      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'ETL 刷新时刻',
    PRIMARY KEY (`payment_no`),
    KEY `idx_dwfp_date` (`date_key`),
    KEY `idx_dwfp_tenant_date` (`tenant_id`, `date_key`),
    KEY `idx_dwfp_order` (`order_no`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓支付事实';

-- ETL 批次台账（调度对账依据：昨日批次缺失即补跑；重跑同值台账不变）。
CREATE TABLE IF NOT EXISTS `dw_etl_log`
(
    `etl_date`         DATE        NOT NULL COMMENT 'ETL 业务日',
    `dim_date_rows`    INT         NOT NULL DEFAULT 0 COMMENT '日历维补行数',
    `dim_tenant_rows`  INT         NOT NULL DEFAULT 0 COMMENT '租户维补行数',
    `dim_party_rows`   INT         NOT NULL DEFAULT 0 COMMENT '参与方维 affected',
    `fact_order_rows`  INT         NOT NULL DEFAULT 0 COMMENT '订单事实 affected',
    `fact_payment_rows` INT        NOT NULL DEFAULT 0 COMMENT '支付事实 affected',
    `cost_ms`          INT         NOT NULL DEFAULT 0 COMMENT '耗时毫秒',
    `finished_at`      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '完成时刻',
    PRIMARY KEY (`etl_date`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='数仓 ETL 批次台账';
