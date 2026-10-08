-- file_db 初始 schema（S4-04，v4/03 阶段 4）
-- 对象存储在 MinIO（S3 兼容，双桶 micro-file/micro-contract），本表只存元数据。

-- 文件元数据：oss_key 规范 /{tenant}/{biz_type}/{yyyy}/{mm}/{uuid}.{ext}（02 §6.5）。
CREATE TABLE IF NOT EXISTS `file_meta`
(
    `file_id`      BIGINT       NOT NULL COMMENT '雪花 ID',
    `bucket`       VARCHAR(64)  NOT NULL COMMENT '目标桶(micro-file/micro-contract,biz_type 路由)',
    `oss_key`      VARCHAR(512) NOT NULL COMMENT '对象键 /{tenant}/{biz_type}/{yyyy}/{mm}/{uuid}.{ext}',
    `name`         VARCHAR(255) NOT NULL COMMENT '原始文件名',
    `content_type` VARCHAR(128) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'MIME 类型',
    `size`         BIGINT       NOT NULL DEFAULT 0 COMMENT '字节数',
    `biz_type`     VARCHAR(64)  NOT NULL DEFAULT 'common' COMMENT '业务类型(contract 路由合同桶,其余普通桶)',
    `biz_id`       BIGINT       NULL COMMENT '业务 ID(合同/工单等,可空)',
    `sha256`       CHAR(64)     NULL COMMENT '内容哈希(校验/去重预留)',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`file_id`),
    KEY `idx_file_meta_biz` (`biz_type`, `biz_id`),
    KEY `idx_file_meta_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='文件元数据';
