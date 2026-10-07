-- hello_db 初始表（S3-06 模板：custom model 覆写示例的数据源）
CREATE TABLE IF NOT EXISTS `greeting`
(
    `id`         BIGINT       NOT NULL COMMENT '雪花 ID',
    `message`    VARCHAR(255) NOT NULL COMMENT '问候语',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID（模板演示 ADR-08 全字段纪律）',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by` BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at` DATETIME     NULL COMMENT '软删标记（NULL=有效）',
    PRIMARY KEY (`id`),
    KEY `idx_greeting_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='问候语（模板示例表）';
