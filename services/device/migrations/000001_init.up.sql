-- device_db 初始 schema（S6-01，v4/03 阶段 6）
-- 设备主数据（一机一密）/生命周期日志（只由事件驱动）/设备拓扑/固件/OTA 任务/指令（FR-DEV-001~006、FR-IOT-006/007）。
-- 状态一律 VARCHAR 枚举（加枚举零 DDL）；雪花 BIGINT 主键；软删 + 租户。

-- 设备主数据（status 状态机：PRODUCED→IN_STOCK→OUT→ACTIVATED→RETIRED→SCRAPPED；
-- 状态只由事件驱动（stock_in/stock_out/device_activated），Transition 仅限退役/报废白名单对，FR-DEV-003）。
CREATE TABLE IF NOT EXISTS `device`
(
    `device_id`     BIGINT       NOT NULL COMMENT '雪花 ID',
    `sn`            VARCHAR(64)  NOT NULL COMMENT '设备 SN（全局唯一，FR-DEV-001）',
    `product_key`   VARCHAR(64)  NOT NULL COMMENT '产品标识（对齐 catalog 商品，遥测超级表前缀 ADR-13）',
    `model`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '型号',
    `batch_no`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '生产批次',
    `device_secret` VARCHAR(512) NULL COMMENT '设备密钥（AES-256-GCM 信封密文，一机一密 FR-IOT-002；明文仅产线 CSV 一次）',
    `status`        VARCHAR(16)  NOT NULL DEFAULT 'PRODUCED' COMMENT 'PRODUCED/IN_STOCK/OUT/ACTIVATED/RETIRED/SCRAPPED',
    `party_id`      BIGINT       NULL COMMENT '绑定客户（激活时写入）',
    `order_no`      VARCHAR(64)  NULL COMMENT '来源订单号（出库事件回填）',
    `activated_at`  DATETIME(3)  NULL COMMENT '激活时刻（device.activated 事件驱动）',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT       NULL COMMENT '创建人 uid',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`    BIGINT       NULL COMMENT '更新人 uid',
    `deleted_at`    DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`device_id`),
    UNIQUE KEY `uk_device_sn` (`sn`),
    KEY `idx_device_tenant_status` (`tenant_id`, `status`, `created_at`),
    KEY `idx_device_tenant_sn` (`tenant_id`, `sn`),
    KEY `idx_device_product` (`tenant_id`, `product_key`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='设备主数据';

-- 设备生命周期日志（只由事件驱动写库：stock.in/stock.out/device.activated/manual.transition；
-- 幂等键 = device_id+event_type+event_id，事件重放 1062 直接跳过，FR-DEV-003）。
CREATE TABLE IF NOT EXISTS `device_lifecycle_log`
(
    `log_id`     BIGINT       NOT NULL COMMENT '雪花 ID',
    `device_id`  BIGINT       NOT NULL COMMENT '设备',
    `sn`         VARCHAR(64)  NOT NULL COMMENT 'SN 冗余（日志只增不改，免联查）',
    `from_status` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '前置状态（首笔为空）',
    `to_status`  VARCHAR(16)  NOT NULL COMMENT '目标状态',
    `event_type` VARCHAR(64)  NOT NULL COMMENT '触发事件类型（点号命名）',
    `event_id`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '事件唯一标识（幂等键）',
    `remark`     VARCHAR(512) NULL COMMENT '备注/原因',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '操作人 uid（manual 类才有）',
    PRIMARY KEY (`log_id`),
    UNIQUE KEY `uk_lifecycle_dedup` (`device_id`, `event_type`, `event_id`),
    KEY `idx_lifecycle_device` (`device_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='设备生命周期日志（只由事件驱动）';

-- 设备拓扑（电芯→模组→电池包→充电桩四级，FR-DEV-004；一设备一棵树，parent_id 空为根）。
CREATE TABLE IF NOT EXISTS `device_topology`
(
    `node_id`    BIGINT       NOT NULL COMMENT '雪花 ID',
    `device_id`  BIGINT       NOT NULL COMMENT '所属设备',
    `parent_id`  BIGINT       NULL COMMENT '父节点（NULL=根）',
    `node_type`  VARCHAR(16)  NOT NULL COMMENT 'CELL 电芯/MODULE 模组/PACK 电池包/CHARGER 充电桩',
    `node_name`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '节点名称/位号',
    `sort`       INT          NOT NULL DEFAULT 0 COMMENT '同级排序',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`node_id`),
    KEY `idx_topology_device` (`device_id`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='设备拓扑（电芯/模组/电池包/充电桩）';

-- 固件包（sha256 + Ed25519 签名强校验：验签失败拒绝入库，未签名不可下发，FR-IOT-007）。
CREATE TABLE IF NOT EXISTS `firmware`
(
    `firmware_id`   BIGINT        NOT NULL COMMENT '雪花 ID',
    `product_key`   VARCHAR(64)   NOT NULL COMMENT '适用产品',
    `version`       VARCHAR(32)   NOT NULL COMMENT '固件版本',
    `file_url`      VARCHAR(512)  NOT NULL COMMENT '固件包对象存储地址（MinIO file 桶）',
    `file_size`     BIGINT        NOT NULL DEFAULT 0 COMMENT '字节数',
    `sha256`        CHAR(64)      NOT NULL COMMENT '固件包 SHA-256（hex）',
    `signature`     VARCHAR(1024) NOT NULL COMMENT 'Ed25519 签名（base64，对 sha256 hex 串签名）',
    `sign_alg`      VARCHAR(16)   NOT NULL DEFAULT 'Ed25519' COMMENT '签名算法',
    `remark`        VARCHAR(512)  NULL,
    `tenant_id`     BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`    BIGINT        NULL COMMENT '上传人 uid',
    `updated_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`    DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`firmware_id`),
    UNIQUE KEY `uk_firmware_pk_version` (`tenant_id`, `product_key`, `version`),
    KEY `idx_firmware_tenant` (`tenant_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='固件包（签名强校验）';

-- OTA 任务（灰度分批 + 失败率超阈自动暂停，FR-IOT-007）。
CREATE TABLE IF NOT EXISTS `ota_task`
(
    `task_id`               BIGINT        NOT NULL COMMENT '雪花 ID',
    `task_no`               VARCHAR(64)   NOT NULL COMMENT '任务号（幂等键）',
    `name`                  VARCHAR(128)  NOT NULL COMMENT '任务名称',
    `product_key`           VARCHAR(64)   NOT NULL COMMENT '目标产品',
    `firmware_id`           BIGINT        NOT NULL COMMENT '目标固件',
    `rollback_firmware_id`  BIGINT        NULL COMMENT '回滚固件（失败回退目标）',
    `batch_size`            INT           NOT NULL DEFAULT 50 COMMENT '灰度批次大小（每轮下发的设备数）',
    `fail_threshold_pct`    INT           NOT NULL DEFAULT 20 COMMENT '失败率阈值 %（超过自动 PAUSED）',
    `status`                VARCHAR(16)   NOT NULL DEFAULT 'CREATED' COMMENT 'CREATED/RUNNING/PAUSED/DONE/CANCELLED/ROLLED_BACK',
    `total`                 INT           NOT NULL DEFAULT 0 COMMENT '设备总数',
    `success_count`         INT           NOT NULL DEFAULT 0 COMMENT '成功数',
    `fail_count`            INT           NOT NULL DEFAULT 0 COMMENT '失败数',
    `fail_reason`           VARCHAR(512)  NULL COMMENT '暂停/回滚原因',
    `tenant_id`             BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`            BIGINT        NULL COMMENT '创建人 uid',
    `updated_at`            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`            DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`task_id`),
    UNIQUE KEY `uk_ota_task_tenant_no` (`tenant_id`, `task_no`),
    KEY `idx_ota_task_scan` (`status`, `updated_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='OTA 任务';

-- OTA 设备明细（断点续传基线：status 保留 + cmd_id 回查，重启后按行状态续推）。
CREATE TABLE IF NOT EXISTS `ota_device`
(
    `id`            BIGINT       NOT NULL COMMENT '雪花 ID',
    `task_id`       BIGINT       NOT NULL COMMENT '所属任务',
    `device_id`     BIGINT       NOT NULL COMMENT '设备',
    `sn`            VARCHAR(64)  NOT NULL COMMENT 'SN 冗余展示',
    `status`        VARCHAR(16)  NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/SUCCESS/FAILED',
    `cmd_id`        BIGINT       NULL COMMENT '升级指令（复用指令链路含 3s ACK/重试，FR-IOT-007）',
    `retry_count`   INT          NOT NULL DEFAULT 0 COMMENT '本轮重试次数',
    `error`         VARCHAR(512) NULL COMMENT '最近失败原因',
    `dispatched_at` DATETIME(3)  NULL COMMENT '最近下发时刻',
    `finished_at`   DATETIME(3)  NULL COMMENT '终态时刻',
    `tenant_id`     BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_ota_device_task_dev` (`task_id`, `device_id`),
    KEY `idx_ota_device_scan` (`task_id`, `status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='OTA 设备明细';

-- 指令下行（cmd 表状态机：PENDING→SENT→ACKED/FAILED；TimingWheel 3s 计 ACK 为内存加速，
-- 定时扫描 next_exec_at 兜底重试 ≤3 次，ADR-09/02 §9.6-9.7；审计走 audit.cmd_audit 独立流）。
CREATE TABLE IF NOT EXISTS `cmd`
(
    `cmd_id`       BIGINT        NOT NULL COMMENT '雪花 ID',
    `device_id`    BIGINT        NOT NULL COMMENT '目标设备',
    `sn`           VARCHAR(64)   NOT NULL COMMENT 'SN 冗余（MQTT 主题构造 down/{sn}/cmd）',
    `cmd_type`     VARCHAR(32)   NOT NULL COMMENT 'REBOOT/RELAY_SET/QUERY/OTA_UPGRADE/…',
    `params`       JSON          NULL COMMENT '指令参数',
    `status`       VARCHAR(16)   NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SENT/ACKED/FAILED',
    `retry_count`  INT           NOT NULL DEFAULT 0 COMMENT '已重试次数（≤max_retry）',
    `max_retry`    INT           NOT NULL DEFAULT 3 COMMENT '最大重试次数',
    `next_exec_at` DATETIME(3)   NOT NULL COMMENT '下次投递/重试时刻（TimingWheel 失联后 cron 兜底，E16）',
    `acked_at`     DATETIME(3)   NULL COMMENT 'ACK 时刻',
    `fail_reason`  VARCHAR(512)  NULL COMMENT '失败原因',
    `client_cmd_id` VARCHAR(64)  NULL COMMENT '调用方幂等键（可空）',
    `operator`     BIGINT        NULL COMMENT '下发操作人 uid',
    `tenant_id`    BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`cmd_id`),
    UNIQUE KEY `uk_cmd_client_id` (`tenant_id`, `client_cmd_id`),
    -- 指令重试扫描索引（E16：扫描 SQL 必须命中索引）
    KEY `idx_cmd_retry_scan` (`status`, `next_exec_at`),
    KEY `idx_cmd_device` (`device_id`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='设备指令';

-- 事件发件箱（eventbus 正本，02 §9.3；device 发 device.activated/cmd_failed/ota_paused/alert 候选不发——越限判定归 iotingest/ops）。
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
