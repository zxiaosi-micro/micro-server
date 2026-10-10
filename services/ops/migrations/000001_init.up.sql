-- ops_db 初始 schema（S7-01，v4/03 阶段 7）
-- 告警规则/告警/工单/工单日志/SLA 计时器/巡检四表（FR-OPS-001~010）。
-- SLA 策略正本在 contract_db（FR-CTR-007），本库只存计时器快照。
-- 状态一律 VARCHAR 枚举；雪花 BIGINT 主键；软删 + 租户。

-- 告警规则（FR-OPS-001：指标/比较器/阈值/持续时间/级别/抑制窗口/作用域）。
CREATE TABLE IF NOT EXISTS `alert_rule`
(
    `rule_id`             BIGINT       NOT NULL COMMENT '雪花 ID',
    `code`                VARCHAR(64)  NOT NULL COMMENT '规则编码（租户内唯一）',
    `name`                VARCHAR(128) NOT NULL COMMENT '规则名称',
    `metric`              VARCHAR(64)  NOT NULL COMMENT '指标名（cell_temp/soc/...或系统事件名）',
    `comparator`          VARCHAR(8)   NOT NULL DEFAULT 'gt' COMMENT 'gt/gte/lt/lte',
    `threshold`           DOUBLE       NOT NULL COMMENT '阈值',
    `duration_sec`        INT          NOT NULL DEFAULT 0 COMMENT '持续越限时长（0=立即生成）',
    `level`               VARCHAR(8)   NOT NULL DEFAULT 'P2' COMMENT 'P1/P2/P3',
    `suppress_window_sec` INT          NOT NULL DEFAULT 300 COMMENT '抑制窗口（同聚合键不重复生成，聚合累计）',
    `scope`               JSON         NULL COMMENT '作用域 {"station_ids":[],"product_keys":[]}（NULL=全部）',
    `enabled`             TINYINT      NOT NULL DEFAULT 1 COMMENT '启用',
    `tenant_id`           BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`          BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`          BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`          DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`rule_id`),
    UNIQUE KEY `uk_rule_tenant_code` (`tenant_id`, `code`),
    KEY `idx_rule_metric` (`tenant_id`, `metric`, `enabled`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='告警规则';

-- 告警（OPEN/ACKED/CLOSED/ARCHIVED；场站聚合 aggregate_count；FR-OPS-002/003/009）。
-- 聚合键 = tenant+station+source+metric+level；抑制窗口内同键候选累计不新开。
CREATE TABLE IF NOT EXISTS `alert`
(
    `alert_id`         BIGINT        NOT NULL COMMENT '雪花 ID（轮询 since_id 有序扫描）',
    `rule_id`          BIGINT        NOT NULL DEFAULT 0 COMMENT '命中规则（0=系统告警无规则）',
    `level`            VARCHAR(8)    NOT NULL DEFAULT 'P2' COMMENT 'P1/P2/P3',
    `source`           VARCHAR(16)   NOT NULL DEFAULT 'TELEMETRY' COMMENT 'TELEMETRY/CMD_FAILED/OTA_PAUSED/DEVICE_ANOMALY',
    `metric`           VARCHAR(64)   NOT NULL COMMENT '指标/事件名',
    `comparator`       VARCHAR(8)    NOT NULL DEFAULT '' COMMENT '比较器（系统告警为空）',
    `threshold`        DOUBLE        NULL COMMENT '阈值',
    `last_value`       DOUBLE        NULL COMMENT '最近值',
    `station_id`       BIGINT        NOT NULL DEFAULT 0 COMMENT '场站（0=平台级，设备未绑站）',
    `station_name`     VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '场站名冗余展示',
    `device_id`        BIGINT        NOT NULL DEFAULT 0 COMMENT '首设备（0=系统告警）',
    `sn`               VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '首设备 SN',
    `title`            VARCHAR(255)  NOT NULL COMMENT '标题',
    `content`          VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '内容',
    `status`           VARCHAR(16)   NOT NULL DEFAULT 'OPEN' COMMENT 'OPEN/ACKED/CLOSED/ARCHIVED',
    `aggregate_count`  INT           NOT NULL DEFAULT 1 COMMENT '聚合次数（抑制窗口内累计）',
    `devices`          JSON          NULL COMMENT '聚合设备明细 [{"sn":"","device_id":0}]',
    `first_seen_at`    DATETIME(3)   NOT NULL COMMENT '首次命中',
    `last_seen_at`     DATETIME(3)   NOT NULL COMMENT '最近命中',
    `acked_by`         BIGINT        NULL COMMENT '确认人 uid',
    `acked_at`         DATETIME(3)   NULL COMMENT '确认时刻',
    `closed_ticket_id` BIGINT        NOT NULL DEFAULT 0 COMMENT '关闭关联工单（关闭前置，FR-OPS-003）',
    `closed_reason`    VARCHAR(512)  NULL COMMENT '人工关闭原因',
    `closed_at`        DATETIME(3)   NULL,
    `archived_at`      DATETIME(3)   NULL COMMENT '归档时刻',
    `tenant_id`        BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`alert_id`),
    -- 轮询增量接口：since_id 有序扫描（FR-OPS-010）
    KEY `idx_alert_pending` (`tenant_id`, `station_id`, `status`, `alert_id`),
    -- 聚合查找：抑制窗口内同聚合键的活跃告警
    KEY `idx_alert_agg` (`tenant_id`, `station_id`, `source`, `metric`, `level`, `status`, `last_seen_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='告警';

-- 工单（状态机 PENDING_DISPATCH→DISPATCHED→ACCEPTED→PROCESSING→SUBMITTED→VERIFIED→CLOSED；FR-OPS-004/005/008）。
CREATE TABLE IF NOT EXISTS `ticket`
(
    `ticket_id`         BIGINT       NOT NULL COMMENT '雪花 ID',
    `ticket_no`         VARCHAR(64)  NOT NULL COMMENT '工单号（租户内唯一）',
    `type`              VARCHAR(16)  NOT NULL DEFAULT 'ALERT' COMMENT 'ALERT/AFTER_SALE/INSPECTION/INSTALL/STATION',
    `title`             VARCHAR(255) NOT NULL COMMENT '标题',
    `content`           VARCHAR(2048) NOT NULL DEFAULT '' COMMENT '内容',
    `priority`          VARCHAR(8)   NOT NULL DEFAULT 'P2' COMMENT 'P1/P2/P3（告警 level 直接映射）',
    `station_id`        BIGINT       NOT NULL DEFAULT 0 COMMENT '场站（0=平台级）',
    `station_name`      VARCHAR(128) NOT NULL DEFAULT '' COMMENT '场站名冗余',
    `device_id`         BIGINT       NOT NULL DEFAULT 0,
    `sn`                VARCHAR(64)  NOT NULL DEFAULT '',
    `alert_id`          BIGINT       NOT NULL DEFAULT 0 COMMENT '来源告警（type=ALERT）',
    `inspection_item_id` BIGINT      NOT NULL DEFAULT 0 COMMENT '来源巡检异常项（type=INSPECTION）',
    `assignee_uid`      BIGINT       NULL COMMENT '受理人 uid（staff.user_id）',
    `assignee_name`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '受理人名冗余',
    `status`            VARCHAR(24)  NOT NULL DEFAULT 'PENDING_DISPATCH' COMMENT '状态机见注释',
    `escalate_level`    INT          NOT NULL DEFAULT 0 COMMENT '接单超时升级次数',
    `dispatch_at`       DATETIME(3)  NULL,
    `accept_deadline`   DATETIME(3)  NULL COMMENT '接单截止（超时扫描升级，ADR-09）',
    `accepted_at`       DATETIME(3)  NULL,
    `submitted_at`      DATETIME(3)  NULL,
    `verified_at`       DATETIME(3)  NULL,
    `closed_at`         DATETIME(3)  NULL,
    `cancel_reason`     VARCHAR(512)  NULL,
    `created_by`        BIGINT       NULL COMMENT '创建人 uid（0=系统生成）',
    `client_action_id`  VARCHAR(64)  NULL COMMENT '离线工单幂等键（FR-OPS-008，L2）',
    `tenant_id`         BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`        BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`        DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`ticket_id`),
    UNIQUE KEY `uk_ticket_tenant_no` (`tenant_id`, `ticket_no`),
    UNIQUE KEY `uk_ticket_client_action` (`tenant_id`, `client_action_id`),
    -- 轮询增量接口（FR-OPS-010）
    KEY `idx_ticket_pending` (`tenant_id`, `station_id`, `status`, `ticket_id`),
    KEY `idx_ticket_assignee` (`tenant_id`, `assignee_uid`, `status`),
    -- 接单超时扫描（ADR-09：next_exec_at 同款语义）
    KEY `idx_ticket_deadline` (`tenant_id`, `status`, `accept_deadline`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='工单';

-- 工单日志（状态迁移留痕；operator_uid=0 表示系统 cron 扫描）。
CREATE TABLE IF NOT EXISTS `ticket_log`
(
    `log_id`       BIGINT       NOT NULL COMMENT '雪花 ID',
    `ticket_id`    BIGINT       NOT NULL COMMENT '工单',
    `action`       VARCHAR(24)  NOT NULL COMMENT 'create/dispatch/accept/start/submit/verify/close/cancel/escalate/reassign',
    `from_status`  VARCHAR(24)  NOT NULL DEFAULT '',
    `to_status`    VARCHAR(24)  NOT NULL DEFAULT '',
    `operator_uid` BIGINT       NOT NULL DEFAULT 0 COMMENT '操作人（0=系统）',
    `note`         VARCHAR(1024) NOT NULL DEFAULT '',
    `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    PRIMARY KEY (`log_id`),
    KEY `idx_tlog_ticket` (`ticket_id`, `log_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='工单日志';

-- SLA 计时器（策略快照冻结 + 暂停留痕 + 分级升级；FR-OPS-006/FR-CTR-007）。
-- 策略正本在 contract_db；本表 snapshot 为创建时刻冻结值，正本变更不影响已计时工单。
CREATE TABLE IF NOT EXISTS `sla_timer`
(
    `timer_id`           BIGINT      NOT NULL COMMENT '雪花 ID',
    `ticket_id`          BIGINT      NOT NULL COMMENT '工单',
    `strategy_id`        BIGINT      NOT NULL COMMENT '策略（contract.sla_strategy）',
    `strategy_snapshot`  JSON        NOT NULL COMMENT '策略快照（response/resolve 分钟，冻结）',
    `response_due_at`    DATETIME(3) NOT NULL COMMENT '响应时限（接单即达成）',
    `resolve_due_at`     DATETIME(3) NOT NULL COMMENT '解决时限（验收即达成）',
    `paused_at`          DATETIME(3) NULL COMMENT '暂停时刻（NULL=计时中）',
    `pause_reason`       VARCHAR(512) NULL COMMENT '暂停原因（必填留痕）',
    `pause_total_min`    INT         NOT NULL DEFAULT 0 COMMENT '累计暂停分钟（恢复时顺延 due_at）',
    `escalate_level`     INT         NOT NULL DEFAULT 0 COMMENT '超时分级升级次数',
    `responded_at`       DATETIME(3) NULL COMMENT '响应达成时刻（接单）',
    `resolved_at`        DATETIME(3) NULL COMMENT '解决达成时刻（验收）',
    `response_breach_at` DATETIME(3) NULL COMMENT '响应超时发生（NULL=未超）',
    `resolve_breach_at`  DATETIME(3) NULL COMMENT '解决超时发生（NULL=未超）',
    `tenant_id`          BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`timer_id`),
    UNIQUE KEY `uk_timer_ticket` (`ticket_id`),
    -- ScanBreaches 扫描（ADR-09）
    KEY `idx_timer_breach` (`tenant_id`, `response_breach_at`, `resolve_breach_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='SLA 计时器';

-- 巡检模板（表单 schema；station_created 自动建计划时用默认模板）。
CREATE TABLE IF NOT EXISTS `inspection_template`
(
    `template_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `name`         VARCHAR(128) NOT NULL COMMENT '模板名',
    `items_schema` JSON         NOT NULL COMMENT '检查项 [{"name":"","type":"bool/number/text","required":true}]',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`template_id`),
    KEY `idx_itpl_tenant` (`tenant_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='巡检模板';

-- 巡检计划（cron 简化集合 @daily/@weekly/@monthly/@h:N；next_exec_at 扫描生成任务，ADR-09）。
CREATE TABLE IF NOT EXISTS `inspection_plan`
(
    `plan_id`      BIGINT       NOT NULL COMMENT '雪花 ID',
    `station_id`   BIGINT       NOT NULL COMMENT '场站',
    `station_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '场站名冗余',
    `name`         VARCHAR(128) NOT NULL COMMENT '计划名',
    `template_id`  BIGINT       NOT NULL COMMENT '模板',
    `cron_expr`    VARCHAR(32)  NOT NULL DEFAULT '@daily' COMMENT '@daily/@weekly/@monthly/@h:N',
    `enabled`      TINYINT      NOT NULL DEFAULT 1,
    `next_exec_at` DATETIME(3)  NOT NULL COMMENT '下次生成时刻（扫描驱动）',
    `last_exec_at` DATETIME(3)  NULL COMMENT '上次生成时刻',
    `tenant_id`    BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`plan_id`),
    UNIQUE KEY `uk_iplan_station_name` (`tenant_id`, `station_id`, `name`),
    KEY `idx_iplan_exec` (`enabled`, `next_exec_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='巡检计划';

-- 巡检任务（cron 生成的待办；PENDING/SUBMITTED/CANCELLED）。
CREATE TABLE IF NOT EXISTS `inspection_task`
(
    `task_id`       BIGINT       NOT NULL COMMENT '雪花 ID',
    `plan_id`       BIGINT       NOT NULL COMMENT '计划',
    `template_id`   BIGINT       NOT NULL COMMENT '模板',
    `station_id`    BIGINT       NOT NULL COMMENT '场站',
    `station_name`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '场站名冗余',
    `title`         VARCHAR(255) NOT NULL COMMENT '任务标题',
    `scheduled_for` DATETIME(3)  NOT NULL COMMENT '计划巡检时刻',
    `status`        VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SUBMITTED/CANCELLED',
    `submitted_by`  BIGINT       NULL COMMENT '提交人 uid',
    `submitted_at`  DATETIME(3)  NULL,
    `location`      VARCHAR(255) NULL COMMENT '提交定位',
    `photos`        JSON         NULL COMMENT '提交照片 URL 数组',
    `remark`        VARCHAR(512)  NULL,
    `anomaly_count` INT          NOT NULL DEFAULT 0 COMMENT '异常项数（提交时统计）',
    `ticket_ids`    JSON         NULL COMMENT '生成工单 ID 数组',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`task_id`),
    -- 同计划同巡检时刻幂等（cron 重放不重复生成）
    UNIQUE KEY `uk_itask_plan_sched` (`plan_id`, `scheduled_for`),
    KEY `idx_itask_station` (`tenant_id`, `station_id`, `status`, `task_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='巡检任务';

-- 巡检明细项（提交时批量写入；异常项直生成工单并回填 ticket_id，FR-OPS-007）。
CREATE TABLE IF NOT EXISTS `inspection_item`
(
    `item_id`  BIGINT       NOT NULL COMMENT '雪花 ID',
    `task_id`  BIGINT       NOT NULL COMMENT '任务',
    `name`     VARCHAR(255) NOT NULL COMMENT '检查项名（模板 schema 同名项）',
    `result`   JSON         NULL COMMENT '结果 {"value":...,"note":""}',
    `anomalous` TINYINT     NOT NULL DEFAULT 0 COMMENT '异常标记',
    `ticket_id` BIGINT      NOT NULL DEFAULT 0 COMMENT '异常项生成工单（0=无）',
    `tenant_id` BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`item_id`),
    KEY `idx_iitem_task` (`task_id`, `item_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='巡检明细项';

-- Outbox（事件三表正本 eventbus/schema.go；此处仅 outbox，dedup/retry/dead 见 000002）。
-- ⚠ 列清单以 schema.go 为唯一正本对齐（known-pit：漏 last_error 列 → Relay 周期性 1054 →
--   sqlx 熔断器打开连累全部 RPC）。
CREATE TABLE IF NOT EXISTS event_outbox (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id      VARCHAR(64)  NOT NULL COMMENT '事件唯一标识(去重/重投/对账键)',
  event_type    VARCHAR(128) NOT NULL COMMENT '事件类型(点号命名)',
  topic         VARCHAR(128) NOT NULL COMMENT '目标 topic(下划线命名)',
  partition_key VARCHAR(128) NOT NULL DEFAULT '' COMMENT '分区保序键=聚合ID',
  tenant_id     BIGINT       NOT NULL DEFAULT 0 COMMENT '租户ID(消费侧恢复)',
  trace_id      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '生产侧链路ID',
  payload       JSON         NOT NULL COMMENT '事件信封全文',
  status        VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/FAILED/DEAD',
  retry_count   INT          NOT NULL DEFAULT 0 COMMENT '投递重试次数',
  next_retry_at DATETIME(3)  NOT NULL COMMENT '下次投递时间(退避)',
  last_error    VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近一次投递错误',
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_event_id (event_id),
  KEY idx_dispatch (status, next_retry_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件发件箱(outbox)';

-- 默认巡检模板不预置种子：station_created 消费时按租户自动创建（多租户口径，
-- 模板名"默认日巡模板"，不存在才建——见 logic/bridge.go）。
