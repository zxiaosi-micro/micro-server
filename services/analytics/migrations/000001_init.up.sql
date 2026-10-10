-- analytics_db 000001：CQRS 读模型（S8-01，v4/02 §9.8）。
-- 六大日聚合表 + 事件台账（业务键幂等）+ watermark + 导出/对账。
-- 红线：报表服务面只读本库；业务库只在日级对账（recon）直查，不入服务面。
-- 聚合表无 deleted_at：读模型全量重建语义（truncate + 回放），不参与软删。

-- ---------- 事件台账（业务键幂等正本） ----------
-- 幂等口径：uk_biz = (event_type, biz_key)。同事件重投（event_id 去重走 event_dedup）
-- 与补投递重放（p4backfill 新 event_id）都被业务键挡住；stock.low 属状态边沿事件
-- 无稳定业务键，biz_key 取 event_id（对账不覆盖该口径，见指标字典）。
CREATE TABLE IF NOT EXISTS report_event_entry (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id     VARCHAR(64)  NOT NULL COMMENT '信封 event_id（记录用，幂等不以它为准）',
  event_type   VARCHAR(64)  NOT NULL COMMENT '点号事件类型',
  topic        VARCHAR(64)  NOT NULL,
  tenant_id    BIGINT       NOT NULL DEFAULT 0,
  biz_key      VARCHAR(128) NOT NULL COMMENT '业务幂等键（order_no/sn/warranty_no/shipment_no/station_no/alert_id/refund_no 等）',
  stat_date    DATE         NOT NULL COMMENT '业务日（信封 occurred_at 取日期）',
  payload      JSON         NOT NULL COMMENT '事件 payload 原文',
  consumed_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY `uk_biz` (`event_type`, `biz_key`),
  KEY `idx_tenant_date` (`tenant_id`, `stat_date`),
  KEY `idx_stat_date` (`stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='读模型事件台账（业务键幂等）';

-- ---------- 六大日聚合表（FR-ANA-002：销售/库存/质保/SLA/财务/场站） ----------
CREATE TABLE IF NOT EXISTS report_sales_daily (
  id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id          BIGINT        NOT NULL,
  stat_date          DATE          NOT NULL,
  orders_created     INT           NOT NULL DEFAULT 0 COMMENT '新增订单数',
  orders_paid        INT           NOT NULL DEFAULT 0 COMMENT '支付完成订单数',
  orders_cancelled   INT           NOT NULL DEFAULT 0 COMMENT '取消订单数（含支付超时）',
  orders_pay_timeout INT           NOT NULL DEFAULT 0 COMMENT '支付超时数',
  returns_approved   INT           NOT NULL DEFAULT 0 COMMENT '退货审批通过数',
  shipments_signed   INT           NOT NULL DEFAULT 0 COMMENT '签收发运单数',
  devices_shipped    INT           NOT NULL DEFAULT 0 COMMENT '签收设备数（SN 计）',
  gmv_paid           DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '实付 GMV（按支付事件累加）',
  created_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date` (`tenant_id`, `stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='销售日报';

CREATE TABLE IF NOT EXISTS report_inventory_daily (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id        BIGINT   NOT NULL,
  stat_date        DATE     NOT NULL,
  warehouse_id     BIGINT   NOT NULL DEFAULT 0 COMMENT '0=未分仓库事件',
  stock_in_qty     BIGINT   NOT NULL DEFAULT 0 COMMENT '入库数量合计',
  stock_out_qty    BIGINT   NOT NULL DEFAULT 0 COMMENT '出库（DEDUCT）数量合计',
  low_stock_events INT      NOT NULL DEFAULT 0 COMMENT '低库存边沿触发次数（状态事件，对账不覆盖）',
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date_wh` (`tenant_id`, `stat_date`, `warehouse_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='库存日报';

CREATE TABLE IF NOT EXISTS report_warranty_daily (
  id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id          BIGINT   NOT NULL,
  stat_date          DATE     NOT NULL,
  warranties_started INT      NOT NULL DEFAULT 0 COMMENT '质保起算数',
  warranties_device  INT      NOT NULL DEFAULT 0 COMMENT '设备质保起算数（level=DEVICE）',
  warranties_station INT      NOT NULL DEFAULT 0 COMMENT '场站质保起算数（level=STATION）',
  devices_activated  INT      NOT NULL DEFAULT 0 COMMENT '设备激活数',
  created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date` (`tenant_id`, `stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='质保日报';

CREATE TABLE IF NOT EXISTS report_sla_daily (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id    BIGINT   NOT NULL,
  stat_date    DATE     NOT NULL,
  station_id   BIGINT   NOT NULL DEFAULT 0 COMMENT '0=平台级（场站归属失败降级）',
  alerts_total INT      NOT NULL DEFAULT 0 COMMENT '告警产生数',
  alerts_p1    INT      NOT NULL DEFAULT 0 COMMENT 'P1 告警数',
  alerts_p2    INT      NOT NULL DEFAULT 0 COMMENT 'P2 告警数',
  alerts_p3    INT      NOT NULL DEFAULT 0 COMMENT 'P3 告警数',
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date_station` (`tenant_id`, `stat_date`, `station_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SLA 日报（告警量口径）';

CREATE TABLE IF NOT EXISTS report_finance_daily (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id     BIGINT        NOT NULL,
  stat_date     DATE          NOT NULL,
  channel       VARCHAR(32)   NOT NULL DEFAULT '' COMMENT '支付渠道 WECHAT/ALIPAY/BANK_OFFLINE',
  payments_count INT          NOT NULL DEFAULT 0 COMMENT '支付完成笔数',
  paid_amount   DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '实付金额合计',
  refunds_count INT           NOT NULL DEFAULT 0 COMMENT '退款笔数',
  refund_amount DECIMAL(18,2) NOT NULL DEFAULT 0 COMMENT '退款金额合计',
  created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date_channel` (`tenant_id`, `stat_date`, `channel`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='财务日报';

CREATE TABLE IF NOT EXISTS report_station_daily (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id        BIGINT   NOT NULL,
  stat_date        DATE     NOT NULL,
  station_id       BIGINT   NOT NULL DEFAULT 0,
  stations_created INT      NOT NULL DEFAULT 0 COMMENT '新建场站数',
  alerts_total     INT      NOT NULL DEFAULT 0 COMMENT '场站告警数',
  alerts_p1        INT      NOT NULL DEFAULT 0 COMMENT '场站 P1 告警数',
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_tenant_date_station` (`tenant_id`, `stat_date`, `station_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='场站日报';

-- ---------- watermark / 元数据（平台级，无租户维度） ----------
CREATE TABLE IF NOT EXISTS report_watermark (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  topic           VARCHAR(64)  NOT NULL COMMENT 'Kafka topic',
  consumer_group  VARCHAR(64)  NOT NULL COMMENT '消费组（含版本号）',
  last_event_id   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '最近消费 event_id',
  last_stat_date  DATE         NULL COMMENT '最近业务日（watermark）',
  processed_count BIGINT       NOT NULL DEFAULT 0 COMMENT '累计处理条数',
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY `uk_topic` (`topic`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='读模型消费 watermark';

CREATE TABLE IF NOT EXISTS report_meta (
  k VARCHAR(64) NOT NULL COMMENT '元数据键（rebuild_version 等）',
  v VARCHAR(255) NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (k)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='读模型元数据（消费组版本号等）';

-- ---------- 导出任务（S8-03） ----------
CREATE TABLE IF NOT EXISTS export_task (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT       NOT NULL,
  report_type  VARCHAR(32)  NOT NULL COMMENT 'sales/inventory/warranty/sla/finance/station/event_ledger',
  format       VARCHAR(16)  NOT NULL COMMENT 'CSV/XLSX/PDF',
  date_start   DATE         NOT NULL,
  date_end     DATE         NOT NULL,
  status       VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/RUNNING/DONE/FAILED',
  file_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '产物文件（file 服务）',
  file_name    VARCHAR(255) NOT NULL DEFAULT '' COMMENT '产物文件名',
  row_count    INT          NOT NULL DEFAULT 0,
  cost_ms      INT          NOT NULL DEFAULT 0 COMMENT '渲染耗时（毫秒）',
  error        VARCHAR(512) NOT NULL DEFAULT '',
  requested_by BIGINT       NOT NULL DEFAULT 0 COMMENT '发起人 uid',
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY `idx_tenant_created` (`tenant_id`, `created_at`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='异步导出任务';

-- ---------- 日级对账（S8-04） ----------
CREATE TABLE IF NOT EXISTS recon_diff (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id    BIGINT   NOT NULL,
  recon_date   DATE     NOT NULL COMMENT '对账执行日',
  biz_date     DATE     NOT NULL COMMENT '被对账的业务日',
  metric_code  VARCHAR(64) NOT NULL COMMENT '指标编码（report_metric_dict）',
  source_count BIGINT   NOT NULL DEFAULT 0 COMMENT '业务库口径值',
  model_count  BIGINT   NOT NULL DEFAULT 0 COMMENT '读模型口径值',
  diff_count   BIGINT   NOT NULL DEFAULT 0 COMMENT '差异（source - model）',
  status       VARCHAR(16) NOT NULL DEFAULT 'OPEN' COMMENT 'OPEN/RESOLVED',
  resolved_at  DATETIME NULL,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY `uk_metric_bizdate` (`tenant_id`, `metric_code`, `biz_date`),
  KEY `idx_status` (`status`, `recon_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='读模型日级对账差异（修复只走补投递，E10 禁裸删）';

CREATE TABLE IF NOT EXISTS recon_state (
  biz_date    DATE     NOT NULL,
  status      VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/RUNNING/DONE',
  diff_total  INT      NOT NULL DEFAULT 0,
  notified    TINYINT  NOT NULL DEFAULT 0 COMMENT '差异站内信是否已发（防重）',
  run_at      DATETIME(3) NULL COMMENT '最近执行时刻',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (biz_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='日级对账执行状态（cron 幂等）';

-- ---------- Outbox（analytics 发出：export_requested / notification_request） ----------
CREATE TABLE IF NOT EXISTS event_outbox (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id       VARCHAR(64)  NOT NULL,
  event_type     VARCHAR(128) NOT NULL,
  topic          VARCHAR(128) NOT NULL,
  partition_key  VARCHAR(128) NOT NULL DEFAULT '',
  tenant_id      BIGINT       NOT NULL DEFAULT 0,
  trace_id       VARCHAR(64)  NOT NULL DEFAULT '',
  payload        JSON         NOT NULL,
  status         VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
  retry_count    INT          NOT NULL DEFAULT 0,
  next_retry_at  DATETIME(3)  NOT NULL,
  last_error     VARCHAR(512) NOT NULL DEFAULT '',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY `uk_event_id` (`event_id`),
  KEY `idx_dispatch` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件 Outbox（eventbus/schema.go 正本口径）';
