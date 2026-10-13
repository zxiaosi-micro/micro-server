-- identity Outbox Relay 接线配套（S11 后补）：Relay 除 event_outbox 外还轮询 event_retry、
-- 死信写 event_dead（eventbus/relay.go）。identity 一直只发不收，建表时漏了这两张——
-- Relay 挂上后轮询即 1146。DDL 以 micro-common/eventbus/schema.go 为唯一正本，逐字对齐。
-- event_dedup 属消费侧三件套，identity 无消费者暂不建。

CREATE TABLE IF NOT EXISTS event_retry (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id       VARCHAR(64)  NOT NULL,
  consumer_group VARCHAR(64)  NOT NULL,
  event_type     VARCHAR(128) NOT NULL,
  topic          VARCHAR(128) NOT NULL,
  partition_key  VARCHAR(128) NOT NULL DEFAULT '',
  tenant_id      BIGINT       NOT NULL DEFAULT 0,
  trace_id       VARCHAR(64)  NOT NULL DEFAULT '',
  payload        JSON         NOT NULL COMMENT '事件信封全文(重投即重发)',
  status         VARCHAR(16)  NOT NULL DEFAULT 'RETRY' COMMENT 'RETRY(待重投)',
  retry_count    INT          NOT NULL DEFAULT 0 COMMENT '消费失败次数',
  next_retry_at  DATETIME(3)  NOT NULL,
  last_error     VARCHAR(512) NOT NULL DEFAULT '',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_group_event (consumer_group, event_id),
  KEY idx_dispatch (status, next_retry_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消费失败重投';

CREATE TABLE IF NOT EXISTS event_dead (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  source         VARCHAR(16)  NOT NULL COMMENT 'outbox(投递失败)/consume(消费失败)/malformed(畸形消息)',
  event_id       VARCHAR(64)  NOT NULL,
  consumer_group VARCHAR(64)  NOT NULL DEFAULT '',
  event_type     VARCHAR(128) NOT NULL DEFAULT '',
  topic          VARCHAR(128) NOT NULL DEFAULT '',
  partition_key  VARCHAR(128) NOT NULL DEFAULT '',
  tenant_id      BIGINT       NOT NULL DEFAULT 0,
  trace_id       VARCHAR(64)  NOT NULL DEFAULT '',
  payload        JSON         NOT NULL,
  retry_count    INT          NOT NULL DEFAULT 0,
  last_error     VARCHAR(512) NOT NULL DEFAULT '',
  failed_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_event (event_id),
  KEY idx_source (source, failed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件死信';
