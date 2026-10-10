-- station_db 初始 schema（S6-02，v4/03 阶段 6）
-- 场站主数据/场站设备/驻场人员/场站拓扑（FR-STN-001~006）。
-- 状态一律 VARCHAR 枚举；雪花 BIGINT 主键；软删 + 租户。

-- 场站主数据（order_no 来源：Saga 步骤 6 建站回链，幂等键 FR-STN-003）。
CREATE TABLE IF NOT EXISTS `station`
(
    `station_id`   BIGINT        NOT NULL COMMENT '雪花 ID',
    `station_no`   VARCHAR(64)   NOT NULL COMMENT '场站编号（租户内唯一）',
    `name`         VARCHAR(128)  NOT NULL COMMENT '场站名称',
    `type`         VARCHAR(16)   NOT NULL DEFAULT 'ESS' COMMENT 'ESS 储能/CHARGING 充电/HESS 混合',
    `status`       VARCHAR(16)   NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE 运行/SUSPENDED 停用/RETIRED 退役',
    `province`     VARCHAR(32)   NOT NULL DEFAULT '' COMMENT '省',
    `city`         VARCHAR(32)   NOT NULL DEFAULT '' COMMENT '市',
    `address`      VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '详细地址',
    `longitude`    DECIMAL(10, 6) NULL COMMENT '经度',
    `latitude`     DECIMAL(10, 6) NULL COMMENT '纬度',
    `capacity_kwh` DECIMAL(12, 3) NULL COMMENT '容量 kWh',
    `power_kw`     DECIMAL(12, 3) NULL COMMENT '额定功率 kW',
    `grid_status`  VARCHAR(16)   NULL COMMENT '并网状态（仅记录，FR-STN-001）',
    `order_no`     VARCHAR(64)   NULL COMMENT '来源订单号（场站单 Saga 步骤 6）',
    `remark`       VARCHAR(512)  NULL,
    `tenant_id`    BIGINT        NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT        NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `updated_by`   BIGINT        NULL COMMENT '更新人 uid',
    `deleted_at`   DATETIME      NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`station_id`),
    UNIQUE KEY `uk_station_tenant_no` (`tenant_id`, `station_no`),
    -- Saga 步骤 6 幂等键：同订单建站重放直接命中（FR-STN-003）
    UNIQUE KEY `uk_station_tenant_order` (`tenant_id`, `order_no`),
    KEY `idx_station_tenant_status` (`tenant_id`, `status`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站主数据';

-- 场站-设备绑定（station_id+device_id 联合 PK；sn 冗余展示——SN 变更不影响关系记录，FR-STN-002）。
CREATE TABLE IF NOT EXISTS `station_device`
(
    `id`           BIGINT      NOT NULL COMMENT '雪花 ID（代理主键，联合唯一在 station+device）',
    `station_id`   BIGINT      NOT NULL COMMENT '场站',
    `device_id`    BIGINT      NOT NULL COMMENT '设备',
    `sn`           VARCHAR(64) NOT NULL COMMENT 'SN 冗余展示（FR-STN-002）',
    `role`         VARCHAR(16) NOT NULL DEFAULT 'PACK' COMMENT '角色：PACK 电池包/CHARGER 充电桩/METER 电表',
    `bound_at`     DATETIME(3) NULL COMMENT '绑定时刻',
    `bound_by`     VARCHAR(64) NULL COMMENT '绑定来源（order_no/manual）',
    `tenant_id`    BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`   BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_station_device` (`station_id`, `device_id`),
    KEY `idx_sdevice_device` (`device_id`, `deleted_at`),
    KEY `idx_sdevice_station` (`station_id`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站-设备绑定';

-- 场站人员（驻场 + 排班；user_id 逻辑引用 identity，FR-STN-006 data_scope 锁定本场站）。
CREATE TABLE IF NOT EXISTS `station_staff`
(
    `id`         BIGINT       NOT NULL COMMENT '雪花 ID',
    `station_id` BIGINT       NOT NULL COMMENT '场站',
    `user_id`    BIGINT       NOT NULL COMMENT '用户（identity 逻辑引用）',
    `staff_type` VARCHAR(16)  NOT NULL DEFAULT 'RESIDENT' COMMENT 'RESIDENT 驻场/INSPECTOR 巡检/MANAGER 站长',
    `shift`      VARCHAR(32)  NULL COMMENT '排班（day/night/JSON 计划）',
    `phone`      VARCHAR(512) NULL COMMENT '联系电话（AES 加密可空）',
    `tenant_id`  BIGINT       NOT NULL COMMENT '租户 ID',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by` BIGINT       NULL COMMENT '创建人 uid',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME     NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_station_staff` (`station_id`, `user_id`),
    KEY `idx_sstaff_user` (`user_id`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站人员';

-- 场站拓扑（版本化 + nodes/edges JSON 快照，FR-STN-004 React Flow 直喂数据形状）。
CREATE TABLE IF NOT EXISTS `station_topology`
(
    `id`          BIGINT      NOT NULL COMMENT '雪花 ID',
    `station_id`  BIGINT      NOT NULL COMMENT '场站',
    `version`     INT         NOT NULL COMMENT '版本号（递增）',
    `nodes`       JSON        NOT NULL COMMENT '节点（React Flow nodes 形状）',
    `edges`       JSON        NOT NULL COMMENT '边（React Flow edges 形状）',
    `tenant_id`   BIGINT      NOT NULL COMMENT '租户 ID',
    `created_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`  BIGINT      NULL COMMENT '创建人 uid',
    `updated_at`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`  DATETIME    NULL COMMENT '软删标记(NULL=有效)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_stopology_ver` (`station_id`, `version`),
    KEY `idx_stopology_latest` (`station_id`, `version` DESC)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='场站拓扑（版本化）';

-- 事件发件箱（eventbus 正本口径；station 发 station_created）。
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
