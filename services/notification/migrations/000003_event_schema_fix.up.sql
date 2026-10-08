-- notification_db 000003：event_retry / event_dead 对齐 eventbus/schema.go 正本（S5-05 修正）。
-- S4 版 000002 缺 consumer_group/status/trace_id 等列——Relay 重投与死信补投递（E10）落地时暴露。
-- 两表为空队列（重试/死信不长期驻留），直接重建；event_dedup 无变化不动。

DROP TABLE IF EXISTS `event_retry`;

CREATE TABLE IF NOT EXISTS `event_retry`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`       VARCHAR(64)  NOT NULL,
    `consumer_group` VARCHAR(64)  NOT NULL,
    `event_type`     VARCHAR(128) NOT NULL,
    `topic`          VARCHAR(128) NOT NULL,
    `partition_key`  VARCHAR(128) NOT NULL DEFAULT '',
    `tenant_id`      BIGINT       NOT NULL DEFAULT 0,
    `trace_id`       VARCHAR(64)  NOT NULL DEFAULT '',
    `payload`        JSON         NOT NULL COMMENT '事件信封全文(重投即重发)',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'RETRY' COMMENT 'RETRY(待重投)',
    `retry_count`    INT          NOT NULL DEFAULT 0 COMMENT '消费失败次数',
    `next_retry_at`  DATETIME(3)  NOT NULL,
    `last_error`     VARCHAR(512) NOT NULL DEFAULT '',
    `created_at`     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_group_event` (`consumer_group`, `event_id`),
    KEY `idx_dispatch` (`status`, `next_retry_at`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='消费失败重投';

DROP TABLE IF EXISTS `event_dead`;

CREATE TABLE IF NOT EXISTS `event_dead`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `source`         VARCHAR(16)  NOT NULL COMMENT 'outbox(投递失败)/consume(消费失败)/malformed(畸形消息)',
    `event_id`       VARCHAR(64)  NOT NULL,
    `consumer_group` VARCHAR(64)  NOT NULL DEFAULT '',
    `event_type`     VARCHAR(128) NOT NULL DEFAULT '',
    `topic`          VARCHAR(128) NOT NULL DEFAULT '',
    `partition_key`  VARCHAR(128) NOT NULL DEFAULT '',
    `tenant_id`      BIGINT       NOT NULL DEFAULT 0,
    `trace_id`       VARCHAR(64)  NOT NULL DEFAULT '',
    `payload`        JSON         NOT NULL,
    `retry_count`    INT          NOT NULL DEFAULT 0,
    `last_error`     VARCHAR(512) NOT NULL DEFAULT '',
    `dead_at`        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '入死信时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='消费死信(留底+补投递重放,禁裸删 E10)';
