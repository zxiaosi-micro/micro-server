-- audit_db 初始 schema（S4-04，v4/03 阶段 4）
-- 追加式禁改删（01 FR-SYS-001）：模型层不提供 UPDATE/DELETE 方法，业务禁改删；
-- 保留 ≥ 3 年（归档策略属上线准备 S11）。平台级实体但保留 tenant_id 列（消费侧恢复）。

-- 操作/登录审计（append-only）。
CREATE TABLE IF NOT EXISTS `audit_log`
(
    `log_id`       BIGINT       NOT NULL COMMENT '雪花 ID',
    `trace_id`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '链路 ID',
    `uid`          BIGINT       NOT NULL DEFAULT 0 COMMENT '操作人 uid(0=系统)',
    `action`       VARCHAR(128) NOT NULL COMMENT '动作(auth.login/user.create/...)',
    `target_type`  VARCHAR(64)  NULL COMMENT '目标类型(user/party/role...)',
    `target_id`    VARCHAR(64)  NULL COMMENT '目标 ID(字符串,兼容多形态)',
    `before_json`  JSON         NULL COMMENT '变更前快照',
    `after_json`   JSON         NULL COMMENT '变更后快照',
    `result`       VARCHAR(16)  NOT NULL DEFAULT 'OK' COMMENT 'OK/FAIL',
    `client`       VARCHAR(32)  NULL COMMENT '来源端(ADMIN_WEB/OPS_APP...)',
    `on_behalf_of` BIGINT       NULL COMMENT '被模拟人(模拟他人场景预留)',
    `detail_json`  JSON         NULL COMMENT '扩展明细(登录 IP/UA 等)',
    `tenant_id`    BIGINT       NOT NULL DEFAULT 0 COMMENT '租户 ID',
    `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间(毫秒精度)',
    PRIMARY KEY (`log_id`),
    KEY `idx_audit_log_action` (`action`, `created_at`),
    KEY `idx_audit_log_uid` (`uid`, `created_at`),
    KEY `idx_audit_log_tenant` (`tenant_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='操作审计(追加式禁改删)';

-- 控制指令独立审计流（01 FR-SYS-001/FR-IOT-006；S7 IoT 指令链路产生，本阶段建表+写入口）。
CREATE TABLE IF NOT EXISTS `cmd_audit`
(
    `cmd_audit_id` BIGINT      NOT NULL COMMENT '雪花 ID',
    `cmd_id`       VARCHAR(64) NOT NULL COMMENT '指令 ID(cmd 表主键,逻辑引用)',
    `sn`           VARCHAR(64) NOT NULL COMMENT '设备序列号',
    `uid`          BIGINT      NOT NULL DEFAULT 0 COMMENT '下发操作人',
    `action`       VARCHAR(64) NOT NULL COMMENT '指令动作(RESTART/SET_PARAM...)',
    `payload_json` JSON        NULL COMMENT '指令参数快照',
    `result`       VARCHAR(16) NOT NULL DEFAULT 'SENT' COMMENT 'SENT/ACK/FAILED',
    `error`        VARCHAR(255) NULL COMMENT '失败原因',
    `trace_id`     VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路 ID',
    `tenant_id`    BIGINT      NOT NULL DEFAULT 0 COMMENT '租户 ID',
    `created_at`   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间(毫秒精度)',
    PRIMARY KEY (`cmd_audit_id`),
    KEY `idx_cmd_audit_cmd` (`cmd_id`),
    KEY `idx_cmd_audit_sn` (`sn`, `created_at`),
    KEY `idx_cmd_audit_tenant` (`tenant_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='指令审计(独立流,追加式禁改删)';
