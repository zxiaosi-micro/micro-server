-- event_outbox 事件发件表（S3-02：auth.login 审计事件走 outbox，02 §9.3）。
-- DDL 正本在 micro-common/eventbus/schema.go（此处随服务迁移下发；生产者只需 outbox 表，
-- dedup/retry/dead 三表由首个消费方落地时一并迁移）。
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
