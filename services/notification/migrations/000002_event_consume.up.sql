-- notification_db 000002：事件消费三表（首个消费方落地时迁移，eventbus/schema.go 口径）。
-- notification 是 notification_request 事件的唯一消费方；去重同事务幂等，失败重投退避，超限进死信留底。

CREATE TABLE IF NOT EXISTS `event_dedup`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `consumer_group` VARCHAR(64) NOT NULL COMMENT '消费组(全局唯一)',
    `event_id`       VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    `consumed_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_group_event` (`consumer_group`, `event_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='事件消费去重';

CREATE TABLE IF NOT EXISTS `event_retry`
(
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`      VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    `topic`         VARCHAR(128) NOT NULL COMMENT '目标 topic',
    `partition_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '分区保序键',
    `payload`       JSON        NOT NULL COMMENT '事件信封全文',
    `retry_count`   INT         NOT NULL DEFAULT 0 COMMENT '已重试次数',
    `next_retry_at` DATETIME(3) NOT NULL COMMENT '下次重投时间(退避)',
    `last_error`    VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近失败原因',
    `created_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_dispatch` (`next_retry_at`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='消费失败重投';

CREATE TABLE IF NOT EXISTS `event_dead`
(
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id`   VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    `topic`      VARCHAR(128) NOT NULL COMMENT '目标 topic',
    `payload`    JSON        NOT NULL COMMENT '事件信封全文',
    `last_error` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最终失败原因',
    `dead_at`    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '入死信时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT ='消费死信(留底+补投递重放,禁裸删 E10)';
