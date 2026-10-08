-- contract_db 000002：事件消费三表（eventbus/schema.go 正本口径，S5-05 对齐修正）。
-- 去重同事务幂等（at-least-once 前提）；消费失败退避重投 ≤16 次；死信留底 + 补投递重放（E10，禁裸删）。

CREATE TABLE IF NOT EXISTS event_dedup (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  consumer_group VARCHAR(64) NOT NULL COMMENT '消费组(全局唯一)',
  event_id       VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
  consumed_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_group_event (consumer_group, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件消费去重';

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
